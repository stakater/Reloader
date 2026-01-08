package flags

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
	"k8s.io/client-go/kubernetes"
)

var (
	kubeClient    kubernetes.Interface
	testNamespace string
	ctx           context.Context
	testEnv       *utils.TestEnvironment
)

func TestFlags(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flag-Based E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx = context.Background()

	// Setup test environment (but don't deploy Reloader - tests do that with specific flags)
	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-flags")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	// Export for use in tests
	kubeClient = testEnv.KubeClient
	testNamespace = testEnv.Namespace

	// Note: Unlike other suites, we don't deploy Reloader here.
	// Each test deploys with specific flag configurations.
})

var _ = AfterSuite(func() {
	if testEnv != nil {
		err := testEnv.Cleanup()
		Expect(err).NotTo(HaveOccurred(), "Failed to cleanup test environment")
	}

	GinkgoWriter.Println("Flags E2E Suite cleanup complete")
})

// deployReloaderWithFlags deploys Reloader with the specified Helm value overrides.
// This is a convenience function for tests that need to deploy with specific flags.
func deployReloaderWithFlags(values map[string]string) error {
	// Always include annotations strategy
	if values == nil {
		values = make(map[string]string)
	}
	if _, ok := values["reloader.reloadStrategy"]; !ok {
		values["reloader.reloadStrategy"] = "annotations"
	}
	return testEnv.DeployAndWait(values)
}

// undeployReloader removes the Reloader installation.
func undeployReloader() error {
	return utils.UndeployReloader(testNamespace, testEnv.ReleaseName)
}

// waitForReloaderReady waits for the Reloader deployment to be ready.
func waitForReloaderReady() error {
	return testEnv.WaitForReloader()
}
