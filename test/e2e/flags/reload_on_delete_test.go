package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Reload On Delete Flag Tests", func() {
	var (
		deploymentName  string
		configMapName   string
		deleteNamespace string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		deleteNamespace = "delete-" + utils.RandName("ns")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, deleteNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, deleteNamespace, configMapName)
	})

	Context("with reloadOnDelete=true flag", func() {
		BeforeEach(func() {
			// Create test namespace
			err := utils.CreateNamespace(ctx, kubeClient, deleteNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Deploy Reloader with reloadOnDelete flag
			err = deployReloaderWithFlags(map[string]string{
				"reloader.reloadOnDelete": "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, deleteNamespace)
		})

		It("should reload when a referenced ConfigMap is deleted", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, deleteNamespace, configMapName,
				map[string]string{"key": "value"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with annotation for the ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, deleteNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, deleteNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the ConfigMap")
			err = utils.DeleteConfigMap(ctx, kubeClient, deleteNamespace, configMapName)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (reloadOnDelete=true)")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, deleteNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when referenced ConfigMap is deleted")
		})

		It("should reload when a referenced Secret is deleted", func() {
			secretName := utils.RandName("secret")

			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, deleteNamespace, secretName,
				map[string]string{"password": "secret"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with annotation for the Secret")
			_, err = utils.CreateDeployment(ctx, kubeClient, deleteNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildSecretReloadAnnotation(secretName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, deleteNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the Secret")
			err = utils.DeleteSecret(ctx, kubeClient, deleteNamespace, secretName)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (reloadOnDelete=true)")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, deleteNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when referenced Secret is deleted")
		})
	})

	Context("with reloadOnDelete=false (default)", func() {
		BeforeEach(func() {
			// Create test namespace
			err := utils.CreateNamespace(ctx, kubeClient, deleteNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Deploy Reloader without reloadOnDelete flag (default is false)
			err = deployReloaderWithFlags(map[string]string{})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, deleteNamespace)
		})

		It("should NOT reload when a referenced ConfigMap is deleted (default behavior)", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, deleteNamespace, configMapName,
				map[string]string{"key": "value"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with annotation for the ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, deleteNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, deleteNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the ConfigMap")
			err = utils.DeleteConfigMap(ctx, kubeClient, deleteNamespace, configMapName)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (reloadOnDelete=false)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, deleteNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload on delete when reloadOnDelete=false")
		})
	})
})
