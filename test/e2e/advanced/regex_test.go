package advanced

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Regex Pattern Tests", func() {
	var (
		deploymentName string
		matchingCM     string
		nonMatchingCM  string
		matchingSecret string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		matchingCM = "app-config-" + utils.RandName("cm")
		nonMatchingCM = "other-" + utils.RandName("cm")
		matchingSecret = "app-secret-" + utils.RandName("secret")
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, matchingCM)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, nonMatchingCM)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, matchingSecret)
	})

	Context("ConfigMap regex pattern", func() {
		It("should reload when ConfigMap matching pattern changes", func() {
			By("Creating a ConfigMap matching the pattern")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, matchingCM,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with ConfigMap pattern annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(matchingCM),
				utils.WithAnnotations(map[string]string{
					utils.AnnotationConfigMapReload: "app-config-.*",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the matching ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, matchingCM, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should be reloaded when matching ConfigMap changes")
		})

		It("should NOT reload when ConfigMap NOT matching pattern changes", func() {
			By("Creating ConfigMaps - one matching, one not")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, matchingCM,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, nonMatchingCM,
				map[string]string{"other": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with ConfigMap pattern annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(matchingCM),
				utils.WithAnnotations(map[string]string{
					utils.AnnotationConfigMapReload: "app-config-.*",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the non-matching ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, nonMatchingCM, map[string]string{"other": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (pattern mismatch)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when non-matching ConfigMap changes")
		})
	})

	Context("Secret regex pattern", func() {
		It("should reload when Secret matching pattern changes", func() {
			By("Creating a Secret matching the pattern")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, matchingSecret,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with Secret pattern annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithSecretEnvFrom(matchingSecret),
				utils.WithAnnotations(map[string]string{
					utils.AnnotationSecretReload: "app-secret-.*",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the matching Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, matchingSecret, map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should be reloaded when matching Secret changes")
		})
	})
})
