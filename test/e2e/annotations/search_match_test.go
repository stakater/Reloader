package annotations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Search and Match Annotation Tests", func() {
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

	Context("with search and match annotations", func() {
		It("should reload when workload has search annotation and ConfigMap has match annotation", func() {
			By("Creating a ConfigMap with match annotation")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"},
				utils.BuildMatchAnnotation())
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with search annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildSearchAnnotation()),
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
			Expect(reloaded).To(BeTrue(), "Deployment with search annotation should reload when ConfigMap has match annotation")
		})

		It("should NOT reload when workload has search but ConfigMap has no match", func() {
			By("Creating a ConfigMap WITHOUT match annotation")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with search annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildSearchAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when ConfigMap lacks match annotation")
		})

		It("should NOT reload when resource has match but no Deployment has search", func() {
			By("Creating a ConfigMap WITH match annotation")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"},
				utils.BuildMatchAnnotation())
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment WITHOUT search annotation (only standard annotation)")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				// Note: No search or reload annotation - deployment won't be affected by match
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment without search annotation should NOT reload even when ConfigMap has match")
		})

		It("should reload only the deployment with search annotation when multiple deployments use same ConfigMap", func() {
			deploymentName2 := utils.RandName("deploy2")
			defer func() {
				_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName2)
			}()

			By("Creating a ConfigMap with match annotation")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"},
				utils.BuildMatchAnnotation())
			Expect(err).NotTo(HaveOccurred())

			By("Creating first Deployment WITH search annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildSearchAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Creating second Deployment WITHOUT search annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName2,
				utils.WithConfigMapEnvFrom(configMapName),
				// No search annotation
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for both Deployments to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())
			err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName2, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for first Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with search annotation should reload")

			By("Verifying second Deployment was NOT reloaded")
			reloaded2, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName2,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded2).To(BeFalse(), "Deployment without search annotation should NOT reload")
		})
	})
})
