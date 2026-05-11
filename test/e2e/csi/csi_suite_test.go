package csi

import (
	"context"
	"encoding/json"
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
	testEnv       *utils.TestEnvironment
)

func TestCSI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSI SecretProviderClass E2E Suite")
}

// SynchronizedBeforeSuite ensures only process 1 deploys Reloader.
// Process 1 also checks prerequisites (CSI driver, Vault) and calls Skip if
// they are not installed — Ginkgo propagates the skip to all processes.
var _ = SynchronizedBeforeSuite(
	// Process 1 only: check prerequisites, create namespace, deploy Reloader.
	func() []byte {
		setupEnv, err := utils.SetupTestEnvironment(context.Background(), "reloader-csi-test")
		Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")
		// Ensure the namespace is deleted even if DeployAndWait fails, so
		// orphaned namespaces don't accumulate on long-lived clusters.
		DeferCleanup(setupEnv.CleanupOnFailure)

		if !utils.IsCSIDriverInstalled(context.Background(), setupEnv.CSIClient) {
			Skip("CSI secrets store driver not installed - skipping CSI suite")
		}
		if !utils.IsVaultProviderInstalled(context.Background(), setupEnv.KubeClient) {
			Skip("Vault CSI provider not installed - skipping CSI suite")
		}

		Expect(setupEnv.DeployAndWait(map[string]string{
			"reloader.reloadStrategy":       "annotations",
			"reloader.watchGlobally":        "false",
			"reloader.enableCSIIntegration": "true",
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
		csiClient = testEnv.CSIClient
		restConfig = testEnv.RestConfig
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
		GinkgoWriter.Println("CSI E2E Suite cleanup complete")
	},
)
