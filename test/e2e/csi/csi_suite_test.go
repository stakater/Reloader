package csi

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

func TestCSI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSI SecretProviderClass E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx, cancel = context.WithCancel(context.Background())

	// Setup test environment
	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-csi-test")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	// Export for use in tests
	kubeClient = testEnv.KubeClient
	csiClient = testEnv.CSIClient
	restConfig = testEnv.RestConfig
	testNamespace = testEnv.Namespace

	// Skip entire suite if CSI driver not installed
	if !utils.IsCSIDriverInstalled(ctx, csiClient) {
		Skip("CSI secrets store driver not installed - skipping CSI suite")
	}

	// Skip entire suite if Vault CSI provider not installed
	if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
		Skip("Vault CSI provider not installed - skipping CSI suite")
	}

	// Deploy Reloader with annotations strategy and CSI integration enabled
	err = testEnv.DeployAndWait(map[string]string{
		"reloader.reloadStrategy":       "annotations",
		"reloader.watchGlobally":        "false", // Only watch own namespace to prevent cross-talk between test suites
		"reloader.enableCSIIntegration": "true",
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

	GinkgoWriter.Println("CSI E2E Suite cleanup complete")
})
