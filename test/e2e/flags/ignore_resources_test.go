package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Ignore Resources Flag Tests", func() {
	var (
		deploymentName string
		configMapName  string
		secretName     string
		ignoreNS       string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		secretName = utils.RandName("secret")
		ignoreNS = "ignore-" + utils.RandName("ns")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, ignoreNS, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, ignoreNS, configMapName)
		_ = utils.DeleteSecret(ctx, kubeClient, ignoreNS, secretName)
	})

	Context("with ignoreSecrets=true flag", func() {
		BeforeEach(func() {
			// Create test namespace
			err := utils.CreateNamespace(ctx, kubeClient, ignoreNS)
			Expect(err).NotTo(HaveOccurred())

			// Deploy Reloader with ignoreSecrets flag
			err = deployReloaderWithFlags(map[string]string{
				"reloader.ignoreSecrets": "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, ignoreNS)
		})

		It("should NOT reload when Secret changes with ignoreSecrets=true", func() {
			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, ignoreNS, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto annotation referencing the Secret")
			_, err = utils.CreateDeployment(ctx, kubeClient, ignoreNS, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, ignoreNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, ignoreNS, secretName,
				map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (ignoreSecrets=true)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, ignoreNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when ignoreSecrets=true")
		})

		It("should still reload when ConfigMap changes with ignoreSecrets=true", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, ignoreNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto annotation referencing the ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, ignoreNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, ignoreNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, ignoreNS, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (ConfigMap should still work)")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, ignoreNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "ConfigMap changes should still trigger reload with ignoreSecrets=true")
		})
	})

	Context("with ignoreConfigMaps=true flag", func() {
		BeforeEach(func() {
			// Create test namespace
			err := utils.CreateNamespace(ctx, kubeClient, ignoreNS)
			Expect(err).NotTo(HaveOccurred())

			// Deploy Reloader with ignoreConfigMaps flag
			err = deployReloaderWithFlags(map[string]string{
				"reloader.ignoreConfigMaps": "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, ignoreNS)
		})

		It("should NOT reload when ConfigMap changes with ignoreConfigMaps=true", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, ignoreNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto annotation referencing the ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, ignoreNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, ignoreNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, ignoreNS, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (ignoreConfigMaps=true)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, ignoreNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when ignoreConfigMaps=true")
		})

		It("should still reload when Secret changes with ignoreConfigMaps=true", func() {
			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, ignoreNS, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto annotation referencing the Secret")
			_, err = utils.CreateDeployment(ctx, kubeClient, ignoreNS, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, ignoreNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, ignoreNS, secretName,
				map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (Secret should still work)")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, ignoreNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Secret changes should still trigger reload with ignoreConfigMaps=true")
		})
	})
})
