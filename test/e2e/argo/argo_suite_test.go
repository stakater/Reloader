package argo

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
	testEnv       *utils.TestEnvironment
)

func TestArgo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Argo Rollouts E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx = context.Background()

	// Setup test environment
	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-argo")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	// Export for use in tests
	kubeClient = testEnv.KubeClient
	dynamicClient = testEnv.DynamicClient
	testNamespace = testEnv.Namespace

	// Check if Argo Rollouts is installed
	// NOTE: Argo Rollouts should be pre-installed using: ./scripts/e2e-cluster-setup.sh
	// This suite does NOT install Argo Rollouts to ensure consistent behavior across all test suites.
	if !utils.IsArgoRolloutsInstalled(ctx, dynamicClient) {
		Skip("Argo Rollouts is not installed. Run ./scripts/e2e-cluster-setup.sh first")
	}
	GinkgoWriter.Println("Argo Rollouts is installed")

	// Deploy Reloader with Argo Rollouts support
	err = testEnv.DeployAndWait(map[string]string{
		"reloader.reloadStrategy": "annotations",
		"reloader.isArgoRollouts": "true",
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy Reloader")
})

var _ = AfterSuite(func() {
	// Cleanup test environment (Reloader + namespace)
	if testEnv != nil {
		err := testEnv.Cleanup()
		Expect(err).NotTo(HaveOccurred(), "Failed to cleanup test environment")
	}

	// NOTE: Argo Rollouts is NOT uninstalled here to allow other test suites (core/)
	// to run Argo tests. Cleanup is handled by: ./scripts/e2e-cluster-cleanup.sh
	GinkgoWriter.Println("Argo Rollouts E2E Suite cleanup complete (Argo Rollouts preserved for other suites)")
})
