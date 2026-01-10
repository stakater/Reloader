package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Namespace Ignore Flag Tests", func() {
	var (
		deploymentName   string
		configMapName    string
		ignoredNamespace string
		watchedNamespace string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		ignoredNamespace = "ignored-" + utils.RandName("ns")
		watchedNamespace = "watched-" + utils.RandName("ns")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, ignoredNamespace, deploymentName)
		_ = utils.DeleteDeployment(ctx, kubeClient, watchedNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, ignoredNamespace, configMapName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, watchedNamespace, configMapName)
	})

	Context("with ignoreNamespaces flag", func() {
		BeforeEach(func() {
			err := utils.CreateNamespace(ctx, kubeClient, ignoredNamespace)
			Expect(err).NotTo(HaveOccurred())
			err = utils.CreateNamespace(ctx, kubeClient, watchedNamespace)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{
				"reloader.ignoreNamespaces": ignoredNamespace,
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, ignoredNamespace)
			_ = utils.DeleteNamespace(ctx, kubeClient, watchedNamespace)
		})

		It("should NOT reload in ignored namespace", func() {
			By("Creating a ConfigMap in the ignored namespace")
			_, err := utils.CreateConfigMap(ctx, kubeClient, ignoredNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment in the ignored namespace")
			_, err = utils.CreateDeployment(ctx, kubeClient, ignoredNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, ignoredNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, ignoredNamespace, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (ignored namespace)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, ignoredNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment in ignored namespace should NOT be reloaded")
		})

		It("should reload in watched (non-ignored) namespace", func() {
			By("Creating a ConfigMap in the watched namespace")
			_, err := utils.CreateConfigMap(ctx, kubeClient, watchedNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment in the watched namespace")
			_, err = utils.CreateDeployment(ctx, kubeClient, watchedNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, watchedNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, watchedNamespace, configMapName,
				map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, watchedNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment in non-ignored namespace should be reloaded")
		})
	})
})
