package annotations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Auto Reload Annotation Tests", func() {
	var (
		deploymentName string
		configMapName  string
		secretName     string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		secretName = utils.RandName("secret")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
	})

	Context("with reloader.stakater.com/auto=true annotation", func() {
		It("should reload Deployment when any referenced ConfigMap changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with auto=true should have been reloaded")
		})

		It("should reload Deployment when any referenced Secret changes", func() {
			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret data")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with auto=true should have been reloaded for Secret change")
		})

		It("should reload Deployment when either ConfigMap or Secret changes", func() {
			By("Creating a ConfigMap and Secret")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true annotation referencing both")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with auto=true should have been reloaded for ConfigMap change")
		})
	})

	Context("with reloader.stakater.com/auto=false annotation", func() {
		It("should NOT reload Deployment when ConfigMap changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=false annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoFalseAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment is NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment with auto=false should NOT have been reloaded")
		})
	})

	Context("with configmap.reloader.stakater.com/auto=true annotation", func() {
		It("should reload Deployment only when ConfigMap changes, not Secret", func() {
			By("Creating a ConfigMap and Secret")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with configmap auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildConfigMapAutoAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for ConfigMap change")
		})
	})

	Context("with secret.reloader.stakater.com/auto=true annotation", func() {
		It("should reload Deployment only when Secret changes, not ConfigMap", func() {
			By("Creating a ConfigMap and Secret")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with secret auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildSecretAutoAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for Secret change")
		})
	})

	Context("with auto annotation and explicit reload annotation together", func() {
		It("should reload when auto-detected resource changes", func() {
			configMapName2 := utils.RandName("cm2")
			defer func() { _ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2) }()

			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key1": "value1"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "value2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and explicit reload for first ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapReloadAnnotation(configMapName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the second ConfigMap (auto-detected)")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for auto-detected ConfigMap change")
		})
	})
})
