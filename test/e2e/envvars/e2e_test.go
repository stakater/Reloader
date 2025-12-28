// Package envvars contains end-to-end tests for Reloader's EnvVars Reload Strategy.
package envvars

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	testNamespacePrefix = "test-reloader-envvars-"
	waitTimeout         = 30 * time.Second
	setupDelay          = 2 * time.Second
	negativeTestTimeout = 5 * time.Second
	envVarPrefix        = "STAKATER_"
)

var (
	k8sClient     kubernetes.Interface
	envVarsCfg    *config.Config
	namespace     string
	skipE2ETests  bool
	cancelManager context.CancelFunc
	restCfg       *rest.Config
)

// envVarsFixture provides test setup/teardown for EnvVars strategy tests.
type envVarsFixture struct {
	t          *testing.T
	name       string
	configMaps []string
	secrets    []string
	workloads  []workloadInfo
}

type workloadInfo struct {
	name string
	kind string
}

func newEnvVarsFixture(t *testing.T, prefix string) *envVarsFixture {
	t.Helper()
	skipIfNoCluster(t)
	return &envVarsFixture{
		t:    t,
		name: prefix + "-" + testutil.RandSeq(5),
	}
}

func (f *envVarsFixture) createConfigMap(name, data string) {
	f.t.Helper()
	_, err := testutil.CreateConfigMap(k8sClient, namespace, name, data)
	if err != nil {
		f.t.Fatalf("Failed to create ConfigMap %s: %v", name, err)
	}
	f.configMaps = append(f.configMaps, name)
}

func (f *envVarsFixture) createSecret(name, data string) {
	f.t.Helper()
	_, err := testutil.CreateSecret(k8sClient, namespace, name, data)
	if err != nil {
		f.t.Fatalf("Failed to create Secret %s: %v", name, err)
	}
	f.secrets = append(f.secrets, name)
}

func (f *envVarsFixture) createDeployment(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateDeployment(k8sClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create Deployment %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "deployment"})
}

func (f *envVarsFixture) createDaemonSet(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateDaemonSet(k8sClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create DaemonSet %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "daemonset"})
}

func (f *envVarsFixture) createStatefulSet(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateStatefulSet(k8sClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create StatefulSet %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "statefulset"})
}

func (f *envVarsFixture) waitForReady() {
	time.Sleep(setupDelay)
}

func (f *envVarsFixture) updateConfigMap(name, data string) {
	f.t.Helper()
	if err := testutil.UpdateConfigMapWithClient(k8sClient, namespace, name, "", data); err != nil {
		f.t.Fatalf("Failed to update ConfigMap %s: %v", name, err)
	}
}

func (f *envVarsFixture) updateConfigMapLabel(name, label string) {
	f.t.Helper()
	cm, err := k8sClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		f.t.Fatalf("Failed to get ConfigMap %s: %v", name, err)
	}
	data := cm.Data["url"]
	if err := testutil.UpdateConfigMapWithClient(k8sClient, namespace, name, label, data); err != nil {
		f.t.Fatalf("Failed to update ConfigMap label %s: %v", name, err)
	}
}

func (f *envVarsFixture) updateSecret(name, data string) {
	f.t.Helper()
	if err := testutil.UpdateSecretWithClient(k8sClient, namespace, name, "", data); err != nil {
		f.t.Fatalf("Failed to update Secret %s: %v", name, err)
	}
}

func (f *envVarsFixture) assertDeploymentHasEnvVar(name string) {
	f.t.Helper()
	updated, err := testutil.WaitForDeploymentEnvVar(k8sClient, namespace, name, envVarPrefix, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for deployment %s env var: %v", name, err)
	}
	if !updated {
		f.t.Errorf("Deployment %s does not have Reloader env var", name)
	}
}

func (f *envVarsFixture) assertDeploymentNoEnvVar(name string) {
	f.t.Helper()
	time.Sleep(negativeTestTimeout)
	updated, _ := testutil.WaitForDeploymentEnvVar(k8sClient, namespace, name, envVarPrefix, negativeTestTimeout)
	if updated {
		f.t.Errorf("Deployment %s should not have Reloader env var", name)
	}
}

func (f *envVarsFixture) assertDaemonSetHasEnvVar(name string) {
	f.t.Helper()
	updated, err := testutil.WaitForDaemonSetEnvVar(k8sClient, namespace, name, envVarPrefix, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for daemonset %s env var: %v", name, err)
	}
	if !updated {
		f.t.Errorf("DaemonSet %s does not have Reloader env var", name)
	}
}

func (f *envVarsFixture) assertStatefulSetHasEnvVar(name string) {
	f.t.Helper()
	updated, err := testutil.WaitForStatefulSetEnvVar(k8sClient, namespace, name, envVarPrefix, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for statefulset %s env var: %v", name, err)
	}
	if !updated {
		f.t.Errorf("StatefulSet %s does not have Reloader env var", name)
	}
}

func (f *envVarsFixture) cleanup() {
	for _, w := range f.workloads {
		switch w.kind {
		case "deployment":
			_ = testutil.DeleteDeployment(k8sClient, namespace, w.name)
		case "daemonset":
			_ = testutil.DeleteDaemonSet(k8sClient, namespace, w.name)
		case "statefulset":
			_ = testutil.DeleteStatefulSet(k8sClient, namespace, w.name)
		}
	}
	for _, name := range f.configMaps {
		_ = testutil.DeleteConfigMap(k8sClient, namespace, name)
	}
	for _, name := range f.secrets {
		_ = testutil.DeleteSecret(k8sClient, namespace, name)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		os.Exit(0)
	}

	zl := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		Level(zerolog.WarnLevel).
		With().
		Timestamp().
		Logger()
	ctrllog.SetLogger(zerologr.New(&zl))

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	var err error
	restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		skipE2ETests = true
		os.Exit(0)
	}

	k8sClient, err = kubernetes.NewForConfig(restCfg)
	if err != nil {
		skipE2ETests = true
		os.Exit(0)
	}

	if _, err = k8sClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{}); err != nil {
		skipE2ETests = true
		os.Exit(0)
	}

	namespace = testNamespacePrefix + testutil.RandSeq(5)
	if err := testutil.CreateNamespace(namespace, k8sClient); err != nil {
		panic(err)
	}

	envVarsCfg = config.NewDefault()
	envVarsCfg.ReloadStrategy = config.ReloadStrategyEnvVars
	envVarsCfg.AutoReloadAll = false

	collectors := metrics.NewCollectors()
	mgr, err := controller.NewManagerWithRestConfig(
		controller.ManagerOptions{
			Config:     envVarsCfg,
			Log:        ctrl.Log.WithName("envvars-test-manager"),
			Collectors: &collectors,
		}, restCfg,
	)
	if err != nil {
		panic("Failed to create EnvVars manager: " + err.Error())
	}

	if err := controller.SetupReconcilers(mgr, envVarsCfg, ctrl.Log.WithName("envvars-test-reconcilers"), &collectors); err != nil {
		panic("Failed to setup EnvVars reconcilers: " + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelManager = cancel

	go func() {
		if err := controller.RunManager(ctx, mgr, ctrl.Log.WithName("envvars-test-runner")); err != nil {
			log.Printf("Manager exited: %v", err)
		}
	}()

	time.Sleep(3 * time.Second)

	code := m.Run()

	if cancelManager != nil {
		cancelManager()
		time.Sleep(2 * time.Second)
	}

	_ = testutil.DeleteNamespace(namespace, k8sClient)
	os.Exit(code)
}

func skipIfNoCluster(t *testing.T) {
	if skipE2ETests {
		t.Skip("Skipping e2e test: no Kubernetes cluster available")
	}
}

// TestEnvVarsConfigMapUpdate tests that updating a ConfigMap triggers env var update in deployment.
func TestEnvVarsConfigMapUpdate(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-cm")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			envVarsCfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentHasEnvVar(f.name)
}

// TestEnvVarsSecretUpdate tests that updating a Secret triggers env var update in deployment.
func TestEnvVarsSecretUpdate(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-secret")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createDeployment(
		f.name, false, map[string]string{
			envVarsCfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertDeploymentHasEnvVar(f.name)
}

// TestEnvVarsAutoReload tests auto-reload with EnvVars strategy.
func TestEnvVarsAutoReload(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-auto")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			envVarsCfg.Annotations.Auto: "true",
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentHasEnvVar(f.name)
}

// TestEnvVarsDaemonSetConfigMap tests that DaemonSets get env var on ConfigMap change.
func TestEnvVarsDaemonSetConfigMap(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-ds-cm")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDaemonSet(
		f.name, true, map[string]string{
			envVarsCfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDaemonSetHasEnvVar(f.name)
}

// TestEnvVarsDaemonSetSecret tests that DaemonSets get env var on Secret change.
func TestEnvVarsDaemonSetSecret(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-ds-secret")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createDaemonSet(
		f.name, false, map[string]string{
			envVarsCfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertDaemonSetHasEnvVar(f.name)
}

// TestEnvVarsStatefulSetConfigMap tests that StatefulSets get env var on ConfigMap change.
func TestEnvVarsStatefulSetConfigMap(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-sts-cm")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createStatefulSet(
		f.name, true, map[string]string{
			envVarsCfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertStatefulSetHasEnvVar(f.name)
}

// TestEnvVarsStatefulSetSecret tests that StatefulSets get env var on Secret change.
func TestEnvVarsStatefulSetSecret(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-sts-secret")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createStatefulSet(
		f.name, false, map[string]string{
			envVarsCfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertStatefulSetHasEnvVar(f.name)
}

// TestEnvVarsLabelOnlyChange tests that label-only changes don't trigger env var updates.
func TestEnvVarsLabelOnlyChange(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-label")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			envVarsCfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMapLabel(f.name, "new-label")
	f.assertDeploymentNoEnvVar(f.name)
}

// TestEnvVarsMultipleUpdates tests multiple updates with EnvVars strategy.
func TestEnvVarsMultipleUpdates(t *testing.T) {
	f := newEnvVarsFixture(t, "envvars-multi")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			envVarsCfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data-1")
	f.assertDeploymentHasEnvVar(f.name)

	deploy1, _ := k8sClient.AppsV1().Deployments(namespace).Get(context.Background(), f.name, metav1.GetOptions{})
	var envValue1 string
	for _, container := range deploy1.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if len(env.Name) > len(envVarPrefix) && env.Name[:len(envVarPrefix)] == envVarPrefix {
				envValue1 = env.Value
				break
			}
		}
	}

	time.Sleep(2 * time.Second)

	f.updateConfigMap(f.name, "updated-data-2")
	time.Sleep(5 * time.Second)

	deploy2, _ := k8sClient.AppsV1().Deployments(namespace).Get(context.Background(), f.name, metav1.GetOptions{})
	var envValue2 string
	for _, container := range deploy2.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if len(env.Name) > len(envVarPrefix) && env.Name[:len(envVarPrefix)] == envVarPrefix {
				envValue2 = env.Value
				break
			}
		}
	}

	if envValue1 == envValue2 {
		t.Errorf("Env var value should have changed after second update, got same value: %s", envValue1)
	}
}
