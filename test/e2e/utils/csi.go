package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	csiclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// CSI Driver constants
const (
	// CSIDriverName is the name of the secrets-store CSI driver
	CSIDriverName = "secrets-store.csi.k8s.io"

	// DefaultCSIProvider is the default provider name for testing (Vault)
	DefaultCSIProvider = "vault"

	// VaultAddress is the default Vault address in the cluster
	VaultAddress = "http://vault.vault:8200"

	// VaultRole is the Kubernetes auth role configured in Vault for testing
	VaultRole = "test-role"

	// VaultNamespace is the namespace where Vault is deployed
	VaultNamespace = "vault"

	// VaultPodName is the name of the Vault pod (dev mode)
	VaultPodName = "vault-0"

	// CSIVolumeName is the default volume name for CSI volumes in tests
	CSIVolumeName = "csi-secrets-store"

	// CSIMountPath is the default mount path for CSI volumes in tests
	CSIMountPath = "/mnt/secrets-store"

	// CSIRotationPollInterval is how often CSI driver checks for secret changes
	CSIRotationPollInterval = 2 * time.Second
)

// NewCSIClient creates a new CSI client using the default kubeconfig.
func NewCSIClient() (csiclient.Interface, error) {
	kubeconfig := GetKubeconfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("building config from kubeconfig: %w", err)
	}
	return NewCSIClientFromConfig(config)
}

// NewCSIClientFromConfig creates a new CSI client from a rest.Config.
func NewCSIClientFromConfig(config *rest.Config) (csiclient.Interface, error) {
	client, err := csiclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating CSI client: %w", err)
	}
	return client, nil
}

// IsCSIDriverInstalled checks if the CSI secrets store driver CRDs are available in the cluster.
// This checks for the SecretProviderClass CRD which is required for CSI tests.
func IsCSIDriverInstalled(ctx context.Context, client csiclient.Interface) bool {
	if client == nil {
		return false
	}

	// Try to list SecretProviderClasses - if CRD doesn't exist, this will fail
	_, err := client.SecretsstoreV1().SecretProviderClasses("default").List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// IsVaultProviderInstalled checks if Vault CSI provider is installed by checking for the vault-csi-provider DaemonSet.
// This is used to determine if CSI tests with actual volume mounting can run.
func IsVaultProviderInstalled(ctx context.Context, kubeClient kubernetes.Interface) bool {
	if kubeClient == nil {
		return false
	}

	// Check if vault-csi-provider DaemonSet exists in vault namespace
	_, err := kubeClient.AppsV1().DaemonSets("vault").Get(ctx, "vault-csi-provider", metav1.GetOptions{})
	return err == nil
}

// CreateSecretProviderClass creates a SecretProviderClass in the given namespace.
// If params is nil, it creates a Vault-compatible SecretProviderClass with default test settings.
func CreateSecretProviderClass(ctx context.Context, client csiclient.Interface, namespace, name string, params map[string]string) (
	*csiv1.SecretProviderClass, error,
) {
	if params == nil {
		params = map[string]string{
			"vaultAddress": VaultAddress,
			"roleName":     VaultRole,
			"objects": `- objectName: "test-secret"
  secretPath: "secret/data/test"
  secretKey: "username"`,
		}
	}

	spc := &csiv1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: csiv1.SecretProviderClassSpec{
			Provider:   DefaultCSIProvider,
			Parameters: params,
		},
	}

	created, err := client.SecretsstoreV1().SecretProviderClasses(namespace).Create(ctx, spc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating SecretProviderClass %s/%s: %w", namespace, name, err)
	}
	return created, nil
}

// CreateSecretProviderClassWithSecret creates a SecretProviderClass that fetches a specific secret from Vault.
// secretPath should be like "secret/mysecret" (the function converts it to KV v2 format "secret/data/mysecret").
// secretKey is the key within that secret to fetch.
func CreateSecretProviderClassWithSecret(ctx context.Context, client csiclient.Interface, namespace, name, secretPath, secretKey string) (
	*csiv1.SecretProviderClass, error,
) {
	kvV2Path := secretPath
	if strings.HasPrefix(secretPath, "secret/") && !strings.HasPrefix(secretPath, "secret/data/") {
		kvV2Path = strings.Replace(secretPath, "secret/", "secret/data/", 1)
	}

	params := map[string]string{
		"vaultAddress": VaultAddress,
		"roleName":     VaultRole,
		"objects": fmt.Sprintf(
			`- objectName: "%s"
  secretPath: "%s"
  secretKey: "%s"`, secretKey, kvV2Path, secretKey,
		),
	}
	return CreateSecretProviderClass(ctx, client, namespace, name, params)
}

// DeleteSecretProviderClass deletes a SecretProviderClass by name.
func DeleteSecretProviderClass(ctx context.Context, client csiclient.Interface, namespace, name string) error {
	err := client.SecretsstoreV1().SecretProviderClasses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting SecretProviderClass %s/%s: %w", namespace, name, err)
	}
	return nil
}

// UpdateSecretProviderClassPodStatusLabels updates only the labels on a SecretProviderClassPodStatus.
// This should NOT trigger a reload (used for negative testing to verify Reloader ignores label-only changes).
func UpdateSecretProviderClassPodStatusLabels(ctx context.Context, client csiclient.Interface, namespace, name string, labels map[string]string) error {
	spcps, err := client.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting SecretProviderClassPodStatus %s/%s: %w", namespace, name, err)
	}

	if spcps.Labels == nil {
		spcps.Labels = make(map[string]string)
	}
	for k, v := range labels {
		spcps.Labels[k] = v
	}

	_, err = client.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Update(ctx, spcps, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating SecretProviderClassPodStatus labels %s/%s: %w", namespace, name, err)
	}
	return nil
}

// =============================================================================
// Vault Integration Helpers
// =============================================================================

// CreateVaultSecret creates a new secret in Vault.
// secretPath should be like "secret/test" (without "data" prefix - it's added automatically).
// data is a map of key-value pairs to store in the secret.
func CreateVaultSecret(ctx context.Context, kubeClient kubernetes.Interface, restConfig *rest.Config, secretPath string, data map[string]string) error {
	return UpdateVaultSecret(ctx, kubeClient, restConfig, secretPath, data)
}

// UpdateVaultSecret updates a secret in Vault. This triggers the CSI driver to
// sync the new secret version, which creates/updates the SecretProviderClassPodStatus.
// secretPath should be like "secret/test" (without "data" prefix - it's added automatically).
// data is a map of key-value pairs to store in the secret.
func UpdateVaultSecret(ctx context.Context, kubeClient kubernetes.Interface, restConfig *rest.Config, secretPath string, data map[string]string) error {
	args := []string{"kv", "put", secretPath}
	for k, v := range data {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}

	if err := execInVaultPod(ctx, kubeClient, restConfig, args); err != nil {
		return fmt.Errorf("updating Vault secret %s: %w", secretPath, err)
	}
	return nil
}

// DeleteVaultSecret deletes a secret from Vault.
// secretPath should be like "secret/test".
func DeleteVaultSecret(ctx context.Context, kubeClient kubernetes.Interface, restConfig *rest.Config, secretPath string) error {
	args := []string{"kv", "metadata", "delete", secretPath}
	if err := execInVaultPod(ctx, kubeClient, restConfig, args); err != nil {
		if strings.Contains(err.Error(), "No value found") {
			return nil
		}
		return fmt.Errorf("deleting Vault secret %s: %w", secretPath, err)
	}
	return nil
}

// execInVaultPod executes a vault command in the Vault pod.
func execInVaultPod(ctx context.Context, kubeClient kubernetes.Interface, restConfig *rest.Config, args []string) error {
	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(VaultPodName).
		Namespace(VaultNamespace).
		SubResource("exec").
		VersionedParams(
			&corev1.PodExecOptions{
				Container: "vault",
				Command:   append([]string{"vault"}, args...),
				Stdout:    true,
				Stderr:    true,
			}, scheme.ParameterCodec,
		)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("creating executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(
		ctx, remotecommand.StreamOptions{
			Stdout: &stdout,
			Stderr: &stderr,
		},
	)
	if err != nil {
		return fmt.Errorf("executing command: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// WaitForSPCPSVersionChange waits for the SecretProviderClassPodStatus version to change
// from the initial version using watches. This is used after updating a Vault secret to
// wait for CSI driver to sync the new version.
func WaitForSPCPSVersionChange(ctx context.Context, client csiclient.Interface, namespace, spcpsName, initialVersion string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return client.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Watch(ctx, opts)
	}

	_, err := WatchUntil(ctx, watchFunc, spcpsName, SPCPSVersionChanged(initialVersion), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return fmt.Errorf("timeout waiting for SecretProviderClassPodStatus %s/%s version to change from %s", namespace, spcpsName, initialVersion)
	}
	return err
}

// FindSPCPSForDeployment finds the SecretProviderClassPodStatus created by CSI driver
// for pods of a given deployment using watches. Returns the first matching SPCPS name.
func FindSPCPSForDeployment(ctx context.Context, csiClient csiclient.Interface, kubeClient kubernetes.Interface, namespace, deploymentName string, timeout time.Duration) (
	string, error,
) {
	pods, err := kubeClient.CoreV1().Pods(namespace).List(
		ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", deploymentName),
		},
	)
	if err != nil {
		return "", fmt.Errorf("listing pods for deployment %s: %w", deploymentName, err)
	}

	podNames := make(map[string]bool)
	for _, pod := range pods.Items {
		podNames[pod.Name] = true
	}

	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return csiClient.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Watch(ctx, opts)
	}

	spcps, err := WatchUntil(ctx, watchFunc, "", SPCPSForPods(podNames), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return "", fmt.Errorf("timeout finding SecretProviderClassPodStatus for deployment %s/%s", namespace, deploymentName)
	}
	if err != nil {
		return "", err
	}
	return spcps.Name, nil
}

// FindSPCPSForSPC finds the SecretProviderClassPodStatus created by CSI driver
// that references a specific SecretProviderClass using watches. Returns the first matching SPCPS name.
func FindSPCPSForSPC(ctx context.Context, csiClient csiclient.Interface, namespace, spcName string, timeout time.Duration) (string, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return csiClient.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Watch(ctx, opts)
	}

	spcps, err := WatchUntil(ctx, watchFunc, "", SPCPSForSPC(spcName), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return "", fmt.Errorf("timeout finding SecretProviderClassPodStatus for SPC %s/%s", namespace, spcName)
	}
	if err != nil {
		return "", err
	}
	return spcps.Name, nil
}

// GetSPCPSVersion gets the current version string from a SecretProviderClassPodStatus.
// Returns the version of the first object, or empty string if not found.
func GetSPCPSVersion(ctx context.Context, client csiclient.Interface, namespace, name string) (string, error) {
	spcps, err := client.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting SecretProviderClassPodStatus %s/%s: %w", namespace, name, err)
	}
	if len(spcps.Status.Objects) == 0 {
		return "", nil
	}
	var versions []string
	for _, obj := range spcps.Status.Objects {
		versions = append(versions, obj.Version)
	}
	return strings.Join(versions, ","), nil
}
