package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Reload On Create Flag Tests", func() {
	var (
		deploymentName  string
		configMapName   string
		createNamespace string
		adapter         *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		createNamespace = "create-" + utils.RandName("ns")
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, createNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, createNamespace, configMapName)
	})

	Context("with reloadOnCreate=true flag", func() {
		BeforeEach(func() {
			err := utils.CreateNamespace(ctx, kubeClient, createNamespace)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{
				"reloader.reloadOnCreate": "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, createNamespace)
		})

		It("should reload when a new ConfigMap is created", func() {
			By("Creating a Deployment with annotation for a ConfigMap that doesn't exist yet")
			_, err := utils.CreateDeployment(ctx, kubeClient, createNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, createNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Creating the ConfigMap that the Deployment references")
			_, err = utils.CreateConfigMap(ctx, kubeClient, createNamespace, configMapName,
				map[string]string{"key": "value"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (reloadOnCreate=true)")
			reloaded, err := adapter.WaitReloaded(ctx, createNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when referenced ConfigMap is created")
		})

		It("should reload when a new Secret is created", func() {
			secretName := utils.RandName("secret")
			defer func() { _ = utils.DeleteSecret(ctx, kubeClient, createNamespace, secretName) }()

			By("Creating a Deployment with annotation for a Secret that doesn't exist yet")
			_, err := utils.CreateDeployment(ctx, kubeClient, createNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildSecretReloadAnnotation(secretName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, createNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Creating the Secret that the Deployment references")
			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, createNamespace, secretName,
				map[string]string{"password": "secret"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (reloadOnCreate=true)")
			reloaded, err := adapter.WaitReloaded(ctx, createNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when referenced Secret is created")
		})
	})

	Context("with reloadOnCreate=false (default)", func() {
		BeforeEach(func() {
			err := utils.CreateNamespace(ctx, kubeClient, createNamespace)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, createNamespace)
		})

		It("should NOT reload when a new ConfigMap is created (default behavior)", func() {
			By("Creating a Deployment with annotation for a ConfigMap that doesn't exist yet")
			_, err := utils.CreateDeployment(ctx, kubeClient, createNamespace, deploymentName,
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, createNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Creating the ConfigMap that the Deployment references")
			_, err = utils.CreateConfigMap(ctx, kubeClient, createNamespace, configMapName,
				map[string]string{"key": "value"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (reloadOnCreate=false)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, createNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload on create when reloadOnCreate=false")
		})
	})
})
