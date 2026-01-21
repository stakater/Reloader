package argo

import (
	"context"
	"testing"

	rolloutsclient "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var (
	kubeClient     kubernetes.Interface
	rolloutsClient rolloutsclient.Interface
	testNamespace  string
	ctx            context.Context
	testEnv        *utils.TestEnvironment
)

func TestArgo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Argo Rollouts E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx = context.Background()

	testEnv, err = utils.SetupTestEnvironment(ctx, "reloader-argo")
	Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

	kubeClient = testEnv.KubeClient
	rolloutsClient = testEnv.RolloutsClient
	testNamespace = testEnv.Namespace

	if !utils.IsArgoRolloutsInstalled(ctx, rolloutsClient) {
		Skip("Argo Rollouts is not installed. Run ./scripts/e2e-cluster-setup.sh first")
	}
	GinkgoWriter.Println("Argo Rollouts is installed")

	err = testEnv.DeployAndWait(map[string]string{
		"reloader.reloadStrategy": "annotations",
		"reloader.isArgoRollouts": "true",
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy Reloader")
})

var _ = AfterSuite(func() {
	if testEnv != nil {
		err := testEnv.Cleanup()
		Expect(err).NotTo(HaveOccurred(), "Failed to cleanup test environment")
	}

	GinkgoWriter.Println("Argo Rollouts E2E Suite cleanup complete (Argo Rollouts preserved for other suites)")
})
