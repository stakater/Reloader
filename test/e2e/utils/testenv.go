package utils

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TestEnvironment holds the common test environment state.
type TestEnvironment struct {
	Ctx             context.Context
	Cancel          context.CancelFunc
	KubeClient      kubernetes.Interface
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	Namespace       string
	ReleaseName     string // Unique Helm release name to prevent cluster-scoped resource conflicts
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

	// Get project directory
	env.ProjectDir, err = GetProjectDir()
	if err != nil {
		return nil, fmt.Errorf("getting project directory: %w", err)
	}

	// Setup Kubernetes client
	kubeconfig := GetKubeconfig()
	GinkgoWriter.Printf("Using kubeconfig: %s\n", kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("building config from kubeconfig: %w", err)
	}

	env.KubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	env.DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	env.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating discovery client: %w", err)
	}

	// Verify cluster connectivity
	GinkgoWriter.Println("Verifying cluster connectivity...")
	_, err = env.KubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("connecting to kubernetes cluster: %w", err)
	}
	GinkgoWriter.Println("Cluster connectivity verified")

	// Create test namespace with random suffix
	env.Namespace = RandName(namespacePrefix)
	// Use a unique release name to prevent cluster-scoped resource conflicts between test suites
	env.ReleaseName = RandName("reloader")
	GinkgoWriter.Printf("Creating test namespace: %s\n", env.Namespace)
	GinkgoWriter.Printf("Using Helm release name: %s\n", env.ReleaseName)
	if err := CreateNamespace(ctx, env.KubeClient, env.Namespace); err != nil {
		return nil, fmt.Errorf("creating test namespace: %w", err)
	}

	GinkgoWriter.Printf("Using test image: %s\n", env.TestImage)
	GinkgoWriter.Printf("Project directory: %s\n", env.ProjectDir)

	return env, nil
}

// Cleanup cleans up the test environment resources.
func (e *TestEnvironment) Cleanup() error {
	if e.Namespace == "" {
		return nil
	}

	GinkgoWriter.Printf("Cleaning up test namespace: %s\n", e.Namespace)
	GinkgoWriter.Printf("Cleaning up Helm release: %s\n", e.ReleaseName)

	// Collect Reloader logs before cleanup (useful for debugging)
	logs, err := GetPodLogs(e.Ctx, e.KubeClient, e.Namespace, ReloaderPodSelector(e.ReleaseName))
	if err == nil && logs != "" {
		GinkgoWriter.Println("Reloader logs:")
		GinkgoWriter.Println(logs)
	}

	// Undeploy Reloader using the suite's release name
	_ = UndeployReloader(e.Namespace, e.ReleaseName)

	// Delete test namespace
	if err := DeleteNamespace(e.Ctx, e.KubeClient, e.Namespace); err != nil {
		return fmt.Errorf("deleting namespace: %w", err)
	}

	return nil
}

// DeployReloaderWithStrategy deploys Reloader with the specified reload strategy.
func (e *TestEnvironment) DeployReloaderWithStrategy(strategy string) error {
	return e.DeployReloaderWithValues(map[string]string{
		"reloader.reloadStrategy": strategy,
	})
}

// DeployReloaderWithValues deploys Reloader with the specified Helm values.
// Each test suite uses a unique release name to prevent cluster-scoped resource conflicts.
func (e *TestEnvironment) DeployReloaderWithValues(values map[string]string) error {
	GinkgoWriter.Printf("Deploying Reloader with values: %v\n", values)
	return DeployReloader(DeployOptions{
		Namespace:   e.Namespace,
		ReleaseName: e.ReleaseName,
		Image:       e.TestImage,
		Values:      values,
	})
}

// WaitForReloader waits for the Reloader deployment to be ready.
func (e *TestEnvironment) WaitForReloader() error {
	GinkgoWriter.Println("Waiting for Reloader to be ready...")
	return WaitForDeploymentReady(e.Ctx, e.KubeClient, e.Namespace, ReloaderDeploymentName(e.ReleaseName), DeploymentReady)
}

// DeployAndWait deploys Reloader with the given values and waits for it to be ready.
func (e *TestEnvironment) DeployAndWait(values map[string]string) error {
	if err := e.DeployReloaderWithValues(values); err != nil {
		return fmt.Errorf("deploying Reloader: %w", err)
	}
	if err := e.WaitForReloader(); err != nil {
		return fmt.Errorf("waiting for Reloader: %w", err)
	}
	GinkgoWriter.Println("Reloader is ready")
	return nil
}
