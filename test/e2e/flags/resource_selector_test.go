package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Resource Label Selector Flag Tests", func() {
	var (
		deploymentName string
		matchingCM     string
		nonMatchingCM  string
		resourceNS     string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		matchingCM = utils.RandName("match-cm")
		nonMatchingCM = utils.RandName("nomatch-cm")
		resourceNS = "resource-" + utils.RandName("ns")
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, resourceNS, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, resourceNS, matchingCM)
		_ = utils.DeleteConfigMap(ctx, kubeClient, resourceNS, nonMatchingCM)
	})

	Context("with resourceLabelSelector flag", func() {
		BeforeEach(func() {
			err := utils.CreateNamespace(ctx, kubeClient, resourceNS)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{
				"reloader.resourceLabelSelector": "reload=true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, resourceNS)
		})

		It("should reload when labeled ConfigMap changes", func() {
			By("Creating a ConfigMap with matching label")
			_, err := utils.CreateConfigMapWithLabels(ctx, kubeClient, resourceNS, matchingCM,
				map[string]string{"key": "initial"},
				map[string]string{"reload": "true"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, resourceNS, deploymentName,
				utils.WithConfigMapEnvFrom(matchingCM),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, resourceNS, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the labeled ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, resourceNS, matchingCM, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, resourceNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should be reloaded when labeled ConfigMap changes")
		})

		It("should NOT reload when unlabeled ConfigMap changes", func() {
			By("Creating a ConfigMap WITHOUT matching label")
			_, err := utils.CreateConfigMap(ctx, kubeClient, resourceNS, nonMatchingCM,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, resourceNS, deploymentName,
				utils.WithConfigMapEnvFrom(nonMatchingCM),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, resourceNS, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the unlabeled ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, resourceNS, nonMatchingCM, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (unlabeled ConfigMap)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, resourceNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when unlabeled ConfigMap changes")
		})
	})
})
