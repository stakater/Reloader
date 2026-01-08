package core

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Workload Reload Tests", func() {
	var (
		configMapName string
		secretName    string
		workloadName  string
	)

	BeforeEach(func() {
		configMapName = utils.RandName("cm")
		secretName = utils.RandName("secret")
		workloadName = utils.RandName("workload")
	})

	AfterEach(func() {
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
	})

	// ============================================================
	// ANNOTATIONS STRATEGY TESTS
	// ============================================================
	Context("Annotations Strategy", func() {
		// Standard workloads that support annotation-based reload
		standardWorkloads := []utils.WorkloadType{
			utils.WorkloadDeployment,
			utils.WorkloadDaemonSet,
			utils.WorkloadStatefulSet,
		}

		// ConfigMap reload tests for standard workloads
		DescribeTable("should reload when ConfigMap changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with ConfigMap reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations: utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s should have been reloaded", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// Secret reload tests for standard workloads
		DescribeTable("should reload when Secret changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with Secret reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:       secretName,
					UseSecretEnvFrom: true,
					Annotations: utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret data")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s should have been reloaded", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// Auto=true annotation tests
		DescribeTable("should reload with auto=true annotation when ConfigMap changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with auto=true annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations: utils.BuildAutoTrueAnnotation(),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with auto=true should have been reloaded", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// Negative tests: label-only changes should NOT trigger reload
		DescribeTable("should NOT reload when only ConfigMap labels change (no data change)",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with ConfigMap reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations: utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the ConfigMap labels (no data change)")
				err = utils.UpdateConfigMapLabels(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload was NOT reloaded (negative test)")
				time.Sleep(utils.NegativeTestWait)
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeFalse(), "%s should NOT reload when only ConfigMap labels change", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should NOT reload when only Secret labels change (no data change)",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with Secret reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:       secretName,
					UseSecretEnvFrom: true,
					Annotations: utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the Secret labels (no data change)")
				err = utils.UpdateSecretLabels(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload was NOT reloaded (negative test)")
				time.Sleep(utils.NegativeTestWait)
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeFalse(), "%s should NOT reload when only Secret labels change", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// CronJob special handling - triggers a Job instead of annotation
		Context("CronJob (special handling)", func() {
			var cronJobAdapter *utils.CronJobAdapter

			BeforeEach(func() {
				adapter := registry.Get(utils.WorkloadCronJob)
				Expect(adapter).NotTo(BeNil())
				var ok bool
				cronJobAdapter, ok = adapter.(*utils.CronJobAdapter)
				Expect(ok).To(BeTrue(), "Should be able to cast to CronJobAdapter")
			})

			It("should trigger a Job when ConfigMap changes", func() {
				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a CronJob with ConfigMap reference annotation")
				err = cronJobAdapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = cronJobAdapter.Delete(ctx, testNamespace, workloadName) })

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for a Job to be created by CronJob reload")
				triggered, err := cronJobAdapter.WaitForTriggeredJob(ctx, testNamespace, workloadName, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(triggered).To(BeTrue(), "CronJob should have triggered a Job creation")
			})

			It("should trigger a Job when Secret changes", func() {
				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a CronJob with Secret reference annotation")
				err = cronJobAdapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:       secretName,
					UseSecretEnvFrom: true,
					Annotations:      utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = cronJobAdapter.Delete(ctx, testNamespace, workloadName) })

				By("Updating the Secret data")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for a Job to be created by CronJob reload")
				triggered, err := cronJobAdapter.WaitForTriggeredJob(ctx, testNamespace, workloadName, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(triggered).To(BeTrue(), "CronJob should have triggered a Job creation")
			})

			It("should trigger a Job with auto=true annotation when ConfigMap changes", func() {
				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a CronJob with auto=true annotation")
				err = cronJobAdapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildAutoTrueAnnotation(),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = cronJobAdapter.Delete(ctx, testNamespace, workloadName) })

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for a Job to be created by CronJob reload")
				triggered, err := cronJobAdapter.WaitForTriggeredJob(ctx, testNamespace, workloadName, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(triggered).To(BeTrue(), "CronJob with auto=true should have triggered a Job creation")
			})
		})

		// Volume mount tests
		DescribeTable("should reload when volume-mounted ConfigMap changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "setting: initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with ConfigMap volume")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:      configMapName,
					UseConfigMapVolume: true,
					Annotations:        utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "setting: updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with volume-mounted ConfigMap should have been reloaded", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should reload when volume-mounted Secret changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials.yaml": "secret: initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with Secret volume")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:      secretName,
					UseSecretVolume: true,
					Annotations:     utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret data")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials.yaml": "secret: updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with volume-mounted Secret should have been reloaded", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// Test for workloads without Reloader annotation
		DescribeTable("should NOT reload without Reloader annotation",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "value"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload WITHOUT Reloader annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					// No Reloader annotations
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload is NOT reloaded (negative test)")
				time.Sleep(utils.NegativeTestWait)
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeFalse(), "%s without Reloader annotation should NOT be reloaded", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
		)

		// Variable to track for use in lint
		_ = standardWorkloads

		// ============================================================
		// EDGE CASE TESTS (Deployment-specific)
		// ============================================================
		Context("Edge Cases", func() {
			It("should reload deployment with multiple ConfigMaps when any one changes", func() {
				configMapName2 := utils.RandName("cm2")
				defer func() { _ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2) }()

				adapter := registry.Get(utils.WorkloadDeployment)
				Expect(adapter).NotTo(BeNil())

				By("Creating two ConfigMaps")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key1": "value1"}, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
					map[string]string{"key2": "value2"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a Deployment referencing both ConfigMaps")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildConfigMapReloadAnnotation(configMapName, configMapName2),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for Deployment to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the second ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
					map[string]string{"key2": "updated-value2"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for Deployment to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded when second ConfigMap changed")
			})

			It("should reload deployment with multiple Secrets when any one changes", func() {
				secretName2 := utils.RandName("secret2")
				defer func() { _ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName2) }()

				adapter := registry.Get(utils.WorkloadDeployment)
				Expect(adapter).NotTo(BeNil())

				By("Creating two Secrets")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"key1": "value1"}, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
					map[string]string{"key2": "value2"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a Deployment referencing both Secrets")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:       secretName,
					UseSecretEnvFrom: true,
					Annotations:      utils.BuildSecretReloadAnnotation(secretName, secretName2),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for Deployment to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the second Secret")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
					map[string]string{"key2": "updated-value2"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for Deployment to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded when second Secret changed")
			})

			It("should reload deployment multiple times for sequential ConfigMap updates", func() {
				adapter := registry.Get(utils.WorkloadDeployment)
				Expect(adapter).NotTo(BeNil())

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "v1"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a Deployment with ConfigMap reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for Deployment to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("First update to ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "v2"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for first reload")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue())

				By("Getting first reload annotation value")
				deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, workloadName)
				Expect(err).NotTo(HaveOccurred())
				firstReloadValue := deploy.Spec.Template.Annotations[utils.AnnotationLastReloadedFrom]

				By("Second update to ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "v3"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for second reload with different annotation value")
				Eventually(func() string {
					deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, workloadName)
					if err != nil {
						return ""
					}
					return deploy.Spec.Template.Annotations[utils.AnnotationLastReloadedFrom]
				}, utils.ReloadTimeout, utils.DefaultInterval).ShouldNot(
					Equal(firstReloadValue),
					"Reload annotation should change after second update",
				)
			})

			It("should reload deployment when either ConfigMap or Secret changes", func() {
				adapter := registry.Get(utils.WorkloadDeployment)
				Expect(adapter).NotTo(BeNil())

				By("Creating a ConfigMap and Secret")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"secret": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a Deployment referencing both")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					SecretName:          secretName,
					UseConfigMapEnvFrom: true,
					UseSecretEnvFrom:    true,
					Annotations: utils.MergeAnnotations(
						utils.BuildConfigMapReloadAnnotation(configMapName),
						utils.BuildSecretReloadAnnotation(secretName),
					),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for Deployment to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"secret": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for Deployment to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded when Secret changed")
			})

			It("should NOT reload deployment with auto=false annotation", func() {
				adapter := registry.Get(utils.WorkloadDeployment)
				Expect(adapter).NotTo(BeNil())

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a Deployment with auto=false annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildAutoFalseAnnotation(),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for Deployment to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying Deployment is NOT reloaded (auto=false)")
				time.Sleep(utils.NegativeTestWait)
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeFalse(), "Deployment with auto=false should NOT have been reloaded")
			})
		})
	})

	// ============================================================
	// ENVVARS STRATEGY TESTS
	// ============================================================
	Context("EnvVars Strategy", Label("envvars"), Ordered, func() {
		// Redeploy Reloader with envvars strategy for this context
		BeforeAll(func() {
			By("Redeploying Reloader with envvars strategy")
			deployValues := map[string]string{
				"reloader.reloadStrategy": "env-vars",
			}
			// Preserve Argo support if available
			if utils.IsArgoRolloutsInstalled(ctx, dynamicClient) {
				deployValues["reloader.isArgoRollouts"] = "true"
			}
			err := testEnv.DeployAndWait(deployValues)
			Expect(err).NotTo(HaveOccurred(), "Failed to redeploy Reloader with envvars strategy")
		})

		AfterAll(func() {
			By("Restoring Reloader to annotations strategy")
			deployValues := map[string]string{
				"reloader.reloadStrategy": "annotations",
			}
			// Preserve Argo support if available
			if utils.IsArgoRolloutsInstalled(ctx, dynamicClient) {
				deployValues["reloader.isArgoRollouts"] = "true"
			}
			err := testEnv.DeployAndWait(deployValues)
			Expect(err).NotTo(HaveOccurred(), "Failed to restore Reloader to annotations strategy")
		})

		// EnvVar workloads (CronJob does NOT support env var strategy)
		envVarWorkloads := []utils.WorkloadType{
			utils.WorkloadDeployment,
			utils.WorkloadDaemonSet,
			utils.WorkloadStatefulSet,
		}

		DescribeTable("should add STAKATER_ env var when ConfigMap changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				if !adapter.SupportsEnvVarStrategy() {
					Skip("Workload type does not support env var strategy")
				}

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with ConfigMap reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to have STAKATER_ env var")
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName,
					utils.StakaterEnvVarPrefix, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue(), "%s should have STAKATER_ env var after ConfigMap change", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should add STAKATER_ env var when Secret changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				if !adapter.SupportsEnvVarStrategy() {
					Skip("Workload type does not support env var strategy")
				}

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with Secret reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:       secretName,
					UseSecretEnvFrom: true,
					Annotations:      utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret data")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to have STAKATER_ env var")
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName,
					utils.StakaterEnvVarPrefix, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue(), "%s should have STAKATER_ env var after Secret change", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// Negative tests for env var strategy
		DescribeTable("should NOT add STAKATER_ env var when only ConfigMap labels change",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				if !adapter.SupportsEnvVarStrategy() {
					Skip("Workload type does not support env var strategy")
				}

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "value"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with ConfigMap reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the ConfigMap labels")
				err = utils.UpdateConfigMapLabels(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload does NOT have STAKATER_ env var")
				time.Sleep(utils.NegativeTestWait)
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName,
					utils.StakaterEnvVarPrefix, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse(), "%s should NOT have STAKATER_ env var for label-only change", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
		)

		DescribeTable("should NOT add STAKATER_ env var when only Secret labels change",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				if !adapter.SupportsEnvVarStrategy() {
					Skip("Workload type does not support env var strategy")
				}

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"password": "value"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with Secret reference annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:       secretName,
					UseSecretEnvFrom: true,
					Annotations:      utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the Secret labels")
				err = utils.UpdateSecretLabels(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload does NOT have STAKATER_ env var")
				time.Sleep(utils.NegativeTestWait)
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName,
					utils.StakaterEnvVarPrefix, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse(), "%s should NOT have STAKATER_ env var for label-only change", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
		)

		// Variable to track for use in lint
		_ = envVarWorkloads
	})
})
