package annotations

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Auto Reload Annotation Tests", func() {
	var (
		deploymentName  string
		configMapName   string
		secretName      string
		spcName         string
		vaultSecretPath string
		adapter         *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		secretName = utils.RandName("secret")
		spcName = utils.RandName("spc")
		vaultSecretPath = fmt.Sprintf("secret/%s", utils.RandName("test"))
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteSecret(ctx, kubeClient, testNamespace, secretName)
		if csiClient != nil {
			_ = utils.DeleteSecretProviderClass(ctx, csiClient, testNamespace, spcName)
		}
		_ = utils.DeleteVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath)
	})

	Context("with reloader.stakater.com/auto=true annotation", func() {
		It("should reload Deployment when any referenced ConfigMap changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with auto=true should have been reloaded")
		})

		It("should reload Deployment when any referenced Secret changes", func() {
			By("Creating a Secret")
			_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"password": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret data")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"password": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with auto=true should have been reloaded for Secret change")
		})

		It("should reload Deployment when either ConfigMap or Secret changes", func() {
			By("Creating a ConfigMap and Secret")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true annotation referencing both")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"config": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with auto=true should have been reloaded for ConfigMap change")
		})
	})

	Context("with reloader.stakater.com/auto=false annotation", func() {
		It("should NOT reload Deployment when ConfigMap changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=false annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoFalseAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap data")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment is NOT reloaded (negative test)")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment with auto=false should NOT have been reloaded")
		})
	})

	Context("with configmap.reloader.stakater.com/auto=true annotation", func() {
		It("should reload Deployment only when ConfigMap changes, not Secret", func() {
			By("Creating a ConfigMap and Secret")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with configmap auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildConfigMapAutoAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"config": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for ConfigMap change")
		})
	})

	Context("with secret.reloader.stakater.com/auto=true annotation", func() {
		It("should reload Deployment only when Secret changes, not ConfigMap", func() {
			By("Creating a ConfigMap and Secret")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"config": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
				map[string]string{"secret": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with secret auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithSecretEnvFrom(secretName),
				utils.WithAnnotations(utils.BuildSecretAutoAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Secret")
			err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName, map[string]string{"secret": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for Secret change")
		})
	})

	Context("with secretproviderclass.reloader.stakater.com/auto=true annotation", Label("csi"), func() {
		BeforeEach(func() {
			if !utils.IsCSIDriverInstalled(ctx, csiClient) {
				Skip("CSI secrets store driver not installed")
			}
			if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
				Skip("Vault CSI provider not installed")
			}
		})

		It("should reload Deployment when SecretProviderClassPodStatus changes", func() {
			By("Creating a secret in Vault")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
				vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with secretproviderclass auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithCSIVolume(spcName),
				utils.WithAnnotations(utils.BuildSecretProviderClassAutoAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
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
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("CSI driver synced new secret version")

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for Vault secret change")
		})

		It("should NOT reload Deployment when ConfigMap changes (only SPC auto enabled)", func() {
			By("Creating a secret in Vault")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
				vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a ConfigMap")
			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with CSI volume AND ConfigMap, but only SPC auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithCSIVolume(spcName),
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildSecretProviderClassAutoAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap (should NOT trigger reload with SPC auto only)")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT reloaded for ConfigMap change")
			time.Sleep(utils.NegativeTestWait)
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeFalse(), "Deployment with SPC auto only should NOT have been reloaded for ConfigMap change")

			By("Getting initial SPCPS version")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Vault secret (should trigger reload)")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync the new secret version")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded for SPC change")
			reloaded, err = adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for Vault secret change")
		})

		It("should reload when using combined auto=true annotation for SPC", func() {
			By("Creating a secret in Vault")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "initial-value-v1"})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault secret")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName,
				vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with CSI volume and general auto=true annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithCSIVolume(spcName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Finding the SPCPS created by CSI driver")
			spcpsName, err := utils.FindSPCPSForDeployment(ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial SPCPS version")
			initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the Vault secret")
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{"api_key": "updated-value-v2"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync the new secret version")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with auto=true should have been reloaded for Vault secret change")
		})
	})

	Context("with auto annotation and explicit reload annotation together", func() {
		It("should reload when auto-detected resource changes", func() {
			configMapName2 := utils.RandName("cm2")
			defer func() { _ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2) }()

			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key1": "value1"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "value2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with auto=true and explicit reload for first ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithConfigMapEnvFrom(configMapName2),
				utils.WithAnnotations(utils.MergeAnnotations(
					utils.BuildAutoTrueAnnotation(),
					utils.BuildConfigMapReloadAnnotation(configMapName),
				)),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the second ConfigMap (auto-detected)")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName2, map[string]string{"key2": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded for auto-detected ConfigMap change")
		})
	})
})
