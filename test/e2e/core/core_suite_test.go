package core

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)


var (
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	testNamespace string
	ctx           context.Context
	cancel        context.CancelFunc
	testEnv       *utils.TestEnvironment
	registry      *utils.AdapterRegistry
)

func TestCore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Core Workload E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx, cancel = context.WithCancel(context.Background())

	// Setup test environment
	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-core-test")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	// Export for use in tests
	kubeClient = testEnv.KubeClient
	dynamicClient = testEnv.DynamicClient
	testNamespace = testEnv.Namespace

	// Create adapter registry
	registry = utils.NewAdapterRegistry(kubeClient, dynamicClient)

	// Register ArgoRolloutAdapter if Argo Rollouts is installed
	if utils.IsArgoRolloutsInstalled(ctx, dynamicClient) {
		GinkgoWriter.Println("Argo Rollouts detected, registering ArgoRolloutAdapter")
		registry.RegisterAdapter(utils.NewArgoRolloutAdapter(dynamicClient))
	} else {
		GinkgoWriter.Println("Argo Rollouts not detected, skipping ArgoRolloutAdapter registration")
	}

	// Register DeploymentConfigAdapter if OpenShift is available
	if utils.HasDeploymentConfigSupport(testEnv.DiscoveryClient) {
		GinkgoWriter.Println("OpenShift detected, registering DeploymentConfigAdapter")
		registry.RegisterAdapter(utils.NewDeploymentConfigAdapter(dynamicClient))
	} else {
		GinkgoWriter.Println("OpenShift not detected, skipping DeploymentConfigAdapter registration")
	}

	// Deploy Reloader with default annotations strategy
	// Individual test contexts will redeploy with different strategies if needed
	deployValues := map[string]string{
		"reloader.reloadStrategy": "annotations",
	}

	// Enable Argo Rollouts support if Argo is installed
	if utils.IsArgoRolloutsInstalled(ctx, dynamicClient) {
		deployValues["reloader.isArgoRollouts"] = "true"
		GinkgoWriter.Println("Deploying Reloader with Argo Rollouts support")
	}

	err = testEnv.DeployAndWait(deployValues)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy Reloader")
})

var _ = AfterSuite(func() {
	if testEnv != nil {
		err := testEnv.Cleanup()
		Expect(err).NotTo(HaveOccurred(), "Failed to cleanup test environment")
	}

	if cancel != nil {
		cancel()
	}

	GinkgoWriter.Println("Core E2E Suite cleanup complete")
})
