package csi

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe(
	"CSI SecretProviderClass Tests", func() {
		var (
			deploymentName  string
			configMapName   string
			spcName         string
			vaultSecretPath string
		)

		BeforeEach(
			func() {
				deploymentName = utils.RandName("deploy")
				configMapName = utils.RandName("cm")
				spcName = utils.RandName("spc")
				// Each test gets its own Vault secret path to avoid conflicts
				vaultSecretPath = fmt.Sprintf("secret/%s", utils.RandName("test"))
			},
		)

		AfterEach(
			func() {
				_ = utils.DeleteDeployment(ctx, kubeClient, testNamespace, deploymentName)
				_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
				_ = utils.DeleteSecretProviderClass(ctx, csiClient, testNamespace, spcName)
				// Clean up Vault secret
				_ = utils.DeleteVaultSecret(ctx, kubeClient, restConfig, vaultSecretPath)
			},
		)

		Context(
			"Real Vault Integration Tests", func() {
				It(
					"should reload when Vault secret changes", func() {
						By("Creating a secret in Vault")
						err := utils.CreateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"api_key": "initial-value-v1"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating a SecretProviderClass pointing to Vault secret")
						_, err = utils.CreateSecretProviderClassWithSecret(
							ctx, csiClient, testNamespace, spcName,
							vaultSecretPath, "api_key",
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating Deployment with CSI volume and SPC reload annotation")
						_, err = utils.CreateDeployment(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.WithCSIVolume(spcName),
							utils.WithAnnotations(utils.BuildSecretProviderClassReloadAnnotation(spcName)),
						)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for Deployment to be ready")
						err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
						Expect(err).NotTo(HaveOccurred())

						By("Finding the SPCPS created by CSI driver")
						spcpsName, err := utils.FindSPCPSForDeployment(
							ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady,
						)
						Expect(err).NotTo(HaveOccurred())
						GinkgoWriter.Printf("Found SPCPS: %s\n", spcpsName)

						By("Getting initial SPCPS version")
						initialVersion, err := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
						Expect(err).NotTo(HaveOccurred())
						GinkgoWriter.Printf("Initial SPCPS version: %s\n", initialVersion)

						By("Updating the Vault secret")
						err = utils.UpdateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"api_key": "updated-value-v2"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for CSI driver to sync the new secret version")
						// CSI rotation poll interval is 10s, wait up to 30s for sync
						err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
						Expect(err).NotTo(HaveOccurred())
						GinkgoWriter.Println("CSI driver synced new secret version")

						By("Waiting for Deployment to be reloaded by Reloader")
						reloaded, err := utils.WaitForDeploymentReloaded(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.AnnotationLastReloadedFrom, utils.ReloadTimeout,
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(reloaded).To(BeTrue(), "Deployment should have been reloaded after Vault secret change")
					},
				)

				It(
					"should handle multiple Vault secret updates", func() {
						By("Creating a secret in Vault")
						err := utils.CreateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"password": "pass-v1"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating a SecretProviderClass pointing to Vault secret")
						_, err = utils.CreateSecretProviderClassWithSecret(
							ctx, csiClient, testNamespace, spcName,
							vaultSecretPath, "password",
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating Deployment with CSI volume")
						_, err = utils.CreateDeployment(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.WithCSIVolume(spcName),
							utils.WithAnnotations(utils.BuildSecretProviderClassReloadAnnotation(spcName)),
						)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for Deployment to be ready")
						err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
						Expect(err).NotTo(HaveOccurred())

						By("Finding the SPCPS")
						spcpsName, err := utils.FindSPCPSForDeployment(
							ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady,
						)
						Expect(err).NotTo(HaveOccurred())

						By("First update to Vault secret")
						initialVersion, _ := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)
						err = utils.UpdateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"password": "pass-v2"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for first CSI sync")
						err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for first reload")
						reloaded, err := utils.WaitForDeploymentReloaded(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.AnnotationLastReloadedFrom, utils.ReloadTimeout,
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(reloaded).To(BeTrue())

						By("Getting annotation value after first reload")
						deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
						Expect(err).NotTo(HaveOccurred())
						firstReloadValue := deploy.Spec.Template.Annotations[utils.AnnotationLastReloadedFrom]
						Expect(firstReloadValue).NotTo(BeEmpty())

						By("Waiting for Deployment to stabilize")
						err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
						Expect(err).NotTo(HaveOccurred())

						By("Finding the NEW SPCPS after first reload (new pod = new SPCPS)")
						newSpcpsName, err := utils.FindSPCPSForDeployment(
							ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady,
						)
						Expect(err).NotTo(HaveOccurred())
						GinkgoWriter.Printf("New SPCPS after first reload: %s\n", newSpcpsName)

						By("Second update to Vault secret")
						err = utils.UpdateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"password": "pass-v3"},
						)
						Expect(err).NotTo(HaveOccurred())

						// Note: We do not wait for SPCPS version change here because:
						// 1. CSI driver syncs the new secret version to SPCPS
						// 2. Reloader sees SPCPS change and immediately reloads deployment
						// 3. Deployment reload creates new pod -> new SPCPS (old one deleted)
						// So by the time we check, the original SPCPS no longer exists.
						// Instead, we directly verify the deployment annotation changed.

						By("Waiting for second reload with different annotation value")
						Eventually(
							func() string {
								deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
								if err != nil {
									return ""
								}
								return deploy.Spec.Template.Annotations[utils.AnnotationLastReloadedFrom]
							}, utils.ReloadTimeout,
						).ShouldNot(Equal(firstReloadValue), "Annotation should change after second Vault secret update")
					},
				)
			},
		)

		Context(
			"Typed Auto Annotation Tests", func() {
				It(
					"should reload only SPC changes with secretproviderclass auto annotation, not ConfigMap", func() {
						By("Creating a ConfigMap")
						_, err := utils.CreateConfigMap(
							ctx, kubeClient, testNamespace, configMapName,
							map[string]string{"key": "initial"}, nil,
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating a secret in Vault")
						err = utils.CreateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"token": "token-v1"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating a SecretProviderClass pointing to Vault secret")
						_, err = utils.CreateSecretProviderClassWithSecret(
							ctx, csiClient, testNamespace, spcName,
							vaultSecretPath, "token",
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating Deployment with ConfigMap envFrom AND CSI volume, but only SPC auto annotation")
						_, err = utils.CreateDeployment(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.WithConfigMapEnvFrom(configMapName),
							utils.WithCSIVolume(spcName),
							utils.WithAnnotations(utils.BuildSecretProviderClassAutoAnnotation()),
						)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for Deployment to be ready")
						err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
						Expect(err).NotTo(HaveOccurred())

						By("Updating the ConfigMap (should NOT trigger reload)")
						err = utils.UpdateConfigMap(
							ctx, kubeClient, testNamespace, configMapName,
							map[string]string{"key": "updated"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Verifying Deployment was NOT reloaded for ConfigMap change")
						time.Sleep(utils.NegativeTestWait)
						reloaded, err := utils.WaitForDeploymentReloaded(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.AnnotationLastReloadedFrom, utils.ShortTimeout,
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(reloaded).To(BeFalse(), "SPC auto annotation should not trigger reload for ConfigMap changes")

						By("Finding the SPCPS")
						spcpsName, err := utils.FindSPCPSForDeployment(
							ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady,
						)
						Expect(err).NotTo(HaveOccurred())

						By("Getting SPCPS version before Vault update")
						initialVersion, _ := utils.GetSPCPSVersion(ctx, csiClient, testNamespace, spcpsName)

						By("Updating the Vault secret (should trigger reload)")
						err = utils.UpdateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"token": "token-v2"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for CSI driver to sync")
						err = utils.WaitForSPCPSVersionChange(ctx, csiClient, testNamespace, spcpsName, initialVersion, 10*time.Second)
						Expect(err).NotTo(HaveOccurred())

						By("Verifying Deployment WAS reloaded for Vault secret change")
						reloaded, err = utils.WaitForDeploymentReloaded(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.AnnotationLastReloadedFrom, utils.ReloadTimeout,
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(reloaded).To(BeTrue(), "SPC auto annotation should trigger reload for Vault secret changes")
					},
				)

				It(
					"should reload for both ConfigMap and SPC when using combined auto=true", func() {
						By("Creating a ConfigMap")
						_, err := utils.CreateConfigMap(
							ctx, kubeClient, testNamespace, configMapName,
							map[string]string{"key": "initial"}, nil,
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating a secret in Vault")
						err = utils.CreateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"secret": "secret-v1"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating a SecretProviderClass pointing to Vault secret")
						_, err = utils.CreateSecretProviderClassWithSecret(
							ctx, csiClient, testNamespace, spcName,
							vaultSecretPath, "secret",
						)
						Expect(err).NotTo(HaveOccurred())

						By("Creating Deployment with ConfigMap envFrom AND CSI volume with combined auto=true")
						_, err = utils.CreateDeployment(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.WithConfigMapEnvFrom(configMapName),
							utils.WithCSIVolume(spcName),
							utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
						)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for Deployment to be ready")
						err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
						Expect(err).NotTo(HaveOccurred())

						By("Updating the ConfigMap (should trigger reload with auto=true)")
						err = utils.UpdateConfigMap(
							ctx, kubeClient, testNamespace, configMapName,
							map[string]string{"key": "updated"},
						)
						Expect(err).NotTo(HaveOccurred())

						By("Verifying Deployment WAS reloaded for ConfigMap change")
						reloaded, err := utils.WaitForDeploymentReloaded(
							ctx, kubeClient, testNamespace, deploymentName,
							utils.AnnotationLastReloadedFrom, utils.ReloadTimeout,
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(reloaded).To(BeTrue(), "Combined auto=true should trigger reload for ConfigMap changes")

						By("Waiting for Deployment to stabilize")
						err = utils.WaitForDeploymentReady(ctx, kubeClient, testNamespace, deploymentName, utils.DeploymentReady)
						Expect(err).NotTo(HaveOccurred())

						By("Getting current annotation value")
						deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
						Expect(err).NotTo(HaveOccurred())
						firstReloadValue := deploy.Spec.Template.Annotations[utils.AnnotationLastReloadedFrom]

						By("Finding the NEW SPCPS after ConfigMap reload (new pod = new SPCPS)")
						newSpcpsName, err := utils.FindSPCPSForDeployment(
							ctx, csiClient, kubeClient, testNamespace, deploymentName, utils.DeploymentReady,
						)
						Expect(err).NotTo(HaveOccurred())
						GinkgoWriter.Printf("New SPCPS after ConfigMap reload: %s\n", newSpcpsName)

						By("Updating the Vault secret (should also trigger reload with auto=true)")
						err = utils.UpdateVaultSecret(
							ctx, kubeClient, restConfig, vaultSecretPath,
							map[string]string{"secret": "secret-v2"},
						)
						Expect(err).NotTo(HaveOccurred())

						// Note: We don't wait for SPCPS version change here because:
						// 1. CSI driver syncs the new secret version to SPCPS
						// 2. Reloader sees SPCPS change and immediately reloads deployment
						// 3. Deployment reload creates new pod → new SPCPS (old one deleted)
						// So by the time we check, the original SPCPS no longer exists.
						// Instead, we directly verify the deployment annotation changed.

						By("Verifying Deployment WAS reloaded for Vault secret change")
						Eventually(
							func() string {
								deploy, err := utils.GetDeployment(ctx, kubeClient, testNamespace, deploymentName)
								if err != nil {
									return ""
								}
								return deploy.Spec.Template.Annotations[utils.AnnotationLastReloadedFrom]
							}, utils.ReloadTimeout,
						).ShouldNot(
							Equal(firstReloadValue),
							"Combined auto=true should trigger reload for Vault secret changes",
						)
					},
				)
			},
		)
	},
)
