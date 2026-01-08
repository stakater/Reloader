package advanced

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Multi-Container Tests", func() {
	var (
		deploymentName string
		configMapName  string
		configMapName2 string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		configMapName2 = utils.RandName("cm2")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2)
	})

	Context("Multiple containers same ConfigMap", func() {
		It("should reload when ConfigMap used by multiple containers changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"shared-key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with multiple containers using the same ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithMultipleContainers(2),
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"shared-key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with multiple containers should be reloaded")
		})
	})

	Context("Multiple containers different ConfigMaps", func() {
		It("should reload when any container's ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key1": "initial1"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with multiple containers using different ConfigMaps")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithMultipleContainersAndEnv(configMapName, configMapName2),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the first ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key1": "updated1"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should be reloaded when first container's ConfigMap changes")
		})
	})
})
