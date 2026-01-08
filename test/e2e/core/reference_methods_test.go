package core

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Reference Method Tests", func() {
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
	// valueFrom.configMapKeyRef TESTS
	// ============================================================
	Context("valueFrom.configMapKeyRef", func() {
		DescribeTable("should reload when ConfigMap referenced via valueFrom.configMapKeyRef changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config_key": "initial_value"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with valueFrom.configMapKeyRef")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:      configMapName,
					UseConfigMapKeyRef: true,
					ConfigMapKey:       "config_key",
					EnvVarName:         "MY_CONFIG_VAR",
					Annotations: utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config_key": "updated_value"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with valueFrom.configMapKeyRef should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})

	// ============================================================
	// valueFrom.secretKeyRef TESTS
	// ============================================================
	Context("valueFrom.secretKeyRef", func() {
		DescribeTable("should reload when Secret referenced via valueFrom.secretKeyRef changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"secret_key": "initial_secret"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with valueFrom.secretKeyRef")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:    secretName,
					UseSecretKeyRef: true,
					SecretKey:     "secret_key",
					EnvVarName:    "MY_SECRET_VAR",
					Annotations: utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"secret_key": "updated_secret"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with valueFrom.secretKeyRef should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})

	// ============================================================
	// PROJECTED VOLUME TESTS
	// ============================================================
	Context("Projected Volumes", func() {
		DescribeTable("should reload when ConfigMap in projected volume changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with projected ConfigMap volume")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:      configMapName,
					UseProjectedVolume: true,
					Annotations: utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with projected ConfigMap volume should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should reload when Secret in projected volume changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with projected Secret volume")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:         secretName,
					UseProjectedVolume: true,
					Annotations: utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with projected Secret volume should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should reload when ConfigMap changes in mixed projected volume",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap and Secret")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with projected volume containing both")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:      configMapName,
					SecretName:         secretName,
					UseProjectedVolume: true,
					Annotations: utils.MergeAnnotations(
						utils.BuildConfigMapReloadAnnotation(configMapName),
						utils.BuildSecretReloadAnnotation(secretName),
					),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s should reload when ConfigMap in mixed projected volume changes", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should reload when Secret changes in mixed projected volume",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap and Secret")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with projected volume containing both")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:      configMapName,
					SecretName:         secretName,
					UseProjectedVolume: true,
					Annotations: utils.MergeAnnotations(
						utils.BuildConfigMapReloadAnnotation(configMapName),
						utils.BuildSecretReloadAnnotation(secretName),
					),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s should reload when Secret in mixed projected volume changes", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})

	// ============================================================
	// INIT CONTAINER TESTS
	// ============================================================
	Context("Init Container with envFrom", func() {
		DescribeTable("should reload when ConfigMap referenced by init container changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"INIT_VAR": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with init container referencing ConfigMap")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:    configMapName,
					UseInitContainer: true,
					Annotations: utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"INIT_VAR": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with init container ConfigMap should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should reload when Secret referenced by init container changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"INIT_SECRET": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with init container referencing Secret")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:       secretName,
					UseInitContainer: true,
					Annotations: utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"INIT_SECRET": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with init container Secret should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})

	Context("Init Container with Volume Mount", func() {
		DescribeTable("should reload when ConfigMap volume mounted in init container changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with init container using ConfigMap volume mount")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:          configMapName,
					UseInitContainerVolume: true,
					Annotations: utils.BuildConfigMapReloadAnnotation(configMapName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"config.yaml": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with init container ConfigMap volume should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)

		DescribeTable("should reload when Secret volume mounted in init container changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a Secret")
				_, err := utils.CreateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with init container using Secret volume mount")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					SecretName:             secretName,
					UseInitContainerVolume: true,
					Annotations: utils.BuildSecretReloadAnnotation(secretName),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the Secret")
				err = utils.UpdateSecretFromStrings(ctx, kubeClient, testNamespace, secretName,
					map[string]string{"credentials": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with init container Secret volume should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})

	// ============================================================
	// AUTO ANNOTATION WITH VALUEFROM TESTS
	// ============================================================
	Context("Auto Annotation with valueFrom", func() {
		DescribeTable("should reload with auto=true when ConfigMap referenced via valueFrom changes",
			func(workloadType utils.WorkloadType) {
				adapter := registry.Get(workloadType)
				if adapter == nil { Skip(fmt.Sprintf("%s adapter not available (CRD not installed)", workloadType)) }

				By("Creating a ConfigMap")
				_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"auto_config_key": "initial"}, nil)
				Expect(err).NotTo(HaveOccurred())

				By("Creating workload with auto=true and valueFrom")
				err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
					ConfigMapName:      configMapName,
					UseConfigMapKeyRef: true,
					ConfigMapKey:       "auto_config_key",
					EnvVarName:         "AUTO_CONFIG_VAR",
					Annotations: utils.BuildAutoTrueAnnotation(),
				})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = adapter.Delete(ctx, testNamespace, workloadName) })

				By("Waiting for workload to be ready")
				err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
				Expect(err).NotTo(HaveOccurred())

				By("Updating the ConfigMap")
				err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
					map[string]string{"auto_config_key": "updated"})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for workload to be reloaded")
				reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
					utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(reloaded).To(BeTrue(), "%s with auto=true and valueFrom should reload", workloadType)
			},
			Entry("Deployment", utils.WorkloadDeployment),
			Entry("DaemonSet", utils.WorkloadDaemonSet),
			Entry("StatefulSet", utils.WorkloadStatefulSet),
			Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
			Entry("DeploymentConfig", Label("openshift"), utils.WorkloadDeploymentConfig),
		)
	})
})
