package annotations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Pause Period Tests", func() {
	var (
		deploymentName string
		configMapName  string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
	})

	Context("with pause-period annotation", func() {
		It("should pause Deployment after reload", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with pause-period annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildConfigMapReloadAnnotation(configMapName),
					utils.BuildPausePeriodAnnotation("10s"),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded")

			By("Verifying Deployment has paused-at annotation")
			paused, err := utils.WaitForDeploymentPaused(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationDeploymentPausedAt, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(paused).To(BeTrue(), "Deployment should have paused-at annotation after reload")
		})

		It("should NOT pause Deployment without pause-period annotation", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment WITHOUT pause-period annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded")

			By("Verifying Deployment does NOT have paused-at annotation")
			time.Sleep(utils.NegativeTestWait)
			paused, err := utils.WaitForDeploymentPaused(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationDeploymentPausedAt, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(paused).To(BeFalse(), "Deployment should NOT have paused-at annotation without pause-period")
		})
	})
})
