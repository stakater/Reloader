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
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (excluded ConfigMap)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
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
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2, map[string]string{"key2": "updated2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
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
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (excluded Secret)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
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
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName2, map[string]string{"password2": "updated2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
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
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying workload was NOT reloaded (excluded ConfigMap)")
				time.Sleep(utils.NegativeTestWait)
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
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

	Context("SecretProviderClass exclude annotation", Label("csi"), func() {
		var (
			spcName          string
			spcName2         string
			vaultSecretPath  string
			vaultSecretPath2 string
		)

		BeforeEach(func() {
			if !utils.IsCSIDriverInstalled(ctx, csiClient) {
				Skip("CSI secrets store driver not installed")
			}
			if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
				Skip("Vault CSI provider not installed")
			}
			spcName = utils.RandName("spc")
			spcName2 = utils.RandName("spc2")
			vaultSecretPath = fmt.Sprintf("secret/%s", utils.RandName("test"))
			vaultSecretPath2 = fmt.Sprintf("secret/%s", utils.RandName("test2"))
		})

		AfterEach(func() {
			_ = utils.DeleteSecretProviderClass(ctx, csiClient, testNamespace, spcName)
			_ = utils.DeleteSecretProviderClass(ctx, csiClient, testNamespace, spcName2)
			_ = utils.DeleteVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath)
			_ = utils.DeleteVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath2)
		})

		It("should NOT reload when excluded SecretProviderClassPodStatus changes", func() {
			By("Creating Vault secret for the excluded SPC")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{
				"api_key": "initial-excluded-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Creating SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName, vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and secretproviderclasses.exclude annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithCSIVolume(spcName),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretProviderClassExcludeAnnotation(spcName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial SPCPS version")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Vault secret for excluded SPC")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{
				"api_key": "updated-excluded-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync (SPCPS version change)")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded (excluded SPC)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment should NOT reload when excluded SecretProviderClassPodStatus changes")
		})

		It("should reload when non-excluded SecretProviderClassPodStatus changes", func() {
			By("Creating two Vault secrets")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{
				"api_key": "initial-excluded-value",
			})
			Expect(err).NotTo(HaveOccurred())

			err = utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath2, map[string]string{
				"api_key": "initial-nonexcluded-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Creating two SecretProviderClasses")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName, vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName2, vaultSecretPath2, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and secretproviderclasses.exclude for first SPC only")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithCSIVolume(spcName),
				utils.WithCSIVolume(spcName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildSecretProviderClassExcludeAnnotation(spcName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS for non-excluded SPC")

			spcpsName2, err := utils.FindSPCPSForSPC(ctx, csiClient, testNamespace, spcName2, 30*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial SPCPS version for non-excluded SPC")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName2)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Vault secret for non-excluded SPC")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath2, map[string]string{
				"api_key": "updated-nonexcluded-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync (SPCPS version change)")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName2, initialVersion, 10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should reload when non-excluded SecretProviderClassPodStatus changes")
		})
	})
})
