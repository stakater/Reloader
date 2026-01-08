package annotations

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
)

func TestAnnotations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Annotations Strategy E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx, cancel = context.WithCancel(context.Background())

	// Setup test environment
	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-annotations-test")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	// Export for use in tests
	kubeClient = testEnv.KubeClient
	dynamicClient = testEnv.DynamicClient
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

	if cancel != nil {
		cancel()
	}

	GinkgoWriter.Println("Annotations E2E Suite cleanup complete")
})
