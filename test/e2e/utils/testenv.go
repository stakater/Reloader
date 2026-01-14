package utils

import (
	"context"
	"fmt"

	rolloutsclient "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	"github.com/onsi/ginkgo/v2"
	openshiftclient "github.com/openshift/client-go/apps/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	csiclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// TestEnvironment holds the common test environment state.
type TestEnvironment struct {
	Ctx             context.Context
	Cancel          context.CancelFunc
	KubeClient      kubernetes.Interface
	DiscoveryClient discovery.DiscoveryInterface
	CSIClient       csiclient.Interface
	RolloutsClient  rolloutsclient.Interface
	OpenShiftClient openshiftclient.Interface
	RestConfig      *rest.Config
	Namespace       string
	ReleaseName     string
	TestImage       string
	ProjectDir      string
}

// SetupTestEnvironment creates a new test environment with kubernetes clients.
// It creates a unique namespace with the given prefix.
func SetupTestEnvironment(ctx context.Context, namespacePrefix string) (*TestEnvironment, error) {
	env := &TestEnvironment{
		Ctx:       ctx,
		TestImage: GetTestImage(),
	}

	var err error

	env.ProjectDir, err = GetProjectDir()
	if err != nil {
		return nil, fmt.Errorf("getting project directory: %w", err)
	}

	kubeconfig := GetKubeconfig()
	ginkgo.GinkgoWriter.Printf("Using kubeconfig: %s\n", kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("building config from kubeconfig: %w", err)
	}

	env.RestConfig = config

	env.KubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	env.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating discovery client: %w", err)
	}

	env.CSIClient, err = csiclient.NewForConfig(config)
	if err != nil {
		ginkgo.GinkgoWriter.Printf("Warning: Could not create CSI client: %v (CSI tests will be skipped)\n", err)
		env.CSIClient = nil
	}

	// Try to create Argo Rollouts client (optional - may not be installed)
	env.RolloutsClient, err = rolloutsclient.NewForConfig(config)
	if err != nil {
		ginkgo.GinkgoWriter.Printf("Warning: Could not create Rollouts client: %v (Argo tests will be skipped)\n", err)
		env.RolloutsClient = nil
	}

	// Try to create OpenShift client (optional - may not be installed)
	env.OpenShiftClient, err = openshiftclient.NewForConfig(config)
	if err != nil {
		ginkgo.GinkgoWriter.Printf("Warning: Could not create OpenShift client: %v (OpenShift tests will be skipped)\n",
			err)
		env.OpenShiftClient = nil
	}

	ginkgo.GinkgoWriter.Println("Verifying cluster connectivity...")
	_, err = env.KubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("connecting to kubernetes cluster: %w", err)
	}
	ginkgo.GinkgoWriter.Println("Cluster connectivity verified")

	env.Namespace = RandName(namespacePrefix)
	env.ReleaseName = RandName("reloader")
	ginkgo.GinkgoWriter.Printf("Creating test namespace: %s\n", env.Namespace)
	ginkgo.GinkgoWriter.Printf("Using Helm release name: %s\n", env.ReleaseName)
	if err := CreateNamespace(ctx, env.KubeClient, env.Namespace); err != nil {
		return nil, fmt.Errorf("creating test namespace: %w", err)
	}

	ginkgo.GinkgoWriter.Printf("Using test image: %s\n", env.TestImage)
	ginkgo.GinkgoWriter.Printf("Project directory: %s\n", env.ProjectDir)

	return env, nil
}

// Cleanup cleans up the test environment resources.
func (e *TestEnvironment) Cleanup() error {
	if e.Namespace == "" {
		return nil
	}

	ginkgo.GinkgoWriter.Printf("Cleaning up test namespace: %s\n", e.Namespace)
	ginkgo.GinkgoWriter.Printf("Cleaning up Helm release: %s\n", e.ReleaseName)

	logs, err := GetPodLogs(e.Ctx, e.KubeClient, e.Namespace, ReloaderPodSelector(e.ReleaseName))
	if err == nil && logs != "" {
		ginkgo.GinkgoWriter.Println("Reloader logs:")
		ginkgo.GinkgoWriter.Println(logs)
	}

	_ = UndeployReloader(e.Namespace, e.ReleaseName)

	if err := DeleteNamespace(e.Ctx, e.KubeClient, e.Namespace); err != nil {
		return fmt.Errorf("deleting namespace: %w", err)
	}

	return nil
}

// DeployReloaderWithStrategy deploys Reloader with the specified reload strategy.
func (e *TestEnvironment) DeployReloaderWithStrategy(strategy string) error {
	return e.DeployReloaderWithValues(
		map[string]string{
			"reloader.reloadStrategy": strategy,
		},
	)
}

// DeployReloaderWithValues deploys Reloader with the specified Helm values.
// Each test suite uses a unique release name to prevent cluster-scoped resource conflicts.
func (e *TestEnvironment) DeployReloaderWithValues(values map[string]string) error {
	ginkgo.GinkgoWriter.Printf("Deploying Reloader with values: %v\n", values)
	return DeployReloader(
		DeployOptions{
			Namespace:   e.Namespace,
			ReleaseName: e.ReleaseName,
			Image:       e.TestImage,
			Values:      values,
		},
	)
}

// WaitForReloader waits for the Reloader deployment to be ready.
func (e *TestEnvironment) WaitForReloader() error {
	ginkgo.GinkgoWriter.Println("Waiting for Reloader to be ready...")
	adapter := NewDeploymentAdapter(e.KubeClient)
	return adapter.WaitReady(e.Ctx, e.Namespace, ReloaderDeploymentName(e.ReleaseName), DeploymentReady)
}

// DeployAndWait deploys Reloader with the given values and waits for it to be ready.
func (e *TestEnvironment) DeployAndWait(values map[string]string) error {
	if err := e.DeployReloaderWithValues(values); err != nil {
		return fmt.Errorf("deploying Reloader: %w", err)
	}
	if err := e.WaitForReloader(); err != nil {
		return fmt.Errorf("waiting for Reloader: %w", err)
	}
	ginkgo.GinkgoWriter.Println("Reloader is ready")
	return nil
}
