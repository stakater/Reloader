package flags

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Ignored Workloads Flag Tests", func() {
	var (
		cronJobName      string
		configMapName    string
		ignoreNS         string
		cronJobAdapter   *utils.CronJobAdapter
		deploymentAdater *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		cronJobName = utils.RandName("cj")
		configMapName = utils.RandName("cm")
		ignoreNS = "ignore-wl-" + utils.RandName("ns")
		cronJobAdapter = utils.NewCronJobAdapter(kubeClient)
		deploymentAdater = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteCronJob(ctx, kubeClient, ignoreNS, cronJobName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, ignoreNS, configMapName)
	})

	Context("with ignoreCronJobs=true flag", func() {
		BeforeEach(func() {
			err := utils.CreateNamespace(ctx, kubeClient, ignoreNS)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{
				"reloader.ignoreCronJobs": "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, ignoreNS)
		})

		It("should NOT reload CronJobs when ignoreCronJobs=true", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, ignoreNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a CronJob with auto annotation referencing the ConfigMap")
			_, err = utils.CreateCronJob(ctx, kubeClient, ignoreNS, cronJobName,
				utils.WithCronJobConfigMapEnvFrom(configMapName),
				utils.WithCronJobAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, ignoreNS, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying CronJob was NOT reloaded (ignoreCronJobs=true)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := cronJobAdapter.WaitReloaded(ctx, ignoreNS, cronJobName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "CronJob should NOT reload when ignoreCronJobs=true")
		})

		It("should still reload Deployments when ignoreCronJobs=true", func() {
			deploymentName := utils.RandName("deploy")

			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, ignoreNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto annotation referencing the ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, ignoreNS, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = utils.DeleteDeployment(ctx, kubeClient, ignoreNS, deploymentName)
			}()

			By("Waiting for Deployment to be ready")
			err = deploymentAdater.WaitReady(ctx, ignoreNS, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, ignoreNS, configMapName, map[string]string{"key": "updated-deploy"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded (Deployment should still work)")
			reloaded, err := deploymentAdater.WaitReloaded(ctx, ignoreNS, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should still reload with ignoreCronJobs=true")
		})
	})

	Context("with both ignoreCronJobs=true and ignoreJobs=true flags", func() {
		BeforeEach(func() {
			err := utils.CreateNamespace(ctx, kubeClient, ignoreNS)
			Expect(err).NotTo(HaveOccurred())

			err = deployReloaderWithFlags(map[string]string{
				"reloader.ignoreCronJobs": "true",
				"reloader.ignoreJobs":     "true",
			})
			Expect(err).NotTo(HaveOccurred())

			err = waitForReloaderReady()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = undeployReloader()
			_ = utils.DeleteNamespace(ctx, kubeClient, ignoreNS)
		})

		It("should NOT reload CronJobs when both job flags are true", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, ignoreNS, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a CronJob with auto annotation")
			_, err = utils.CreateCronJob(ctx, kubeClient, ignoreNS, cronJobName,
				utils.WithCronJobConfigMapEnvFrom(configMapName),
				utils.WithCronJobAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, ignoreNS, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying CronJob was NOT reloaded")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := cronJobAdapter.WaitReloaded(ctx, ignoreNS, cronJobName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "CronJob should NOT reload when ignoreCronJobs=true and ignoreJobs=true")
		})
	})
})
