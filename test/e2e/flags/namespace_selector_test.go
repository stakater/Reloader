package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Namespace Selector Flag Tests", func() {
	var (
		deploymentName string
		configMapName  string
		matchingNS     string
		nonMatchingNS  string
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		matchingNS = "match-" + utils.RandName("ns")
		nonMatchingNS = "nomatch-" + utils.RandName("ns")
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, matchingNS, deploymentName)
		_ = utils.DeleteDeployment(ctx, kubeClient, nonMatchingNS, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, matchingNS, configMapName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, nonMatchingNS, configMapName)
	})

	Context("with namespaceSelector flag", func() {
		BeforeEach(func() {
			err := utils.CreateNamespaceWithLabels(ctx, kubeClient, matchingNS, map[string]string{"env": "test"})
			Expect(err).NotTo(HaveOccurred())

			err = utils.CreateNamespace(ctx, kubeClient, nonMatchingNS)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{
				"reloader.namespaceSelector": "env=test",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, matchingNS)
			_ = utils.DeleteNamespace(ctx, kubeClient, nonMatchingNS)
		})

		It("should reload workloads in matching namespaces", func() {
			By("Creating a ConfigMap in matching namespace")
			_, err := utils.CreateConfigMap(ctx, kubeClient, matchingNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment in matching namespace with auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, matchingNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, matchingNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, matchingNS, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, matchingNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment in matching namespace should be reloaded")
		})

		It("should NOT reload workloads in non-matching namespaces", func() {
			By("Creating a ConfigMap in non-matching namespace")
			_, err := utils.CreateConfigMap(ctx, kubeClient, nonMatchingNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment in non-matching namespace with auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, nonMatchingNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = utils.WaitForDeploymentReady(ctx, kubeClient, nonMatchingNS, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, nonMatchingNS, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (non-matching namespace)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, nonMatchingNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment in non-matching namespace should NOT be reloaded")
		})
	})
})
