package advanced

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Multi-Container Tests", func() {
	var (
		deploymentName string
		configMapName  string
		configMapName2 string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		configMapName2 = utils.RandName("cm2")
		adapter = utils.NewDeploymentAdapter(kubeClient)
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName2)
	})

	Context("Multiple containers same ConfigMap", func() {
		It("should reload when ConfigMap used by multiple containers changes", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"shared-key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with multiple containers using the same ConfigMap")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithMultipleContainers(2),
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"shared-key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with multiple containers should be reloaded")
		})
	})

	Context("Multiple containers different ConfigMaps", func() {
		It("should reload when any container's ConfigMap changes", func() {
			By("Creating two ConfigMaps")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key1": "initial1"}, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName2,
				map[string]string{"key2": "initial2"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with multiple containers using different ConfigMaps")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithMultipleContainersAndEnv(configMapName, configMapName2),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be ready")
			err = adapter.WaitReady(ctx, testNamespace, deploymentName, utils.DeploymentReady)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the first ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key1": "updated1"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment should be reloaded when first container's ConfigMap changes")
		})
	})

	Context("Init container with CSI volume", Label("csi"), func() {
		var (
			spcName         string
			vaultSecretPath string
		)

		BeforeEach(func() {
			if !utils.IsCSIDriverInstalled(ctx, csiClient) {
				Skip("CSI secrets store driver not installed")
			}
			if !utils.IsVaultProviderInstalled(ctx, kubeClient) {
				Skip("Vault CSI provider not installed")
			}
			spcName = utils.RandName("spc")
			vaultSecretPath = fmt.Sprintf("secret/%s", utils.RandName("test"))
		})

		AfterEach(func() {
			if spcName != "" {
				_ = utils.DeleteSecretProviderClass(ctx, csiClient, testNamespace, spcName)
			}
			if vaultSecretPath != "" {
				_ = utils.DeleteVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath)
			}
		})

		It("should reload when SecretProviderClassPodStatus used by init container changes", func() {
			By("Creating a Vault secret")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{
				"api_key": "initial-init-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName, vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with init container using CSI volume")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithInitContainerCSIVolume(spcName),
				utils.WithAnnotations(utils.BuildSecretProviderClassReloadAnnotation(spcName)),
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
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{
				"api_key": "updated-init-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync (SPCPS version change)")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with init container using CSI volume should be reloaded")
		})

		It("should reload with auto annotation when init container CSI volume changes", func() {
			By("Creating a Vault secret")
			err := utils.CreateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{
				"api_key": "initial-init-auto-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a SecretProviderClass pointing to Vault")
			_, err = utils.CreateSecretProviderClassWithSecret(ctx, csiClient, testNamespace, spcName, vaultSecretPath, "api_key")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Deployment with init container using CSI volume and auto annotation")
			_, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
				utils.WithInitContainerCSIVolume(spcName),
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
			err = utils.UpdateVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath, map[string]string{
				"api_key": "updated-init-auto-value",
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for CSI driver to sync (SPCPS version change)")
			err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Deployment to be reloaded")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, deploymentName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment with init container CSI volume and auto=true should be reloaded")
		})
	})
})
