package annotations

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Exclude Annotation Tests", func() {
	var (
		deploymentName string
		configMapName  string
		configMapName2 string
		secretName     string
		secretName2    string
		workloadName   string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		configMapName2 = utils.RandName("cm2")
		secretName = utils.RandName("secret")
		secretName2 = utils.RandName("secret2")
		workloadName = utils.RandName("workload")
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName2)
	})

	Context("ConfigMap exclude annotation", func() {
		It("should NOT reload when excluded ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and configmaps.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapExcludeAnnotation(configMapName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the excluded ConfigMap")
			// Capture the reload-annotation baseline before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (excluded ConfigMap)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when excluded ConfigMap changes")
		})

		It("should reload when non-excluded ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and configmaps.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapExcludeAnnotation(configMapName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the non-excluded ConfigMap")
			// Capture the reload-annotation baseline before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2, map[string]string{"key2": "updated2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when non-excluded ConfigMap changes")
		})
	})

	Context("Secret exclude annotation", func() {
		It("should NOT reload when excluded Secret changes", func() {
			By("Creating two Secrets")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
				map[string]string{"password2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and secrets.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithSecretEnvFrom(secretName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretExcludeAnnotation(secretName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the excluded Secret")
			// Capture the reload-annotation baseline before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (excluded Secret)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when excluded Secret changes")
		})

		It("should reload when non-excluded Secret changes", func() {
			By("Creating two Secrets")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
				map[string]string{"password2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and secrets.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithSecretEnvFrom(secretName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretExcludeAnnotation(secretName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the non-excluded Secret")
			// Capture the reload-annotation baseline before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2, map[string]string{"password2": "updated2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when non-excluded Secret changes")
		})
	})

	// TODO: Reloader currently only reads exclude annotations from workload metadata, not pod template.
	// This test documents the expected behavior but needs Reloader code changes to pass.
	Context("Exclude annotation on pod template", func() {
		PDescribeTable("should NOT reload when exclude annotation is on pod template only",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

				By("Creating two ConfigMaps")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
					map[string]string{"key2": "initial2"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with auto=true and exclude annotation on pod template ONLY")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					PodTemplateAnnotations: utils.MergeAnnotations(
						utils.BuildAutoTrueAnnotation(),
						utils.BuildConfigMapExcludeAnnotation(configMapName),
					),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the excluded ConfigMap")
				// Capture the reload-annotation baseline before the trigger to avoid the
				// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
				priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom)
				Expect(err).NotTo(HaveOccurred())
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload was NOT reloaded (excluded ConfigMap)")
				time.Sleep(utils.NegativeTestWait)
				reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, priorReload, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeFalse(), "%s should NOT reload with exclude on pod template", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})
})
