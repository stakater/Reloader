package annotations

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var (
	kubeClient    kubernetes.Interface
	restConfig    *rest.Config
	testNamespace string
	ctx           context.Context
	testEnv       *utils.TestEnvironment
	registry      *utils.AdapterRegistry
)

func TestAnnotations(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Annotations Strategy E2E Suite")
}

// SynchronizedBeforeSuite ensures only process 1 deploys Reloader.
// The namespace and release name are forwarded to all other processes so they
// share a single Reloader instance, avoiding resource exhaustion on Kind.
var _ = SynchronizedBeforeSuite(
	// Process 1 only: create namespace, deploy Reloader.
	func() []byte {
		setupEnv, err := utils.SetupTestEnvironment(context.Background(), "reloader-annotations-test")
		Expect(err).NotTo(HaveOccurred(), "Failed to setup test environment")
		// Ensure the namespace is deleted even if DeployAndWait fails, so
		// orphaned namespaces don't accumulate on long-lived clusters.
		DeferCleanup(setupEnv.CleanupOnFailure)

		deployValues := map[string]string{
			"reloader.reloadStrategy": "annotations",
			"reloader.watchGlobally":  "false",
		}

		Expect(setupEnv.DeployAndWait(deployValues)).To(Succeed(), "Failed to deploy Reloader")

		data, err := json.Marshal(utils.SharedEnvData{
			Namespace:   setupEnv.Namespace,
			ReleaseName: setupEnv.ReleaseName,
		})
		Expect(err).NotTo(HaveOccurred())
		return data
	},
	// All processes (including #1): connect to shared environment and build adapter registry.
	func(data []byte) {
		var shared utils.SharedEnvData
		Expect(json.Unmarshal(data, &shared)).To(Succeed())

		var err error
		testEnv, err = utils.SetupSharedTestEnvironment(context.Background(), shared.Namespace, shared.ReleaseName)
		Expect(err).NotTo(HaveOccurred(), "Failed to setup shared test environment")

		kubeClient = testEnv.KubeClient
		restConfig = testEnv.RestConfig
		testNamespace = testEnv.Namespace
		ctx = testEnv.Ctx

		registry = utils.NewAdapterRegistry(kubeClient)
		if utils.IsArgoRolloutsInstalled(ctx, testEnv.RolloutsClient) {
			GinkgoWriter.Println("Argo Rollouts detected, registering ArgoRolloutAdapter")
			registry.RegisterAdapter(utils.NewArgoRolloutAdapter(testEnv.RolloutsClient))
		} else {
			GinkgoWriter.Println("Argo Rollouts not detected, skipping ArgoRolloutAdapter registration")
		}
		if utils.HasDeploymentConfigSupport(testEnv.DiscoveryClient) && testEnv.OpenShiftClient != nil {
			GinkgoWriter.Println("OpenShift detected, registering DeploymentConfigAdapter")
			registry.RegisterAdapter(utils.NewDeploymentConfigAdapter(testEnv.OpenShiftClient))
		} else {
			GinkgoWriter.Println("OpenShift not detected, skipping DeploymentConfigAdapter registration")
		}
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
		GinkgoWriter.Println("Annotations E2E Suite cleanup complete")
	},
)
