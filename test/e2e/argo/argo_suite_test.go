package argo

import (
	"context"
	"encoding/json"
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

// SynchronizedBeforeSuite ensures only process 1 deploys Reloader.
// Process 1 also checks for Argo Rollouts and calls Skip if not installed —
// Ginkgo propagates the skip to all processes.
var _ = SynchronizedBeforeSuite(
	// Process 1 only: check prerequisites, create namespace, deploy Reloader.
	func() []byte {
		setupEnv, err := utils.SetupTestEnvironment(context.Background(), "reloader-argo")
		Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")

		if !utils.IsArgoRolloutsInstalled(context.Background(), setupEnv.RolloutsClient) {
			Skip("Argo Rollouts is not installed. Run ./scripts/e2e-cluster-setup.sh first")
		}
		GinkgoWriter.Println("Argo Rollouts is installed")

		Expect(setupEnv.DeployAndWait(map[string]string{
			"reloader.reloadStrategy": "annotations",
			"reloader.isArgoRollouts": "true",
		})).To(Succeed(), "Failed to deploy Reloader")

		data, err := json.Marshal(utils.SharedEnvData{
			Namespace:   setupEnv.Namespace,
			ReleaseName: setupEnv.ReleaseName,
		})
		Expect(err).NotTo(HaveOccurred())
		return data
	},
	// All processes (including #1): connect to the shared environment.
	func(data []byte) {
		var shared utils.SharedEnvData
		Expect(json.Unmarshal(data, &shared)).To(Succeed())

		var err error
		testEnv, err = utils.SetupSharedTestEnvironment(context.Background(), shared.Namespace, shared.ReleaseName)
		Expect(err).NotTo(HaveOccurred(), "Failed to setup shared test environment")

		kubeClient = testEnv.KubeClient
		rolloutsClient = testEnv.RolloutsClient
		testNamespace = testEnv.Namespace
		ctx = testEnv.Ctx
	},
)

var _ = SynchronizedAfterSuite(
	// All processes: cancel the per-process context.
	func() {
		if testEnv != nil {
			testEnv.Cancel()
		}
	},
	// Process 1 only (runs last): undeploy Reloader and delete namespace.
	func() {
		if testEnv != nil {
			err := testEnv.Cleanup()
			Expect(err).NotTo(HaveOccurred(), "Failed to cleanup test environment")
		}
		GinkgoWriter.Println("Argo Rollouts E2E Suite cleanup complete (Argo Rollouts preserved for other suites)")
	},
)
