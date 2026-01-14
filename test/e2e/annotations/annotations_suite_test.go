package annotations

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	csiclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var (
	kubeClient    kubernetes.Interface
	csiClient     csiclient.Interface
	restConfig    *rest.Config
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

	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-annotations-test")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	kubeClient = testEnv.KubeClient
	csiClient = testEnv.CSIClient
	restConfig = testEnv.RestConfig
	testNamespace = testEnv.Namespace

	deployValues := map[string]string{
		"reloader.reloadStrategy": "annotations",
		"reloader.watchGlobally":  "false", // Only watch own namespace to prevent cross-talk between test suites
	}

	if utils.IsCSIDriverInstalled(ctx, csiClient) {
		deployValues["reloader.enableCSIIntegration"] = "true"
		GinkgoWriter.Println("Deploying Reloader with CSI integration support")
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

	GinkgoWriter.Println("Annotations E2E Suite cleanup complete")
})
