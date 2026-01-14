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
		configMapName   string
		secretName      string
		workloadName    string
		spcName         string
		vaultSecretPath string
	)

	BeforeEach(func() {
		configMapName = utils.RandName("cm")
		secretName = utils.RandName("secret")
		workloadName = utils.RandName("workload")
		spcName = utils.RandName("spc")
		vaultSecretPath = fmt.Sprintf("secret/%s", utils.RandName("test"))
	})

	AfterEach(func() {
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
		if csiClient != nil {
			_ = utils.DeleteSecretProviderClass(ctx, csiClient, testNamespace, spcName)
		}
		_ = utils.DeleteVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath)
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
		DescribeTable("should reload when ConfigMap changes", func(workloadType utils.WorkloadType) {
			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
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
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for workload to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom,
				utils.ReloadTimeout)
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
		DescribeTable("should reload when Secret changes", func(workloadType utils.WorkloadType) {
			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
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
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret data")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for workload to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "%s should have been reloaded", workloadType)
		},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// SecretProviderClassPodStatus (CSI) reload tests with real Vault
		DescribeTable("should reload when SecretProviderClassPodStatus changes", func(workloadType utils.WorkloadType) {
			if !utils.IsCSIDriverInstalled(ctx, csiClient) {
				Skip("CSI secrets store driver not installed")
			}
			if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
				Skip("Vault CSI provider not installed")
			}

			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
			}

			By("Creating a secret in Vault")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName, vaultSecretPath,
				"api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating workload with CSI volume and SPC reload annotation")
			err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
				SPCName:      spcName,
				UseCSIVolume: true,
				Annotations:  utils.BuildSecretProviderClassReloadAnnotation(spcName),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

			By("Waiting for workload to be ready")
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, workloadName,
				utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Found SPCPS: %s\n", spcpsName)

			By("Getting initial SPCPS version")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Initial SPCPS version: %s\n", initialVersion)

			By("Updating the Vault secret")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync the new secret version")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion,
				10*time.Second)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("CSI driver synced new secret version")

			By("Waiting for workload to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "%s should have been reloaded when Vault secret changed", workloadType)
		}, Entry("Deployment", Label("csi"), utils.WorkloadDeployment),
			Entry("DaemonSet", Label("csi"), utils.WorkloadDaemonSet),
			Entry("StatefulSet", Label("csi"), utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("csi", "argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("csi", "openshift"), utils.WorkloadDeploymentConfig),
		)

		// Auto=true annotation tests
		DescribeTable("should reload with auto=true annotation when ConfigMap changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with auto=true annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:       configMapName,
					UseConfigMapEnvFrom: true,
					Annotations:         utils.BuildAutoTrueAnnotation(),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap data")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
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
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
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
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the ConfigMap labels (no data change)")
				err = utils.UpdateConfigMapLabels(ctx, kubeClient, testNamespace, configMapName, map[string]string{"new-label": "new-value"})
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
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
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
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the Secret labels (no data change)")
				err = utils.UpdateSecretLabels(ctx, kubeClient, testNamespace, secretName, map[string]string{"new-label": "new-value"})
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

		// Negative test: SPCPS label-only changes should NOT trigger reload
		DescribeTable("should NOT reload when only SecretProviderClassPodStatus labels change",
			func(workloadType utils.WorkloadType) {
				if !utils.IsCSIDriverInstalled(ctx, csiClient) {
					Skip("CSI secrets store driver not installed")
				}
				if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
					Skip("Vault CSI provider not installed")
				}

				adapter := registry.Get(workloadType)
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

				By("Creating a secret in Vault")
				err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Creating a SecretProviderClass pointing to Vault secret")
				_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
					vaultSecretPath, "api_key")
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with CSI volume and SPC reload annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SPCName:      spcName,
					UseCSIVolume: true,
					Annotations:  utils.BuildSecretProviderClassReloadAnnotation(spcName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Finding the SPCPS created by CSI driver")
				spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, workloadName,
					utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the SPCPS labels (no objects change)")
				err = utils.UpdateSecretProviderClassPodStatusLabels(ctx, csiClient, testNamespace, spcpsName, map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload was NOT reloaded (negative test)")
				time.Sleep(utils.NegativeTestWait)
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeFalse(), "%s should NOT reload when only SPCPS labels change", workloadType)
			}, Entry("Deployment", Label("csi"), utils.WorkloadDeployment),
			Entry("DaemonSet", Label("csi"), utils.WorkloadDaemonSet),
			Entry("StatefulSet", Label("csi"), utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("csi", "argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("csi", "openshift"), utils.WorkloadDeploymentConfig),
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
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for a Job to be created by CronJob reload")
				triggered, err := cronJobAdapter.WaitForTriggeredJob(ctx, testNamespace, workloadName,
					utils.ReloadTimeout)
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
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for a Job to be created by CronJob reload")
				triggered, err := cronJobAdapter.WaitForTriggeredJob(ctx, testNamespace, workloadName,
					utils.ReloadTimeout)
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
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for a Job to be created by CronJob reload")
				triggered, err := cronJobAdapter.WaitForTriggeredJob(ctx, testNamespace, workloadName,
					utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(triggered).To(BeTrue(), "CronJob with auto=true should have triggered a Job creation")
			})
		})

		// Volume mount tests
		DescribeTable("should reload when volume-mounted ConfigMap changes", func(workloadType utils.WorkloadType) {
			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
			}

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
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"config.yaml": "setting: updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for workload to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "%s with volume-mounted ConfigMap should have been reloaded", workloadType)
		},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should reload when volume-mounted Secret changes", func(workloadType utils.WorkloadType) {
			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
			}

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
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret data")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"credentials.yaml": "secret: updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for workload to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom,
				utils.ReloadTimeout)
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
		DescribeTable("should NOT reload without Reloader annotation", func(workloadType utils.WorkloadType) {
			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
			}

			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "value"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating workload WITHOUT Reloader annotation")
			err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
				ConfigMapName:       configMapName,
				UseConfigMapEnvFrom: true, // No Reloader annotations
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

			By("Waiting for workload to be ready")
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying workload is NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName, utils.AnnotationLastReloadedFrom,
				utils.ShortTimeout)
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
		// EDGE CASE TESTS
		// These tests verify edge cases that should work across all workload types.
		// ============================================================
		Context("Edge Cases", func() {
			DescribeTable("should reload with multiple ConfigMaps when any one changes",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					configMapName2 := utils.RandName("cm2")
					DeferCleanup(func() { _ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2) })

					By("Creating two ConfigMaps")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"key1": "value1"}, nil)
					Expect(err).NotTo(HaveOccurred())

					_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
						map[string]string{"key2": "value2"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload referencing both ConfigMaps")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						ConfigMapName:       configMapName,
						UseConfigMapEnvFrom: true,
						Annotations:         utils.BuildConfigMapReloadAnnotation(configMapName, configMapName2),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the second ConfigMap")
					err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2, map[string]string{"key2": "updated-value2"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload when second ConfigMap changes", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload with multiple Secrets when any one changes",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					secretName2 := utils.RandName("secret2")
					DeferCleanup(func() { _ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName2) })

					By("Creating two Secrets")
					_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
						map[string]string{"key1": "value1"}, nil)
					Expect(err).NotTo(HaveOccurred())

					_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2,
						map[string]string{"key2": "value2"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload referencing both Secrets")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						SecretName:       secretName,
						UseSecretEnvFrom: true,
						Annotations:      utils.BuildSecretReloadAnnotation(secretName, secretName2),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the second Secret")
					err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2, map[string]string{"key2": "updated-value2"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload when second Secret changes", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload multiple times for sequential ConfigMap updates",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a ConfigMap")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"key": "v1"}, nil)
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
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("First update to ConfigMap")
					err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "v2"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for first reload")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue())

					By("Getting first reload annotation value")
					firstReloadValue, err := adapter.GetPodTemplateAnnotation(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom)
					Expect(err).NotTo(HaveOccurred())

					By("Second update to ConfigMap")
					err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "v3"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for second reload with different annotation value")
					Eventually(func() string {
						val, _ := adapter.GetPodTemplateAnnotation(ctx, testNamespace, workloadName,
							utils.AnnotationLastReloadedFrom)
						return val
					}, utils.ReloadTimeout, utils.DefaultInterval).ShouldNot(Equal(firstReloadValue),
						"Reload annotation should change after second update")
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload when either ConfigMap or Secret changes",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a ConfigMap and Secret")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"config": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
						map[string]string{"secret": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload referencing both")
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

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the Secret")
					err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"secret": "updated"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload when Secret changes", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should NOT reload with auto=false annotation",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a ConfigMap")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"key": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with auto=false annotation")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						ConfigMapName:       configMapName,
						UseConfigMapEnvFrom: true,
						Annotations:         utils.BuildAutoFalseAnnotation(),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the ConfigMap data")
					err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
					Expect(err).NotTo(HaveOccurred())

					By("Verifying workload is NOT reloaded (auto=false)")
					time.Sleep(utils.NegativeTestWait)
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeFalse(), "%s with auto=false should NOT be reloaded", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)
		})

		// ============================================================
		// POD TEMPLATE ANNOTATION TESTS
		// These tests verify that annotations placed on the pod template
		// (spec.template.metadata.annotations) work the same as annotations
		// placed on the workload metadata (metadata.annotations).
		// ============================================================
		Context("Pod Template Annotations", func() {
			DescribeTable("should reload when ConfigMap annotation is on pod template only",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a ConfigMap")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"key": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with ConfigMap annotation on pod template ONLY")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						ConfigMapName:          configMapName,
						UseConfigMapEnvFrom:    true,
						PodTemplateAnnotations: utils.BuildConfigMapReloadAnnotation(configMapName),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the ConfigMap")
					err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload with pod template annotation", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload when Secret annotation is on pod template only",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a Secret")
					_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
						map[string]string{"password": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with Secret annotation on pod template ONLY")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						SecretName:             secretName,
						UseSecretEnvFrom:       true,
						PodTemplateAnnotations: utils.BuildSecretReloadAnnotation(secretName),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the Secret")
					err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload with pod template annotation", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload when auto=true annotation is on pod template only",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a ConfigMap")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"key": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with auto=true annotation on pod template ONLY")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						ConfigMapName:          configMapName,
						UseConfigMapEnvFrom:    true,
						PodTemplateAnnotations: utils.BuildAutoTrueAnnotation(),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the ConfigMap")
					err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s with auto=true on pod template should reload", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload when SecretProviderClass annotation is on pod template only",
				func(workloadType utils.WorkloadType) {
					if !utils.IsCSIDriverInstalled(ctx, csiClient) {
						Skip("CSI secrets store driver not installed")
					}
					if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
						Skip("Vault CSI provider not installed")
					}

					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a secret in Vault")
					err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
					Expect(err).NotTo(HaveOccurred())

					By("Creating a SecretProviderClass pointing to Vault secret")
					_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
						vaultSecretPath, "api_key")
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with SPC annotation on pod template ONLY")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						SPCName:                spcName,
						UseCSIVolume:           true,
						PodTemplateAnnotations: utils.BuildSecretProviderClassReloadAnnotation(spcName),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Finding the SPCPS created by CSI driver")
					spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace,
						workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Getting initial SPCPS version")
					initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the Vault secret")
					err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for CSI driver to sync the new secret version")
					err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName,
						initialVersion, 10*time.Second)
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload with SPC annotation on pod template", workloadType)
				},
				Entry("Deployment", Label("csi"), utils.WorkloadDeployment),
				Entry("DaemonSet", Label("csi"), utils.WorkloadDaemonSet),
				Entry("StatefulSet", Label("csi"), utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("csi", "argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("csi", "openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload when secretproviderclass auto annotation is on pod template only",
				func(workloadType utils.WorkloadType) {
					if !utils.IsCSIDriverInstalled(ctx, csiClient) {
						Skip("CSI secrets store driver not installed")
					}
					if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
						Skip("Vault CSI provider not installed")
					}

					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a secret in Vault")
					err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
					Expect(err).NotTo(HaveOccurred())

					By("Creating a SecretProviderClass pointing to Vault secret")
					_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
						vaultSecretPath, "api_key")
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with SPC auto annotation on pod template ONLY")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						SPCName:                spcName,
						UseCSIVolume:           true,
						PodTemplateAnnotations: utils.BuildSecretProviderClassAutoAnnotation(),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Finding the SPCPS created by CSI driver")
					spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace,
						workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Getting initial SPCPS version")
					initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the Vault secret")
					err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for CSI driver to sync the new secret version")
					err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName,
						initialVersion, 10*time.Second)
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload with SPC auto on pod template", workloadType)
				},
				Entry("Deployment", Label("csi"), utils.WorkloadDeployment),
				Entry("DaemonSet", Label("csi"), utils.WorkloadDaemonSet),
				Entry("StatefulSet", Label("csi"), utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("csi", "argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("csi", "openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should reload when annotations are on both workload and pod template",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a ConfigMap")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"key": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with annotations on BOTH workload metadata and pod template")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						ConfigMapName:          configMapName,
						UseConfigMapEnvFrom:    true,
						Annotations:            utils.BuildConfigMapReloadAnnotation(configMapName),
						PodTemplateAnnotations: utils.BuildConfigMapReloadAnnotation(configMapName),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the ConfigMap")
					err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for workload to be reloaded")
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue(), "%s should reload with annotations on both locations", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)

			DescribeTable("should NOT reload when pod template has ConfigMap annotation but Secret is updated",
				func(workloadType utils.WorkloadType) {
					adapter := registry.Get(workloadType)
					if adapter == nil {
						Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
					}

					By("Creating a ConfigMap and Secret")
					_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
						map[string]string{"key": "value"}, nil)
					Expect(err).NotTo(HaveOccurred())

					_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
						map[string]string{"password": "initial"}, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Creating workload with ConfigMap annotation on pod template but using Secret")
					err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
						SecretName:             secretName,
						UseSecretEnvFrom:       true,
						PodTemplateAnnotations: utils.BuildConfigMapReloadAnnotation(configMapName),
					})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

					By("Waiting for workload to be ready")
					err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
					Expect(err).NotTo(HaveOccurred())

					By("Updating the Secret (not the ConfigMap)")
					err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
					Expect(err).NotTo(HaveOccurred())

					By("Verifying workload was NOT reloaded (negative test)")
					time.Sleep(utils.NegativeTestWait)
					reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
						utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeFalse(), "%s should NOT reload when updating different resource than annotated", workloadType)
				},
				Entry("Deployment", utils.WorkloadDeployment),
				Entry("DaemonSet", utils.WorkloadDaemonSet),
				Entry("StatefulSet", utils.WorkloadStatefulSet),
				Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
				Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
			)
		})
	})

	// ============================================================
	// ENVVARS STRATEGY TESTS
	// ============================================================
	Context("EnvVars Strategy", Label("envvars"), Ordered, ContinueOnFailure, func() {
		// Redeploy Reloader with envvars strategy for this context
		BeforeAll(func() {
			By("Redeploying Reloader with envvars strategy")
			deployValues := map[string]string{
				"reloader.reloadStrategy": "env-vars",
			}
			// Preserve Argo support if available
			if utils.IsArgoRolloutsInstalled(ctx, testEnv.RolloutsClient) {
				deployValues["reloader.isArgoRollouts"] = "true"
			}
			// Enable CSI integration if CSI driver is installed
			if utils.IsCSIDriverInstalled(ctx, csiClient) {
				deployValues["reloader.enableCSIIntegration"] = "true"
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
			if utils.IsArgoRolloutsInstalled(ctx, testEnv.RolloutsClient) {
				deployValues["reloader.isArgoRollouts"] = "true"
			}
			// Preserve CSI integration if CSI driver is installed
			if utils.IsCSIDriverInstalled(ctx, csiClient) {
				deployValues["reloader.enableCSIIntegration"] = "true"
			}
			err := testEnv.DeployAndWait(deployValues)
			Expect(err).NotTo(HaveOccurred(), "Failed to restore Reloader to annotations strategy")
		})

		DescribeTable("should add STAKATER_ env var when ConfigMap changes", func(workloadType utils.WorkloadType) {
			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
			}

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
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for workload to have STAKATER_ env var")
			found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "%s should have STAKATER_ env var after ConfigMap change", workloadType)
		},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should add STAKATER_ env var when Secret changes", func(workloadType utils.WorkloadType) {
			adapter := registry.Get(workloadType)
			if adapter == nil {
				Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
			}

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
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret data")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for workload to have STAKATER_ env var")
			found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "%s should have STAKATER_ env var after Secret change", workloadType)
		},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		// CSI SecretProviderClassPodStatus env var tests with real Vault
		DescribeTable("should add STAKATER_ env var when SecretProviderClassPodStatus changes",
			func(workloadType utils.WorkloadType) {
				if !utils.IsCSIDriverInstalled(ctx, csiClient) {
					Skip("CSI secrets store driver not installed")
				}
				if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
					Skip("Vault CSI provider not installed")
				}

				adapter := registry.Get(workloadType)
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

				if !adapter.SupportsEnvVarStrategy() {
					Skip("Workload type does not support env var strategy")
				}

				By("Creating a secret in Vault")
				err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
				Expect(err).NotTo(HaveOccurred())

				By("Creating a SecretProviderClass pointing to Vault secret")
				_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
					vaultSecretPath, "api_key")
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with CSI volume and SPC reload annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SPCName:      spcName,
					UseCSIVolume: true,
					Annotations:  utils.BuildSecretProviderClassReloadAnnotation(spcName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Finding the SPCPS created by CSI driver")
				spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, workloadName,
					utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Getting initial SPCPS version")
				initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Vault secret")
				err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for CSI driver to sync the new secret version")
				err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion,
					10*time.Second)
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to have STAKATER_ env var")
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
					utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue(), "%s should have STAKATER_ env var after Vault secret change", workloadType)
			}, Entry("Deployment", Label("csi"), utils.WorkloadDeployment),
			Entry("DaemonSet", Label("csi"), utils.WorkloadDaemonSet),
			Entry("StatefulSet", Label("csi"), utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("csi", "argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("csi", "openshift"), utils.WorkloadDeploymentConfig),
		)

		// Negative tests for env var strategy
		DescribeTable("should NOT add STAKATER_ env var when only ConfigMap labels change",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

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
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the ConfigMap labels")
				err = utils.UpdateConfigMapLabels(ctx, kubeClient, testNamespace, configMapName, map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload does NOT have STAKATER_ env var")
				time.Sleep(utils.NegativeTestWait)
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
					utils.ShortTimeout)
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
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

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
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the Secret labels")
				err = utils.UpdateSecretLabels(ctx, kubeClient, testNamespace, secretName, map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload does NOT have STAKATER_ env var")
				time.Sleep(utils.NegativeTestWait)
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
					utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse(), "%s should NOT have STAKATER_ env var for label-only change", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
		)

		// CSI SPCPS label-only change negative test with real Vault
		DescribeTable("should NOT add STAKATER_ env var when only SecretProviderClassPodStatus labels change",
			func(workloadType utils.WorkloadType) {
				if !utils.IsCSIDriverInstalled(ctx, csiClient) {
					Skip("CSI secrets store driver not installed")
				}
				if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
					Skip("Vault CSI provider not installed")
				}

				adapter := registry.Get(workloadType)
				if adapter == nil {
					Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType))
				}

				if !adapter.SupportsEnvVarStrategy() {
					Skip("Workload type does not support env var strategy")
				}

				By("Creating a secret in Vault")
				err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Creating a SecretProviderClass pointing to Vault secret")
				_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
					vaultSecretPath, "api_key")
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with CSI volume and SPC reload annotation")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SPCName:      spcName,
					UseCSIVolume: true,
					Annotations:  utils.BuildSecretProviderClassReloadAnnotation(spcName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Finding the SPCPS created by CSI driver")
				spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, workloadName,
					utils.WorkloadReadyTimeout)
				Expect(err).NotTo(HaveOccurred())

				By("Updating only the SPCPS labels (should NOT trigger reload)")
				err = utils.UpdateSecretProviderClassPodStatusLabels(ctx, csiClient, testNamespace, spcpsName, map[string]string{"new-label": "new-value"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload does NOT have STAKATER_ env var")
				time.Sleep(utils.NegativeTestWait)
				found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
					utils.ShortTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse(), "%s should NOT have STAKATER_ env var for SPCPS label-only change",
					workloadType)
			}, Entry("Deployment", Label("csi"), utils.WorkloadDeployment),
			Entry("DaemonSet", Label("csi"), utils.WorkloadDaemonSet),
			Entry("StatefulSet", Label("csi"), utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("csi", "argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("csi", "openshift"), utils.WorkloadDeploymentConfig),
		)

		// CSI auto annotation with EnvVar strategy and real Vault
		It("should add STAKATER_ env var with secretproviderclass auto annotation", Label("csi"), func() {
			if !utils.IsCSIDriverInstalled(ctx, csiClient) {
				Skip("CSI secrets store driver not installed")
			}
			if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
				Skip("Vault CSI provider not installed")
			}

			adapter := registry.Get(utils.WorkloadDeployment)
			Expect(adapter).NotTo(BeNil())

			By("Creating a secret in Vault")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName, vaultSecretPath,
				"api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating Deployment with CSI volume and SPC auto annotation")
			err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
				SPCName:      spcName,
				UseCSIVolume: true,
				Annotations:  utils.BuildSecretProviderClassAutoAnnotation(),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, workloadName,
				utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial SPCPS version")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Vault secret")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync the new secret version")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion,
				10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to have STAKATER_ env var")
			found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
				utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "Deployment with SPC auto annotation should have STAKATER_ env var")
		})

		// CSI exclude annotation with EnvVar strategy and real Vault
		It("should NOT add STAKATER_ env var when excluded SecretProviderClassPodStatus changes", Label("csi"), func() {
			if !utils.IsCSIDriverInstalled(ctx, csiClient) {
				Skip("CSI secrets store driver not installed")
			}
			if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
				Skip("Vault CSI provider not installed")
			}

			adapter := registry.Get(utils.WorkloadDeployment)
			Expect(adapter).NotTo(BeNil())

			By("Creating a secret in Vault")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName, vaultSecretPath,
				"api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating Deployment with auto=true and SPC exclude annotation")
			err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
				SPCName:      spcName,
				UseCSIVolume: true,
				Annotations: utils.MergeAnnotations(utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretProviderClassExcludeAnnotation(spcName)),
			})
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, workloadName,
				utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial SPCPS version")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Vault secret (excluded SPC - should NOT trigger reload)")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync the new secret version")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion,
				10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment does NOT have STAKATER_ env var")
			time.Sleep(utils.NegativeTestWait)
			found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName, utils.StakaterEnvVarPrefix,
				utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse(), "Deployment should NOT have STAKATER_ env var for excluded SPCPS change")
		})

		// CSI init container with EnvVar strategy and real Vault
		It("should add STAKATER_ env var when SecretProviderClassPodStatus used by init container changes", Label("csi"), func() {
			if !utils.IsCSIDriverInstalled(ctx, csiClient) {
				Skip("CSI secrets store driver not installed")
			}
			if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
				Skip("Vault CSI provider not installed")
			}

			By("Creating a secret in Vault")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
				vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating Deployment with init container using CSI volume")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, workloadName,
				utils.WithInitContainerCSIVolume(spcName),
				utils.WithAnnotations(utils.BuildSecretProviderClassReloadAnnotation(spcName)))
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, workloadName) })

			adapter := utils.NewDeploymentAdapter(kubeClient)

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, workloadName,
				utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial SPCPS version")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Vault secret")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync the new secret version")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion,
				10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to have STAKATER_ env var")
			found, err := adapter.WaitEnvVar(ctx, testNamespace, workloadName,
				utils.StakaterEnvVarPrefix, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "Deployment with init container CSI should have STAKATER_ env var")
		})
	})
})
