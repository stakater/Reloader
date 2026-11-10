package annotations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Sync Windows Tests", func() {
	var (
		deploymentName string
		configMapName  string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
	})

	// reloadAfterUpdate updates the ConfigMap and reports whether the Deployment reloads.
	reloadAfterUpdate := func(expectReload bool) bool {
		By("Waiting for Deployment to be ready")
		err := adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
		Expect(err).NotTo(HaveOccurred())

		By("Updating the ConfigMap data")
		priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
		Expect(err).NotTo(HaveOccurred())
		err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
		Expect(err).NotTo(HaveOccurred())

		timeout := utils.ReloadTimeout
		if !expectReload {
			time.Sleep(utils.NegativeTestWait)
			timeout = utils.ShortTimeout
		}
		reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
			utils.AnnotationLastReloadedFrom, priorReload, timeout)
		Expect(err).NotTo(HaveOccurred())
		return reloaded
	}

	Context("with sync-windows annotation", func() {
		BeforeEach(func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should defer the reload while a deny window is active", func() {
			By("Creating a Deployment with a deny window that is always active")
			_, err := utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildConfigMapReloadAnnotation(configMapName),
					utils.BuildSyncWindowsAnnotation(utils.SyncWindowsAlwaysDenied),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(reloadAfterUpdate(false)).To(BeFalse(), "Deployment should NOT reload while a deny window is active")

			By("Verifying the deferred reload is recorded on the Deployment")
			Eventually(func() string {
				deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
				if err != nil {
					return ""
				}
				return deploy.Annotations[utils.AnnotationPendingReload]
			}, utils.ShortTimeout).Should(ContainSubstring(configMapName), "Deployment should record the pending reload")

			By("Verifying a ReloadDeferred event was recorded")
			Eventually(func() bool {
				events, err := kubeClient.EventsV1().Events(testNamespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return false
				}
				for _, event := range events.Items {
					if event.Reason == "ReloadDeferred" && event.Regarding.Name == deploymentName {
						return true
					}
				}
				return false
			}, utils.ShortTimeout).Should(BeTrue(), "Deployment should have a ReloadDeferred event")
		})

		It("should reload a deferred Deployment once its sync windows are removed", func() {
			By("Creating a Deployment with a deny window that is always active")
			_, err := utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildConfigMapReloadAnnotation(configMapName),
					utils.BuildSyncWindowsAnnotation(utils.SyncWindowsAlwaysDenied),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(reloadAfterUpdate(false)).To(BeFalse(), "Deployment should NOT reload while a deny window is active")
			priorReload, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())

			By("Removing the sync-windows annotation")
			Eventually(func() error {
				deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
				if err != nil {
					return err
				}
				delete(deploy.Annotations, utils.AnnotationSyncWindows)
				_, err = kubeClient.AppsV1().Deployments(testNamespace).Update(ctx, deploy, metav1.UpdateOptions{})
				return err
			}, utils.ShortTimeout).Should(Succeed())

			By("Waiting for the deferred reload")
			reloaded, err := adapter.WaitReloadedFrom(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload once nothing defers it")

			By("Verifying the pending reload was cleared")
			Eventually(func() bool {
				deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
				if err != nil {
					return false
				}
				_, pending := deploy.Annotations[utils.AnnotationPendingReload]
				return !pending
			}, utils.ShortTimeout).Should(BeTrue(), "Deployment should no longer record a pending reload")
		})

		It("should reload inside an allow window", func() {
			By("Creating a Deployment with an allow window that is always active")
			_, err := utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildConfigMapReloadAnnotation(configMapName),
					utils.BuildSyncWindowsAnnotation(utils.SyncWindowsAlwaysAllowed),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(reloadAfterUpdate(true)).To(BeTrue(), "Deployment should reload inside an allow window")
		})

		It("should defer the reload when the deny window is on the pod template", func() {
			By("Creating a Deployment with a deny window on the pod template only")
			_, err := utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildConfigMapReloadAnnotation(configMapName)),
				utils.WithPodTemplateAnnotations(utils.BuildSyncWindowsAnnotation(utils.SyncWindowsAlwaysDenied)),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(reloadAfterUpdate(false)).To(BeFalse(), "Deployment should NOT reload while a pod template deny window is active")
		})

		It("should reload when the sync-windows annotation is invalid", func() {
			By("Creating a Deployment with an unparseable sync-windows annotation")
			_, err := utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildConfigMapReloadAnnotation(configMapName),
					utils.BuildSyncWindowsAnnotation("not a list"),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(reloadAfterUpdate(true)).To(BeTrue(), "Deployment should reload when its sync windows cannot be parsed")
		})
	})
})
