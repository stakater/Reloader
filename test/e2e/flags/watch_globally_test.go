package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Watch Globally Flag Tests", func() {
	var (
		deploymentName string
		configMapName  string
		otherNS        string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		otherNS = "other-" + utils.RandName("ns")
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		// Clean up resources in both namespaces
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteDeployment(ctx, kubeClient, otherNS, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, otherNS, configMapName)
	})

	Context("with watchGlobally=false flag", func() {
		BeforeEach(func() {
			// Create the other namespace for testing cross-namespace behavior
			err := utils.CreateNamespace(ctx, kubeClient, otherNS)
			Expect(err).NotTo(HaveOccurred())

			// Deploy Reloader with watchGlobally=false
			// This makes Reloader only watch resources in its own namespace (testNamespace)
			err = deployReloaderWithFlags(map[string]string{
				"reloader.watchGlobally": "false",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, otherNS)
		})

		It("should reload workloads in Reloader's namespace when watchGlobally=false", func() {
			By("Creating a ConfigMap in Reloader's namespace")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment in Reloader's namespace with auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (same namespace should work)")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment in Reloader's namespace should reload with watchGlobally=false")
		})

		It("should NOT reload workloads in other namespaces when watchGlobally=false", func() {
			By("Creating a ConfigMap in another namespace")
			_, err := utils.CreateConfigMap(ctx, kubeClient, otherNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment in another namespace with auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, otherNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, otherNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap in the other namespace")
			err = utils.UpdateConfigMap(ctx, kubeClient, otherNS, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (different namespace with watchGlobally=false)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, otherNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment in other namespace should NOT reload with watchGlobally=false")
		})
	})

	Context("with watchGlobally=true flag (default)", func() {
		var globalNS string

		BeforeEach(func() {
			globalNS = "global-" + utils.RandName("ns")

			// Create test namespace
			err := utils.CreateNamespace(ctx, kubeClient, globalNS)
			Expect(err).NotTo(HaveOccurred())

			// Deploy Reloader with watchGlobally=true (default)
			err = deployReloaderWithFlags(map[string]string{
				"reloader.watchGlobally": "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = utils.DeleteDeployment(ctx, kubeClient, globalNS, deploymentName)
			_ = utils.DeleteConfigMap(ctx, kubeClient, globalNS, configMapName)
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, globalNS)
		})

		It("should reload workloads in any namespace when watchGlobally=true", func() {
			By("Creating a ConfigMap in a different namespace")
			_, err := utils.CreateConfigMap(ctx, kubeClient, globalNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment in a different namespace with auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, globalNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, globalNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, globalNS, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (watchGlobally=true)")
			reloaded, err := adapter.WaitReloaded(ctx, globalNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment in any namespace should reload with watchGlobally=true")
		})
	})
})
