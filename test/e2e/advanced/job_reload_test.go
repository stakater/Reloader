package advanced

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Job Workload Recreation Tests", func() {
	var (
		jobName       string
		configMapName string
		secretName    string
	)

	BeforeEach(func() {
		jobName = utils.RandName("job")
		configMapName = utils.RandName("cm")
		secretName = utils.RandName("secret")
	})

	AfterEach(func() {
		_ = utils.DeleteJob(ctx, kubeClient, testNamespace, jobName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
	})

	Context("Job with ConfigMap reference", func() {
		It("should recreate Job when referenced ConfigMap changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"JOB_CONFIG": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Job with ConfigMap envFrom")
			job, err := utils.CreateJob(ctx, kubeClient, testNamespace, jobName,
				utils.WithJobConfigMapEnvFrom(configMapName),
				utils.WithJobAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)))
			Expect(err).NotTo(HaveOccurred())
			originalUID := string(job.UID)

			By("Waiting for Job to exist")
			err = utils.WaitForJobExists(ctx, kubeClient, testNamespace, jobName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"JOB_CONFIG": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Job to be recreated (new UID)")
			_, recreated, err := utils.WaitForJobRecreated(ctx, kubeClient, testNamespace, jobName, originalUID,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(recreated).To(BeTrue(), "Job should be recreated with new UID when ConfigMap changes")
		})
	})

	Context("Job with Secret reference", func() {
		It("should recreate Job when referenced Secret changes", func() {
			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"JOB_SECRET": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Job with Secret envFrom")
			job, err := utils.CreateJob(ctx, kubeClient, testNamespace, jobName, utils.WithJobSecretEnvFrom(secretName),
				utils.WithJobAnnotations(utils.BuildSecretReloadAnnotation(secretName)))
			Expect(err).NotTo(HaveOccurred())
			originalUID := string(job.UID)

			By("Waiting for Job to exist")
			err = utils.WaitForJobExists(ctx, kubeClient, testNamespace, jobName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"JOB_SECRET": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Job to be recreated (new UID)")
			_, recreated, err := utils.WaitForJobRecreated(ctx, kubeClient, testNamespace, jobName, originalUID,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(recreated).To(BeTrue(), "Job should be recreated with new UID when Secret changes")
		})
	})

	Context("Job with auto annotation", func() {
		It("should recreate Job with auto=true when ConfigMap changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"AUTO_CONFIG": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Job with auto annotation")
			job, err := utils.CreateJob(ctx, kubeClient, testNamespace, jobName,
				utils.WithJobConfigMapEnvFrom(configMapName),
				utils.WithJobAnnotations(utils.BuildAutoTrueAnnotation()))
			Expect(err).NotTo(HaveOccurred())
			originalUID := string(job.UID)

			By("Waiting for Job to exist")
			err = utils.WaitForJobExists(ctx, kubeClient, testNamespace, jobName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"AUTO_CONFIG": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Job to be recreated (new UID)")
			_, recreated, err := utils.WaitForJobRecreated(ctx, kubeClient, testNamespace, jobName, originalUID,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(recreated).To(BeTrue(), "Job with auto=true should be recreated when ConfigMap changes")
		})
	})

	Context("Job with valueFrom ConfigMap reference", func() {
		It("should recreate Job when ConfigMap referenced via valueFrom changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config_key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Job with valueFrom.configMapKeyRef")
			job, err := utils.CreateJob(ctx, kubeClient, testNamespace, jobName,
				utils.WithJobConfigMapKeyRef(configMapName, "config_key", "MY_CONFIG"),
				utils.WithJobAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)))
			Expect(err).NotTo(HaveOccurred())
			originalUID := string(job.UID)

			By("Waiting for Job to exist")
			err = utils.WaitForJobExists(ctx, kubeClient, testNamespace, jobName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config_key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Job to be recreated (new UID)")
			_, recreated, err := utils.WaitForJobRecreated(ctx, kubeClient, testNamespace, jobName, originalUID,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(recreated).To(BeTrue(),
				"Job with valueFrom.configMapKeyRef should be recreated when ConfigMap changes")
		})
	})

	Context("Job with valueFrom Secret reference", func() {
		It("should recreate Job when Secret referenced via valueFrom changes", func() {
			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret_key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Job with valueFrom.secretKeyRef")
			job, err := utils.CreateJob(ctx, kubeClient, testNamespace, jobName,
				utils.WithJobSecretKeyRef(secretName, "secret_key", "MY_SECRET"),
				utils.WithJobAnnotations(utils.BuildSecretReloadAnnotation(secretName)))
			Expect(err).NotTo(HaveOccurred())
			originalUID := string(job.UID)

			By("Waiting for Job to exist")
			err = utils.WaitForJobExists(ctx, kubeClient, testNamespace, jobName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret_key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Job to be recreated (new UID)")
			_, recreated, err := utils.WaitForJobRecreated(ctx, kubeClient, testNamespace, jobName, originalUID,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(recreated).To(BeTrue(), "Job with valueFrom.secretKeyRef should be recreated when Secret changes")
		})
	})
})
