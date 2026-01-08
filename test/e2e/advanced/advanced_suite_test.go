package advanced

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

func TestAdvanced(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Advanced E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx = context.Background()

	// Setup test environment
	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-advanced")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	// Export for use in tests
	kubeClient = testEnv.KubeClient
	testNamespace = testEnv.Namespace

	// Deploy Reloader with annotations strategy
	err = testEnv.DeployAndWait(map[string]string{
		"reloader.reloadStrategy": "annotations",
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy Reloader")
})

var _ = AfterSuite(func() {
	if testEnv != nil {
		err := testEnv.Cleanup()
		Expect(err).NotTo(HaveOccurred(), "Failed to cleanup test environment")
	}

	GinkgoWriter.Println("Advanced E2E Suite cleanup complete")
})
