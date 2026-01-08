package annotations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Exclude Annotation Tests", func() {
	var (
		deploymentName string
		configMapName  string
		configMapName2 string
		secretName     string
		secretName2    string
		excludeNS      string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		configMapName2 = utils.RandName("cm2")
		secretName = utils.RandName("secret")
		secretName2 = utils.RandName("secret2")
		excludeNS = "exclude-" + utils.RandName("ns")

		// Create test namespace
		err := utils.CreateNamespace(ctx, kubeClient, excludeNS)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, excludeNS, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, excludeNS, configMapName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, excludeNS, configMapName2)
		_ = utils.DeleteSecret(ctx, kubeClient, excludeNS, secretName)
		_ = utils.DeleteSecret(ctx, kubeClient, excludeNS, secretName2)
		_ = utils.DeleteNamespace(ctx, kubeClient, excludeNS)
	})

	Context("ConfigMap exclude annotation", func() {
		It("should NOT reload when excluded ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, excludeNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, excludeNS, configMapName2,
				map[string]string{"key2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and configmaps.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, excludeNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapExcludeAnnotation(configMapName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, excludeNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the excluded ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, excludeNS, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (excluded ConfigMap)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, excludeNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when excluded ConfigMap changes")
		})

		It("should reload when non-excluded ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, excludeNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, excludeNS, configMapName2,
				map[string]string{"key2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and configmaps.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, excludeNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapExcludeAnnotation(configMapName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, excludeNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the non-excluded ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, excludeNS, configMapName2,
				map[string]string{"key2": "updated2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, excludeNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when non-excluded ConfigMap changes")
		})
	})

	Context("Secret exclude annotation", func() {
		It("should NOT reload when excluded Secret changes", func() {
			By("Creating two Secrets")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, excludeNS, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, excludeNS, secretName2,
				map[string]string{"password2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and secrets.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, excludeNS, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithSecretEnvFrom(secretName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretExcludeAnnotation(secretName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, excludeNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the excluded Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, excludeNS, secretName,
				map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (excluded Secret)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, excludeNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when excluded Secret changes")
		})

		It("should reload when non-excluded Secret changes", func() {
			By("Creating two Secrets")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, excludeNS, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, excludeNS, secretName2,
				map[string]string{"password2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and secrets.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, excludeNS, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithSecretEnvFrom(secretName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretExcludeAnnotation(secretName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, excludeNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the non-excluded Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, excludeNS, secretName2,
				map[string]string{"password2": "updated2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, excludeNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when non-excluded Secret changes")
		})
	})
})
