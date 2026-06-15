package flags

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"

	"github.com/stakater/Reloader/test/e2e/utils"
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

// SynchronizedBeforeSuite ensures only process 1 creates the shared namespace.
// The flags tests each deploy/undeploy Reloader themselves (marked Serial), so
// there is no shared Reloader instance — only the namespace is shared.
var _ = SynchronizedBeforeSuite(
	// Process 1 only: create namespace, build clients.
	func() []byte {
		setupEnv, err := utils.SetupTestEnvironment(context.Background(), "reloader-flags")
		Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")
		// Ensure the namespace is cleaned up if setup fails.
		DeferCleanup(setupEnv.CleanupOnFailure)

		data, err := json.Marshal(utils.SharedEnvData{
			Namespace:   setupEnv.Namespace,
			ReleaseName: setupEnv.ReleaseName,
		})
		Expect(err).NotTo(HaveOccurred())
		return data
	},
	// All processes (including #1): connect to the shared namespace.
	func(data []byte) {
		var shared utils.SharedEnvData
		Expect(json.Unmarshal(data, &shared)).To(Succeed())

		var err error
		testEnv, err = utils.SetupSharedTestEnvironment(context.Background(), shared.Namespace, shared.ReleaseName)
		Expect(err).NotTo(HaveOccurred(), "Failed to setup shared test environment")

		kubeClient = testEnv.KubeClient
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
	// Process 1 only (runs last): delete namespace.
	func() {
		if testEnv != nil {
			err := testEnv.Cleanup()
			Expect(err).NotTo(HaveOccurred(), "Failed to cleanup test environment")
		}
		GinkgoWriter.Println("Flags E2E Suite cleanup complete")
	},
)

// deployReloaderWithFlags deploys Reloader with the specified Helm value overrides.
// This is a convenience function for tests that need to deploy with specific flags.
func deployReloaderWithFlags(values map[string]string) error {
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
