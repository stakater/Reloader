package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Auto Reload All Flag Tests", func() {
	var (
		deploymentName string
		configMapName  string
		autoNamespace  string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		autoNamespace = "auto-" + utils.RandName("ns")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, autoNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, autoNamespace, configMapName)
	})

	Context("with autoReloadAll=true flag", func() {
		BeforeEach(func() {
			err := utils.CreateNamespace(ctx, kubeClient, autoNamespace)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{
				"reloader.autoReloadAll": "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, autoNamespace)
		})

		It("should reload workloads without any annotations when autoReloadAll is true", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, autoNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment WITHOUT any Reloader annotations")
			_, err = utils.CreateDeployment(ctx, kubeClient, autoNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, autoNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, autoNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (autoReloadAll=true)")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, autoNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment without annotations should reload when autoReloadAll=true")
		})

		It("should respect auto=false annotation even when autoReloadAll is true", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, autoNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=false annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, autoNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoFalseAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, autoNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, autoNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (auto=false overrides autoReloadAll)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, autoNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment with auto=false should NOT reload even with autoReloadAll=true")
		})
	})
})
