package annotations

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Search and Match Annotation Tests", func() {
	var (
		deploymentName string
		configMapName  string
		workloadName   string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		workloadName = utils.RandName("workload")
		adapter = utils.NewDeploymentAdapter(kubeClient)
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
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			// Capture the reload-annotation baseline before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ReloadTimeout)
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
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			// Capture the reload-annotation baseline before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ShortTimeout)
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
				utils.WithConfigMapEnvFrom(configMapName))
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			// Capture the reload-annotation baseline before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ShortTimeout)
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
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for both Deployments to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())
			err = adapter.WaitReady(ctx, testNamespace, deploymentName2, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			// Capture the reload-annotation baselines before the trigger to avoid the
			// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
			priorReload1, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			priorReload2, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName2, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for first Deployment to be reloaded")
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload1, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with search annotation should reload")

			By("Verifying second Deployment was NOT reloaded")
			reloaded2, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName2,
				utils.AnnotationLastReloadedFrom, priorReload2, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded2).To(BeFalse(), "Deployment without search annotation should NOT reload")
		})
	})

	// TODO: Reloader currently only reads search annotations from workload metadata, not pod template.
	// This test documents the expected behavior but needs Reloader code changes to pass.
	Context("with search annotation on pod template", func() {
		PDescribeTable("should reload when search annotation is on pod template only",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

				By("Creating a ConfigMap with match annotation")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"},
					utils.BuildMatchAnnotation())
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with search annotation on pod template ONLY")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:          configMapName,
					UseConfigMapEnvFrom:    true,
					PodTemplateAnnotations: utils.BuildSearchAnnotation(),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap")
				// Capture the reload-annotation baseline before the trigger to avoid the
				// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
				priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom)
				Expect(err).NotTo(HaveOccurred())
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, priorReload, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s should reload with search annotation on pod template", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})
})
