package annotations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Combination Annotation Tests", func() {
	var (
		deploymentName string
		configMapName  string
		configMapName2 string
		secretName     string
		secretName2    string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		configMapName2 = utils.RandName("cm2")
		secretName = utils.RandName("secret")
		secretName2 = utils.RandName("secret2")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName2)
	})

	Context("auto=true with explicit reload annotations", func() {
		It("should reload when both auto-detected and explicitly listed ConfigMaps change", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"extra": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true AND explicit reload annotation for extra ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName), // auto-detected
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapReloadAnnotation(configMapName2), // explicitly listed
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the auto-detected ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when auto-detected ConfigMap changes")
		})

		It("should reload when explicitly listed ConfigMap changes with auto=true", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"extra": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true AND explicit reload annotation for extra ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName), // auto-detected
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapReloadAnnotation(configMapName2), // explicitly listed
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the explicitly listed ConfigMap (not mounted)")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2, map[string]string{"extra": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when explicitly listed ConfigMap changes")
		})

		It("should reload when Secret changes with auto=true and explicit Secret annotation", func() {
			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
				map[string]string{"api-key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true AND explicit reload annotation for extra Secret")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithSecretEnvFrom(secretName), // auto-detected
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretReloadAnnotation(secretName2), // explicitly listed
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the explicitly listed Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2, map[string]string{"api-key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when explicitly listed Secret changes")
		})
	})

	Context("auto=true with exclude annotations", func() {
		It("should NOT reload when excluded ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"excluded": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true AND exclude for second ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2), // also mounted, but excluded
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapExcludeAnnotation(configMapName2), // exclude this one
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the excluded ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2, map[string]string{"excluded": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when excluded ConfigMap changes")
		})

		It("should reload when non-excluded ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"excluded": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true AND exclude for second ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapExcludeAnnotation(configMapName2),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the non-excluded ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when non-excluded ConfigMap changes")
		})

		It("should NOT reload when excluded Secret changes", func() {
			By("Creating two Secrets")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
				map[string]string{"excluded": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true AND exclude for second Secret")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithSecretEnvFrom(secretName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretExcludeAnnotation(secretName2),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the excluded Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2, map[string]string{"excluded": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when excluded Secret changes")
		})
	})

	Context("multiple explicit references", func() {
		It("should reload when any of multiple explicitly listed ConfigMaps change", func() {
			By("Creating multiple ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key1": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with multiple ConfigMaps in reload annotation (comma-separated)")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName, configMapName2)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the second ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2, map[string]string{"key2": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when any of the listed ConfigMaps changes")
		})

		It("should reload when any of multiple explicitly listed Secrets change", func() {
			By("Creating multiple Secrets")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"key1": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
				map[string]string{"key2": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with multiple Secrets in reload annotation (comma-separated)")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildSecretReloadAnnotation(secretName, secretName2)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the first Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"key1": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when any of the listed Secrets changes")
		})

		It("should reload when both ConfigMap and Secret annotations are present", func() {
			By("Creating a ConfigMap and a Secret")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with both ConfigMap and Secret reload annotations")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildConfigMapReloadAnnotation(configMapName),
					utils.BuildSecretReloadAnnotation(secretName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when Secret changes with both annotations present")
		})
	})
})
