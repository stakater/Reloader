package advanced

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Reload-After Ordering Tests", func() {
	var (
		cronJobName       string
		deploymentName    string
		secretName        string
		deploymentAdapter *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		cronJobName = utils.RandName("migrate")
		deploymentName = utils.RandName("api")
		secretName = utils.RandName("secret")
		deploymentAdapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteCronJob(ctx, kubeClient, testNamespace, cronJobName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
	})

	// triggeredJob returns the Job Reloader created from the CronJob, if any. Scheduled
	// runs share the name prefix, so the manual-instantiate marker tells them apart.
	triggeredJob := func() *batchv1.Job {
		jobs, err := kubeClient.BatchV1().Jobs(testNamespace).List(ctx, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		for i := range jobs.Items {
			if strings.HasPrefix(jobs.Items[i].Name, cronJobName+"-") &&
				jobs.Items[i].Annotations["cronjob.kubernetes.io/instantiate"] == "manual" {
				return &jobs.Items[i]
			}
		}
		return nil
	}

	jobHasCondition := func(job *batchv1.Job, condition batchv1.JobConditionType) bool {
		for _, c := range job.Status.Conditions {
			if c.Type == condition && c.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}

	setup := func(cronJobOpts ...utils.CronJobOption) string {
		By("Creating a Secret")
		_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
			map[string]string{"DB_PASSWORD": "initial"}, nil)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a CronJob that reloads on the Secret")
		opts := append([]utils.CronJobOption{
			utils.WithCronJobSecretEnvFrom(secretName),
			utils.WithCronJobAnnotations(utils.BuildSecretReloadAnnotation(secretName)),
		}, cronJobOpts...)
		_, err = utils.CreateCronJob(ctx, kubeClient, testNamespace, cronJobName, opts...)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a Deployment that reloads after the CronJob")
		annotations := utils.BuildSecretReloadAnnotation(secretName)
		annotations[utils.AnnotationReloadAfter] = "cronjob/" + cronJobName
		_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
			utils.WithSecretEnvFrom(secretName), utils.WithAnnotations(annotations))
		Expect(err).NotTo(HaveOccurred())
		Expect(deploymentAdapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)).To(Succeed())

		prior, _ := deploymentAdapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
		return prior
	}

	It("should restart the Deployment only after the CronJob's job completes", func() {
		prior := setup(utils.WithCronJobCommand("sleep 20"))

		By("Updating the Secret")
		Expect(utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
			map[string]string{"DB_PASSWORD": "updated"})).To(Succeed())

		By("Verifying the Deployment waits while the job runs")
		Eventually(triggeredJob, utils.ReloadTimeout, time.Second).ShouldNot(BeNil())
		reloaded, err := deploymentAdapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
			utils.AnnotationLastReloadedFrom, prior, 10*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded).To(BeFalse(), "Deployment must not reload while its reload-after dependency runs")

		By("Waiting for the Deployment to reload after the job completes")
		reloaded, err = deploymentAdapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
			utils.AnnotationLastReloadedFrom, prior, 90*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded).To(BeTrue(), "Deployment should reload once the job completed")
		Expect(jobHasCondition(triggeredJob(), batchv1.JobComplete)).To(BeTrue(),
			"the triggered job should be complete when the Deployment reloads")
	})

	It("should not restart the Deployment when the CronJob's job fails", func() {
		prior := setup(utils.WithCronJobCommand("exit 1"), utils.WithCronJobFailFast())

		By("Updating the Secret")
		Expect(utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
			map[string]string{"DB_PASSWORD": "updated"})).To(Succeed())

		By("Waiting for the triggered job to fail")
		Eventually(func() bool {
			job := triggeredJob()
			return job != nil && jobHasCondition(job, batchv1.JobFailed)
		}, 90*time.Second, time.Second).Should(BeTrue())

		By("Waiting for Reloader to drop the deferred reload")
		Eventually(func() bool {
			d, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
			Expect(err).NotTo(HaveOccurred())
			_, pending := d.Annotations[utils.AnnotationPendingReload]
			return pending
		}, 60*time.Second, time.Second).Should(BeFalse())

		current, _ := deploymentAdapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
		Expect(current).To(Equal(prior), "Deployment must not reload when its reload-after dependency failed")
	})
})
