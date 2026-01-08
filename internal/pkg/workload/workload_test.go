package workload

import (
	"testing"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/testutil"
)

// testRolloutStrategyAnnotation is the annotation key used in tests for rollout strategy.
const testRolloutStrategyAnnotation = "reloader.stakater.com/rollout-strategy"

// addEnvVar adds an environment variable with a ConfigMapKeyRef or SecretKeyRef to a container.
func addEnvVarConfigMapRef(containers []corev1.Container, envName, configMapName, key string) {
	if len(containers) > 0 {
		containers[0].Env = append(containers[0].Env, corev1.EnvVar{
			Name: envName,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					Key:                  key,
				},
			},
		})
	}
}

func addEnvVarSecretRef(containers []corev1.Container, envName, secretName, key string) {
	if len(containers) > 0 {
		containers[0].Env = append(containers[0].Env, corev1.EnvVar{
			Name: envName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  key,
				},
			},
		})
	}
}

func TestDeploymentWorkload_BasicGetters(t *testing.T) {
	deploy := testutil.NewDeployment("test-deploy", "test-ns", map[string]string{"key": "value"})

	w := NewDeploymentWorkload(deploy)

	if w.Kind() != KindDeployment {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindDeployment)
	}
	if w.GetName() != "test-deploy" {
		t.Errorf("GetName() = %v, want test-deploy", w.GetName())
	}
	if w.GetNamespace() != "test-ns" {
		t.Errorf("GetNamespace() = %v, want test-ns", w.GetNamespace())
	}
	if w.GetAnnotations()["key"] != "value" {
		t.Errorf("GetAnnotations()[key] = %v, want value", w.GetAnnotations()["key"])
	}
	if w.GetObject() != deploy {
		t.Error("GetObject() should return the underlying deployment")
	}
}

func TestDeploymentWorkload_PodTemplateAnnotations(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Annotations["existing"] = "annotation"

	w := NewDeploymentWorkload(deploy)

	// Test get
	annotations := w.GetPodTemplateAnnotations()
	if annotations["existing"] != "annotation" {
		t.Errorf("GetPodTemplateAnnotations()[existing] = %v, want annotation", annotations["existing"])
	}

	// Test set
	w.SetPodTemplateAnnotation("new-key", "new-value")
	if w.GetPodTemplateAnnotations()["new-key"] != "new-value" {
		t.Error("SetPodTemplateAnnotation should add new annotation")
	}
}

func TestDeploymentWorkload_PodTemplateAnnotations_NilInit(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Annotations = nil

	w := NewDeploymentWorkload(deploy)

	// Should initialize nil map
	annotations := w.GetPodTemplateAnnotations()
	if annotations == nil {
		t.Error("GetPodTemplateAnnotations should initialize nil map")
	}

	// Should work with nil initial map
	w.SetPodTemplateAnnotation("key", "value")
	if w.GetPodTemplateAnnotations()["key"] != "value" {
		t.Error("SetPodTemplateAnnotation should work with nil initial map")
	}
}

func TestDeploymentWorkload_Containers(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "busybox"}}

	w := NewDeploymentWorkload(deploy)

	// Test get containers
	containers := w.GetContainers()
	if len(containers) != 1 || containers[0].Name != "main" {
		t.Errorf("GetContainers() = %v, want [main]", containers)
	}

	// Test get init containers
	initContainers := w.GetInitContainers()
	if len(initContainers) != 1 || initContainers[0].Name != "init" {
		t.Errorf("GetInitContainers() = %v, want [init]", initContainers)
	}

	// Test set containers
	newContainers := []corev1.Container{{Name: "new-main", Image: "alpine"}}
	w.SetContainers(newContainers)
	if w.GetContainers()[0].Name != "new-main" {
		t.Error("SetContainers should update containers")
	}

	// Test set init containers
	newInitContainers := []corev1.Container{{Name: "new-init", Image: "alpine"}}
	w.SetInitContainers(newInitContainers)
	if w.GetInitContainers()[0].Name != "new-init" {
		t.Error("SetInitContainers should update init containers")
	}
}

func TestDeploymentWorkload_Volumes(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "config-vol"},
		{Name: "secret-vol"},
	}

	w := NewDeploymentWorkload(deploy)

	volumes := w.GetVolumes()
	if len(volumes) != 2 {
		t.Errorf("GetVolumes() length = %d, want 2", len(volumes))
	}
}

func TestDeploymentWorkload_UsesConfigMap_Volume(t *testing.T) {
	deploy := testutil.NewDeploymentWithVolume("test", "default", "my-config", "")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("my-config") {
		t.Error("UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestDeploymentWorkload_UsesConfigMap_ProjectedVolume(t *testing.T) {
	deploy := testutil.NewDeploymentWithProjectedVolume("test", "default", "projected-config", "")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("projected-config") {
		t.Error("UsesConfigMap should return true for projected ConfigMap volume")
	}
}

func TestDeploymentWorkload_UsesConfigMap_EnvFrom(t *testing.T) {
	deploy := testutil.NewDeploymentWithEnvFrom("test", "default", "env-config", "")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("env-config") {
		t.Error("UsesConfigMap should return true for envFrom ConfigMap")
	}
}

func TestDeploymentWorkload_UsesConfigMap_EnvVar(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	addEnvVarConfigMapRef(deploy.Spec.Template.Spec.Containers, "CONFIG_VALUE", "var-config", "some-key")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("var-config") {
		t.Error("UsesConfigMap should return true for env var ConfigMapKeyRef")
	}
}

func TestDeploymentWorkload_UsesConfigMap_InitContainer(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name: "init",
			EnvFrom: []corev1.EnvFromSource{
				{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "init-config"},
					},
				},
			},
		},
	}

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("init-config") {
		t.Error("UsesConfigMap should return true for init container ConfigMap")
	}
}

func TestDeploymentWorkload_UsesSecret_Volume(t *testing.T) {
	deploy := testutil.NewDeploymentWithVolume("test", "default", "", "my-secret")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("my-secret") {
		t.Error("UsesSecret should return true for Secret volume")
	}
	if w.UsesSecret("other-secret") {
		t.Error("UsesSecret should return false for non-existent Secret")
	}
}

func TestDeploymentWorkload_UsesSecret_ProjectedVolume(t *testing.T) {
	deploy := testutil.NewDeploymentWithProjectedVolume("test", "default", "", "projected-secret")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("projected-secret") {
		t.Error("UsesSecret should return true for projected Secret volume")
	}
}

func TestDeploymentWorkload_UsesSecret_EnvFrom(t *testing.T) {
	deploy := testutil.NewDeploymentWithEnvFrom("test", "default", "", "env-secret")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("env-secret") {
		t.Error("UsesSecret should return true for envFrom Secret")
	}
}

func TestDeploymentWorkload_UsesSecret_EnvVar(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	addEnvVarSecretRef(deploy.Spec.Template.Spec.Containers, "SECRET_VALUE", "var-secret", "some-key")

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("var-secret") {
		t.Error("UsesSecret should return true for env var SecretKeyRef")
	}
}

func TestDeploymentWorkload_UsesSecret_InitContainer(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name: "init",
			EnvFrom: []corev1.EnvFromSource{
				{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "init-secret"},
					},
				},
			},
		},
	}

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("init-secret") {
		t.Error("UsesSecret should return true for init container Secret")
	}
}

func TestDeploymentWorkload_GetEnvFromSources(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:    "main",
			EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
		},
		{
			Name:    "sidecar",
			EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "secret1"}}}},
		},
	}
	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:    "init",
			EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "init-cm"}}}},
		},
	}

	w := NewDeploymentWorkload(deploy)

	sources := w.GetEnvFromSources()
	if len(sources) != 3 {
		t.Errorf("GetEnvFromSources() returned %d sources, want 3", len(sources))
	}
}

func TestDeploymentWorkload_DeepCopy(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)

	w := NewDeploymentWorkload(deploy)
	copy := w.DeepCopy()

	// Modify original
	w.SetPodTemplateAnnotation("modified", "true")

	// Copy should not be affected
	copyAnnotations := copy.GetPodTemplateAnnotations()
	if copyAnnotations["modified"] == "true" {
		t.Error("DeepCopy should create independent copy")
	}
}

func TestDeploymentWorkload_GetOwnerReferences(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.OwnerReferences = []metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "test-rs"},
	}

	w := NewDeploymentWorkload(deploy)

	refs := w.GetOwnerReferences()
	if len(refs) != 1 || refs[0].Name != "test-rs" {
		t.Errorf("GetOwnerReferences() = %v, want owner ref to test-rs", refs)
	}
}

// DaemonSet tests
func TestDaemonSetWorkload_BasicGetters(t *testing.T) {
	ds := testutil.NewDaemonSet("test-ds", "test-ns", map[string]string{"key": "value"})

	w := NewDaemonSetWorkload(ds)

	if w.Kind() != KindDaemonSet {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindDaemonSet)
	}
	if w.GetName() != "test-ds" {
		t.Errorf("GetName() = %v, want test-ds", w.GetName())
	}
	if w.GetNamespace() != "test-ns" {
		t.Errorf("GetNamespace() = %v, want test-ns", w.GetNamespace())
	}
	if w.GetAnnotations()["key"] != "value" {
		t.Errorf("GetAnnotations()[key] = %v, want value", w.GetAnnotations()["key"])
	}
	if w.GetObject() != ds {
		t.Error("GetObject() should return the underlying daemonset")
	}
}

func TestDaemonSetWorkload_PodTemplateAnnotations(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Annotations["existing"] = "annotation"

	w := NewDaemonSetWorkload(ds)

	annotations := w.GetPodTemplateAnnotations()
	if annotations["existing"] != "annotation" {
		t.Errorf("GetPodTemplateAnnotations()[existing] = %v, want annotation", annotations["existing"])
	}

	w.SetPodTemplateAnnotation("new-key", "new-value")
	if w.GetPodTemplateAnnotations()["new-key"] != "new-value" {
		t.Error("SetPodTemplateAnnotation should add new annotation")
	}
}

func TestDaemonSetWorkload_PodTemplateAnnotations_NilInit(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Annotations = nil

	w := NewDaemonSetWorkload(ds)

	annotations := w.GetPodTemplateAnnotations()
	if annotations == nil {
		t.Error("GetPodTemplateAnnotations should initialize nil map")
	}

	w.SetPodTemplateAnnotation("key", "value")
	if w.GetPodTemplateAnnotations()["key"] != "value" {
		t.Error("SetPodTemplateAnnotation should work with nil initial map")
	}
}

func TestDaemonSetWorkload_Containers(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "busybox"}}

	w := NewDaemonSetWorkload(ds)

	containers := w.GetContainers()
	if len(containers) != 1 || containers[0].Name != "main" {
		t.Errorf("GetContainers() = %v, want [main]", containers)
	}

	initContainers := w.GetInitContainers()
	if len(initContainers) != 1 || initContainers[0].Name != "init" {
		t.Errorf("GetInitContainers() = %v, want [init]", initContainers)
	}

	newContainers := []corev1.Container{{Name: "new-main", Image: "alpine"}}
	w.SetContainers(newContainers)
	if w.GetContainers()[0].Name != "new-main" {
		t.Error("SetContainers should update containers")
	}

	newInitContainers := []corev1.Container{{Name: "new-init", Image: "alpine"}}
	w.SetInitContainers(newInitContainers)
	if w.GetInitContainers()[0].Name != "new-init" {
		t.Error("SetInitContainers should update init containers")
	}
}

func TestDaemonSetWorkload_Volumes(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "config-vol"},
		{Name: "secret-vol"},
	}

	w := NewDaemonSetWorkload(ds)

	volumes := w.GetVolumes()
	if len(volumes) != 2 {
		t.Errorf("GetVolumes() length = %d, want 2", len(volumes))
	}
}

func TestDaemonSetWorkload_UsesConfigMap(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "ds-config"},
				},
			},
		},
	}

	w := NewDaemonSetWorkload(ds)

	if !w.UsesConfigMap("ds-config") {
		t.Error("DaemonSet UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestDaemonSetWorkload_UsesConfigMap_EnvFrom(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "ds-env-config"}}},
	}

	w := NewDaemonSetWorkload(ds)

	if !w.UsesConfigMap("ds-env-config") {
		t.Error("DaemonSet UsesConfigMap should return true for envFrom ConfigMap")
	}
}

func TestDaemonSetWorkload_UsesSecret(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "secret-vol",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "ds-secret"},
			},
		},
	}

	w := NewDaemonSetWorkload(ds)

	if !w.UsesSecret("ds-secret") {
		t.Error("DaemonSet UsesSecret should return true for Secret volume")
	}
	if w.UsesSecret("other-secret") {
		t.Error("UsesSecret should return false for non-existent Secret")
	}
}

func TestDaemonSetWorkload_GetEnvFromSources(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}},
	}
	ds.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:    "init",
			EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "secret1"}}}},
		},
	}

	w := NewDaemonSetWorkload(ds)

	sources := w.GetEnvFromSources()
	if len(sources) != 2 {
		t.Errorf("GetEnvFromSources() returned %d sources, want 2", len(sources))
	}
}

func TestDaemonSetWorkload_DeepCopy(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)

	w := NewDaemonSetWorkload(ds)
	copy := w.DeepCopy()

	w.SetPodTemplateAnnotation("modified", "true")

	copyAnnotations := copy.GetPodTemplateAnnotations()
	if copyAnnotations["modified"] == "true" {
		t.Error("DeepCopy should create independent copy")
	}
}

func TestDaemonSetWorkload_GetOwnerReferences(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	ds.OwnerReferences = []metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "test-owner"},
	}

	w := NewDaemonSetWorkload(ds)

	refs := w.GetOwnerReferences()
	if len(refs) != 1 || refs[0].Name != "test-owner" {
		t.Errorf("GetOwnerReferences() = %v, want owner ref to test-owner", refs)
	}
}

// StatefulSet tests
func TestStatefulSetWorkload_BasicGetters(t *testing.T) {
	sts := testutil.NewStatefulSet("test-sts", "test-ns", map[string]string{"key": "value"})

	w := NewStatefulSetWorkload(sts)

	if w.Kind() != KindStatefulSet {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindStatefulSet)
	}
	if w.GetName() != "test-sts" {
		t.Errorf("GetName() = %v, want test-sts", w.GetName())
	}
	if w.GetNamespace() != "test-ns" {
		t.Errorf("GetNamespace() = %v, want test-ns", w.GetNamespace())
	}
	if w.GetAnnotations()["key"] != "value" {
		t.Errorf("GetAnnotations()[key] = %v, want value", w.GetAnnotations()["key"])
	}
	if w.GetObject() != sts {
		t.Error("GetObject() should return the underlying statefulset")
	}
}

func TestStatefulSetWorkload_PodTemplateAnnotations(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Annotations["existing"] = "annotation"

	w := NewStatefulSetWorkload(sts)

	annotations := w.GetPodTemplateAnnotations()
	if annotations["existing"] != "annotation" {
		t.Errorf("GetPodTemplateAnnotations()[existing] = %v, want annotation", annotations["existing"])
	}

	w.SetPodTemplateAnnotation("new-key", "new-value")
	if w.GetPodTemplateAnnotations()["new-key"] != "new-value" {
		t.Error("SetPodTemplateAnnotation should add new annotation")
	}
}

func TestStatefulSetWorkload_PodTemplateAnnotations_NilInit(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Annotations = nil

	w := NewStatefulSetWorkload(sts)

	annotations := w.GetPodTemplateAnnotations()
	if annotations == nil {
		t.Error("GetPodTemplateAnnotations should initialize nil map")
	}

	w.SetPodTemplateAnnotation("key", "value")
	if w.GetPodTemplateAnnotations()["key"] != "value" {
		t.Error("SetPodTemplateAnnotation should work with nil initial map")
	}
}

func TestStatefulSetWorkload_Containers(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "busybox"}}

	w := NewStatefulSetWorkload(sts)

	containers := w.GetContainers()
	if len(containers) != 1 || containers[0].Name != "main" {
		t.Errorf("GetContainers() = %v, want [main]", containers)
	}

	initContainers := w.GetInitContainers()
	if len(initContainers) != 1 || initContainers[0].Name != "init" {
		t.Errorf("GetInitContainers() = %v, want [init]", initContainers)
	}

	newContainers := []corev1.Container{{Name: "new-main", Image: "alpine"}}
	w.SetContainers(newContainers)
	if w.GetContainers()[0].Name != "new-main" {
		t.Error("SetContainers should update containers")
	}

	newInitContainers := []corev1.Container{{Name: "new-init", Image: "alpine"}}
	w.SetInitContainers(newInitContainers)
	if w.GetInitContainers()[0].Name != "new-init" {
		t.Error("SetInitContainers should update init containers")
	}
}

func TestStatefulSetWorkload_Volumes(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "config-vol"},
		{Name: "secret-vol"},
	}

	w := NewStatefulSetWorkload(sts)

	volumes := w.GetVolumes()
	if len(volumes) != 2 {
		t.Errorf("GetVolumes() length = %d, want 2", len(volumes))
	}
}

func TestStatefulSetWorkload_UsesConfigMap(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "sts-config"},
				},
			},
		},
	}

	w := NewStatefulSetWorkload(sts)

	if !w.UsesConfigMap("sts-config") {
		t.Error("StatefulSet UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestStatefulSetWorkload_UsesConfigMap_EnvFrom(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "sts-env-config"}}},
	}

	w := NewStatefulSetWorkload(sts)

	if !w.UsesConfigMap("sts-env-config") {
		t.Error("StatefulSet UsesConfigMap should return true for envFrom ConfigMap")
	}
}

func TestStatefulSetWorkload_UsesSecret(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "secret-vol",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "sts-secret"},
			},
		},
	}

	w := NewStatefulSetWorkload(sts)

	if !w.UsesSecret("sts-secret") {
		t.Error("StatefulSet UsesSecret should return true for Secret volume")
	}
	if w.UsesSecret("other-secret") {
		t.Error("UsesSecret should return false for non-existent Secret")
	}
}

func TestStatefulSetWorkload_UsesSecret_EnvFrom(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "sts-env-secret"}}},
	}

	w := NewStatefulSetWorkload(sts)

	if !w.UsesSecret("sts-env-secret") {
		t.Error("StatefulSet UsesSecret should return true for envFrom Secret")
	}
}

func TestStatefulSetWorkload_GetEnvFromSources(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}},
	}
	sts.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:    "init",
			EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "secret1"}}}},
		},
	}

	w := NewStatefulSetWorkload(sts)

	sources := w.GetEnvFromSources()
	if len(sources) != 2 {
		t.Errorf("GetEnvFromSources() returned %d sources, want 2", len(sources))
	}
}

func TestStatefulSetWorkload_DeepCopy(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)

	w := NewStatefulSetWorkload(sts)
	copy := w.DeepCopy()

	w.SetPodTemplateAnnotation("modified", "true")

	copyAnnotations := copy.GetPodTemplateAnnotations()
	if copyAnnotations["modified"] == "true" {
		t.Error("DeepCopy should create independent copy")
	}
}

func TestStatefulSetWorkload_GetOwnerReferences(t *testing.T) {
	sts := testutil.NewStatefulSet("test", "default", nil)
	sts.OwnerReferences = []metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "test-owner"},
	}

	w := NewStatefulSetWorkload(sts)

	refs := w.GetOwnerReferences()
	if len(refs) != 1 || refs[0].Name != "test-owner" {
		t.Errorf("GetOwnerReferences() = %v, want owner ref to test-owner", refs)
	}
}

// Test that workloads implement the interface
func TestWorkloadInterface(t *testing.T) {
	var _ Workload = (*DeploymentWorkload)(nil)
	var _ Workload = (*DaemonSetWorkload)(nil)
	var _ Workload = (*StatefulSetWorkload)(nil)
	var _ Workload = (*RolloutWorkload)(nil)
}

// RolloutWorkload tests
func TestRolloutWorkload_BasicGetters(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rollout",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"key": "value",
			},
		},
	}

	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)

	if w.Kind() != KindArgoRollout {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindArgoRollout)
	}
	if w.GetName() != "test-rollout" {
		t.Errorf("GetName() = %v, want test-rollout", w.GetName())
	}
	if w.GetNamespace() != "test-ns" {
		t.Errorf("GetNamespace() = %v, want test-ns", w.GetNamespace())
	}
	if w.GetAnnotations()["key"] != "value" {
		t.Errorf("GetAnnotations()[key] = %v, want value", w.GetAnnotations()["key"])
	}
	if w.GetObject() != rollout {
		t.Error("GetObject() should return the underlying rollout")
	}
}

func TestRolloutWorkload_PodTemplateAnnotations(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: argorolloutv1alpha1.RolloutSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"existing": "annotation",
					},
				},
			},
		},
	}

	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)

	// Test get
	annotations := w.GetPodTemplateAnnotations()
	if annotations["existing"] != "annotation" {
		t.Errorf("GetPodTemplateAnnotations()[existing] = %v, want annotation", annotations["existing"])
	}

	// Test set
	w.SetPodTemplateAnnotation("new-key", "new-value")
	if w.GetPodTemplateAnnotations()["new-key"] != "new-value" {
		t.Error("SetPodTemplateAnnotation should add new annotation")
	}
}

func TestRolloutWorkload_GetStrategy_Default(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)

	if w.GetStrategy() != RolloutStrategyRollout {
		t.Errorf("GetStrategy() = %v, want %v (default)", w.GetStrategy(), RolloutStrategyRollout)
	}
}

func TestRolloutWorkload_GetStrategy_Restart(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				testRolloutStrategyAnnotation: "restart",
			},
		},
	}

	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)

	if w.GetStrategy() != RolloutStrategyRestart {
		t.Errorf("GetStrategy() = %v, want %v", w.GetStrategy(), RolloutStrategyRestart)
	}
}

func TestRolloutWorkload_UsesConfigMap_Volume(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: argorolloutv1alpha1.RolloutSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "config-vol",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "rollout-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)

	if !w.UsesConfigMap("rollout-config") {
		t.Error("Rollout UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("Rollout UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestRolloutWorkload_UsesSecret_EnvFrom(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: argorolloutv1alpha1.RolloutSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "rollout-secret",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)

	if !w.UsesSecret("rollout-secret") {
		t.Error("Rollout UsesSecret should return true for Secret envFrom")
	}
	if w.UsesSecret("other-secret") {
		t.Error("Rollout UsesSecret should return false for non-existent Secret")
	}
}

func TestRolloutWorkload_DeepCopy(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: argorolloutv1alpha1.RolloutSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"original": "value",
					},
				},
			},
		},
	}

	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)
	copy := w.DeepCopy()

	// Verify copy is independent
	w.SetPodTemplateAnnotation("modified", "true")

	copyAnnotations := copy.(*RolloutWorkload).GetPodTemplateAnnotations()
	if copyAnnotations["modified"] == "true" {
		t.Error("DeepCopy should create independent copy")
	}
}

func TestRolloutStrategy_Validate(t *testing.T) {
	tests := []struct {
		strategy RolloutStrategy
		wantErr  bool
	}{
		{RolloutStrategyRollout, false},
		{RolloutStrategyRestart, false},
		{RolloutStrategy("invalid"), true},
		{RolloutStrategy(""), true},
	}

	for _, tt := range tests {
		err := tt.strategy.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate(%s) error = %v, wantErr %v", tt.strategy, err, tt.wantErr)
		}
	}
}

func TestToRolloutStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected RolloutStrategy
	}{
		{"rollout", RolloutStrategyRollout},
		{"restart", RolloutStrategyRestart},
		{"invalid", RolloutStrategyRollout}, // defaults to rollout
		{"", RolloutStrategyRollout},        // defaults to rollout
	}

	for _, tt := range tests {
		result := ToRolloutStrategy(tt.input)
		if result != tt.expected {
			t.Errorf("ToRolloutStrategy(%s) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// Job tests
func TestJobWorkload_BasicGetters(t *testing.T) {
	job := testutil.NewJobWithAnnotations("test-job", "test-ns", map[string]string{"key": "value"})

	w := NewJobWorkload(job)

	if w.Kind() != KindJob {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindJob)
	}
	if w.GetName() != "test-job" {
		t.Errorf("GetName() = %v, want test-job", w.GetName())
	}
	if w.GetNamespace() != "test-ns" {
		t.Errorf("GetNamespace() = %v, want test-ns", w.GetNamespace())
	}
	if w.GetAnnotations()["key"] != "value" {
		t.Errorf("GetAnnotations()[key] = %v, want value", w.GetAnnotations()["key"])
	}
	if w.GetObject() != job {
		t.Error("GetObject() should return the underlying job")
	}
}

func TestJobWorkload_PodTemplateAnnotations(t *testing.T) {
	job := testutil.NewJob("test", "default")
	job.Spec.Template.Annotations["existing"] = "annotation"

	w := NewJobWorkload(job)

	annotations := w.GetPodTemplateAnnotations()
	if annotations["existing"] != "annotation" {
		t.Errorf("GetPodTemplateAnnotations()[existing] = %v, want annotation", annotations["existing"])
	}

	w.SetPodTemplateAnnotation("new-key", "new-value")
	if w.GetPodTemplateAnnotations()["new-key"] != "new-value" {
		t.Error("SetPodTemplateAnnotation should add new annotation")
	}
}

func TestJobWorkload_UsesConfigMap(t *testing.T) {
	job := testutil.NewJob("test", "default")
	job.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "job-config"},
				},
			},
		},
	}

	w := NewJobWorkload(job)

	if !w.UsesConfigMap("job-config") {
		t.Error("Job UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("Job UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestJobWorkload_UsesSecret(t *testing.T) {
	job := testutil.NewJob("test", "default")
	job.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "job-secret"},
			},
		},
	}

	w := NewJobWorkload(job)

	if !w.UsesSecret("job-secret") {
		t.Error("Job UsesSecret should return true for Secret envFrom")
	}
}

func TestJobWorkload_DeepCopy(t *testing.T) {
	job := testutil.NewJob("test", "default")
	job.Spec.Template.Annotations["original"] = "value"

	w := NewJobWorkload(job)
	copy := w.DeepCopy()

	w.SetPodTemplateAnnotation("modified", "true")

	copyAnnotations := copy.GetPodTemplateAnnotations()
	if copyAnnotations["modified"] == "true" {
		t.Error("DeepCopy should create independent copy")
	}
}

// CronJob tests
func TestCronJobWorkload_BasicGetters(t *testing.T) {
	cj := testutil.NewCronJobWithAnnotations("test-cronjob", "test-ns", map[string]string{"key": "value"})

	w := NewCronJobWorkload(cj)

	if w.Kind() != KindCronJob {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindCronJob)
	}
	if w.GetName() != "test-cronjob" {
		t.Errorf("GetName() = %v, want test-cronjob", w.GetName())
	}
	if w.GetNamespace() != "test-ns" {
		t.Errorf("GetNamespace() = %v, want test-ns", w.GetNamespace())
	}
	if w.GetAnnotations()["key"] != "value" {
		t.Errorf("GetAnnotations()[key] = %v, want value", w.GetAnnotations()["key"])
	}
	if w.GetObject() != cj {
		t.Error("GetObject() should return the underlying cronjob")
	}
}

func TestCronJobWorkload_PodTemplateAnnotations(t *testing.T) {
	cj := testutil.NewCronJob("test", "default")
	cj.Spec.JobTemplate.Spec.Template.Annotations["existing"] = "annotation"

	w := NewCronJobWorkload(cj)

	annotations := w.GetPodTemplateAnnotations()
	if annotations["existing"] != "annotation" {
		t.Errorf("GetPodTemplateAnnotations()[existing] = %v, want annotation", annotations["existing"])
	}

	w.SetPodTemplateAnnotation("new-key", "new-value")
	if w.GetPodTemplateAnnotations()["new-key"] != "new-value" {
		t.Error("SetPodTemplateAnnotation should add new annotation")
	}
}

func TestCronJobWorkload_UsesConfigMap(t *testing.T) {
	cj := testutil.NewCronJob("test", "default")
	cj.Spec.JobTemplate.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "cronjob-config"},
				},
			},
		},
	}

	w := NewCronJobWorkload(cj)

	if !w.UsesConfigMap("cronjob-config") {
		t.Error("CronJob UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("CronJob UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestCronJobWorkload_UsesSecret(t *testing.T) {
	cj := testutil.NewCronJob("test", "default")
	addEnvVarSecretRef(cj.Spec.JobTemplate.Spec.Template.Spec.Containers, "SECRET_VALUE", "cronjob-secret", "key")

	w := NewCronJobWorkload(cj)

	if !w.UsesSecret("cronjob-secret") {
		t.Error("CronJob UsesSecret should return true for Secret envVar")
	}
}

func TestCronJobWorkload_DeepCopy(t *testing.T) {
	cj := testutil.NewCronJob("test", "default")
	cj.Spec.JobTemplate.Spec.Template.Annotations["original"] = "value"

	w := NewCronJobWorkload(cj)
	copy := w.DeepCopy()

	w.SetPodTemplateAnnotation("modified", "true")

	copyAnnotations := copy.GetPodTemplateAnnotations()
	if copyAnnotations["modified"] == "true" {
		t.Error("DeepCopy should create independent copy")
	}
}

// Test that Job and CronJob implement the interface
func TestJobCronJobWorkloadInterface(t *testing.T) {
	var _ Workload = (*JobWorkload)(nil)
	var _ Workload = (*CronJobWorkload)(nil)
}

// DeploymentConfig tests
func TestDeploymentConfigWorkload_BasicGetters(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test-dc", "test-ns", map[string]string{"key": "value"})

	w := NewDeploymentConfigWorkload(dc)

	if w.Kind() != KindDeploymentConfig {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindDeploymentConfig)
	}
	if w.GetName() != "test-dc" {
		t.Errorf("GetName() = %v, want test-dc", w.GetName())
	}
	if w.GetNamespace() != "test-ns" {
		t.Errorf("GetNamespace() = %v, want test-ns", w.GetNamespace())
	}
	if w.GetAnnotations()["key"] != "value" {
		t.Errorf("GetAnnotations()[key] = %v, want value", w.GetAnnotations()["key"])
	}
	if w.GetObject() != dc {
		t.Error("GetObject() should return the underlying deploymentconfig")
	}
}

func TestDeploymentConfigWorkload_PodTemplateAnnotations(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Annotations = map[string]string{"existing": "annotation"}

	w := NewDeploymentConfigWorkload(dc)

	annotations := w.GetPodTemplateAnnotations()
	if annotations["existing"] != "annotation" {
		t.Errorf("GetPodTemplateAnnotations()[existing] = %v, want annotation", annotations["existing"])
	}

	w.SetPodTemplateAnnotation("new-key", "new-value")
	if w.GetPodTemplateAnnotations()["new-key"] != "new-value" {
		t.Error("SetPodTemplateAnnotation should add new annotation")
	}
}

func TestDeploymentConfigWorkload_PodTemplateAnnotations_NilTemplate(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template = nil

	w := NewDeploymentConfigWorkload(dc)

	// Should handle nil template gracefully
	annotations := w.GetPodTemplateAnnotations()
	if annotations != nil {
		t.Error("GetPodTemplateAnnotations should return nil for nil template")
	}

	// SetPodTemplateAnnotation should initialize template
	w.SetPodTemplateAnnotation("key", "value")
	if w.GetPodTemplateAnnotations()["key"] != "value" {
		t.Error("SetPodTemplateAnnotation should work with nil template")
	}
}

func TestDeploymentConfigWorkload_PodTemplateAnnotations_NilInit(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Annotations = nil

	w := NewDeploymentConfigWorkload(dc)

	// Should initialize nil map
	annotations := w.GetPodTemplateAnnotations()
	if annotations == nil {
		t.Error("GetPodTemplateAnnotations should initialize nil map")
	}

	w.SetPodTemplateAnnotation("key", "value")
	if w.GetPodTemplateAnnotations()["key"] != "value" {
		t.Error("SetPodTemplateAnnotation should work with nil initial map")
	}
}

func TestDeploymentConfigWorkload_Containers(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Spec.Containers = []corev1.Container{
		{Name: "main", Image: "nginx"},
	}
	dc.Spec.Template.Spec.InitContainers = []corev1.Container{
		{Name: "init", Image: "busybox"},
	}

	w := NewDeploymentConfigWorkload(dc)

	containers := w.GetContainers()
	if len(containers) != 1 || containers[0].Name != "main" {
		t.Errorf("GetContainers() = %v, want [main]", containers)
	}

	initContainers := w.GetInitContainers()
	if len(initContainers) != 1 || initContainers[0].Name != "init" {
		t.Errorf("GetInitContainers() = %v, want [init]", initContainers)
	}

	newContainers := []corev1.Container{{Name: "new-main", Image: "alpine"}}
	w.SetContainers(newContainers)
	if w.GetContainers()[0].Name != "new-main" {
		t.Error("SetContainers should update containers")
	}

	newInitContainers := []corev1.Container{{Name: "new-init", Image: "alpine"}}
	w.SetInitContainers(newInitContainers)
	if w.GetInitContainers()[0].Name != "new-init" {
		t.Error("SetInitContainers should update init containers")
	}
}

func TestDeploymentConfigWorkload_Containers_NilTemplate(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template = nil

	w := NewDeploymentConfigWorkload(dc)

	if w.GetContainers() != nil {
		t.Error("GetContainers should return nil for nil template")
	}
	if w.GetInitContainers() != nil {
		t.Error("GetInitContainers should return nil for nil template")
	}

	// SetContainers should initialize template
	w.SetContainers([]corev1.Container{{Name: "main"}})
	if len(w.GetContainers()) != 1 {
		t.Error("SetContainers should work with nil template")
	}
}

func TestDeploymentConfigWorkload_Volumes(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "config-vol"},
		{Name: "secret-vol"},
	}

	w := NewDeploymentConfigWorkload(dc)

	volumes := w.GetVolumes()
	if len(volumes) != 2 {
		t.Errorf("GetVolumes() length = %d, want 2", len(volumes))
	}
}

func TestDeploymentConfigWorkload_Volumes_NilTemplate(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template = nil

	w := NewDeploymentConfigWorkload(dc)

	if w.GetVolumes() != nil {
		t.Error("GetVolumes should return nil for nil template")
	}
}

func TestDeploymentConfigWorkload_UsesConfigMap_Volume(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "dc-config",
					},
				},
			},
		},
	}

	w := NewDeploymentConfigWorkload(dc)

	if !w.UsesConfigMap("dc-config") {
		t.Error("DeploymentConfig UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestDeploymentConfigWorkload_UsesConfigMap_EnvFrom(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name: "main",
			EnvFrom: []corev1.EnvFromSource{
				{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "dc-env-config",
						},
					},
				},
			},
		},
	}

	w := NewDeploymentConfigWorkload(dc)

	if !w.UsesConfigMap("dc-env-config") {
		t.Error("DeploymentConfig UsesConfigMap should return true for envFrom ConfigMap")
	}
}

func TestDeploymentConfigWorkload_UsesConfigMap_NilTemplate(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template = nil

	w := NewDeploymentConfigWorkload(dc)

	if w.UsesConfigMap("any-config") {
		t.Error("UsesConfigMap should return false for nil template")
	}
}

func TestDeploymentConfigWorkload_UsesSecret_Volume(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "secret-vol",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "dc-secret",
				},
			},
		},
	}

	w := NewDeploymentConfigWorkload(dc)

	if !w.UsesSecret("dc-secret") {
		t.Error("DeploymentConfig UsesSecret should return true for Secret volume")
	}
	if w.UsesSecret("other-secret") {
		t.Error("UsesSecret should return false for non-existent Secret")
	}
}

func TestDeploymentConfigWorkload_UsesSecret_EnvFrom(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name: "main",
			EnvFrom: []corev1.EnvFromSource{
				{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "dc-env-secret",
						},
					},
				},
			},
		},
	}

	w := NewDeploymentConfigWorkload(dc)

	if !w.UsesSecret("dc-env-secret") {
		t.Error("DeploymentConfig UsesSecret should return true for envFrom Secret")
	}
}

func TestDeploymentConfigWorkload_UsesSecret_NilTemplate(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template = nil

	w := NewDeploymentConfigWorkload(dc)

	if w.UsesSecret("any-secret") {
		t.Error("UsesSecret should return false for nil template")
	}
}

func TestDeploymentConfigWorkload_GetEnvFromSources(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name: "main",
			EnvFrom: []corev1.EnvFromSource{
				{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}},
			},
		},
	}
	dc.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name: "init",
			EnvFrom: []corev1.EnvFromSource{
				{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "secret1"}}},
			},
		},
	}

	w := NewDeploymentConfigWorkload(dc)

	sources := w.GetEnvFromSources()
	if len(sources) != 2 {
		t.Errorf("GetEnvFromSources() returned %d sources, want 2", len(sources))
	}
}

func TestDeploymentConfigWorkload_GetEnvFromSources_NilTemplate(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template = nil

	w := NewDeploymentConfigWorkload(dc)

	if w.GetEnvFromSources() != nil {
		t.Error("GetEnvFromSources should return nil for nil template")
	}
}

func TestDeploymentConfigWorkload_DeepCopy(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.Spec.Template.Annotations = map[string]string{"original": "value"}

	w := NewDeploymentConfigWorkload(dc)
	copy := w.DeepCopy()

	w.SetPodTemplateAnnotation("modified", "true")

	copyAnnotations := copy.GetPodTemplateAnnotations()
	if copyAnnotations["modified"] == "true" {
		t.Error("DeepCopy should create independent copy")
	}
}

func TestDeploymentConfigWorkload_GetOwnerReferences(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)
	dc.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "apps.openshift.io/v1",
			Kind:       "DeploymentConfig",
			Name:       "test-owner",
		},
	}

	w := NewDeploymentConfigWorkload(dc)

	refs := w.GetOwnerReferences()
	if len(refs) != 1 || refs[0].Name != "test-owner" {
		t.Errorf("GetOwnerReferences() = %v, want owner ref to test-owner", refs)
	}
}

func TestDeploymentConfigWorkload_GetDeploymentConfig(t *testing.T) {
	dc := testutil.NewDeploymentConfig("test", "default", nil)

	w := NewDeploymentConfigWorkload(dc)

	if w.GetDeploymentConfig() != dc {
		t.Error("GetDeploymentConfig should return the underlying DeploymentConfig")
	}
}

// Test that DeploymentConfig implements the interface
func TestDeploymentConfigWorkloadInterface(t *testing.T) {
	var _ Workload = (*DeploymentConfigWorkload)(nil)
}

// Tests for UpdateStrategy
func TestWorkload_UpdateStrategy(t *testing.T) {
	tests := []struct {
		name     string
		workload Workload
		expected UpdateStrategy
	}{
		{
			name:     "Deployment uses Patch strategy",
			workload: NewDeploymentWorkload(testutil.NewDeployment("test", "default", nil)),
			expected: UpdateStrategyPatch,
		},
		{
			name:     "DaemonSet uses Patch strategy",
			workload: NewDaemonSetWorkload(testutil.NewDaemonSet("test", "default", nil)),
			expected: UpdateStrategyPatch,
		},
		{
			name:     "StatefulSet uses Patch strategy",
			workload: NewStatefulSetWorkload(testutil.NewStatefulSet("test", "default", nil)),
			expected: UpdateStrategyPatch,
		},
		{
			name:     "Job uses Recreate strategy",
			workload: NewJobWorkload(testutil.NewJob("test", "default")),
			expected: UpdateStrategyRecreate,
		},
		{
			name:     "CronJob uses CreateNew strategy",
			workload: NewCronJobWorkload(testutil.NewCronJob("test", "default")),
			expected: UpdateStrategyCreateNew,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.workload.UpdateStrategy(); got != tt.expected {
				t.Errorf("UpdateStrategy() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Tests for ResetOriginal
func TestDeploymentWorkload_ResetOriginal(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	w := NewDeploymentWorkload(deploy)

	// Modify the workload
	w.SetPodTemplateAnnotation("modified", "true")

	// Original should still not have the annotation
	originalAnnotations := w.Original().Spec.Template.Annotations
	if originalAnnotations != nil && originalAnnotations["modified"] == "true" {
		t.Error("Original should not be modified yet")
	}

	// Reset original
	w.ResetOriginal()

	// Now original should have the annotation
	if w.Original().Spec.Template.Annotations["modified"] != "true" {
		t.Error("ResetOriginal should update original to match current state")
	}
}

func TestJobWorkload_ResetOriginal(t *testing.T) {
	job := testutil.NewJob("test", "default")
	w := NewJobWorkload(job)

	// ResetOriginal should be a no-op for Jobs (they don't use strategic merge patch)
	w.SetPodTemplateAnnotation("modified", "true")
	w.ResetOriginal() // Should not panic or error
}

func TestCronJobWorkload_ResetOriginal(t *testing.T) {
	cj := testutil.NewCronJob("test", "default")
	w := NewCronJobWorkload(cj)

	// ResetOriginal should be a no-op for CronJobs
	w.SetPodTemplateAnnotation("modified", "true")
	w.ResetOriginal() // Should not panic or error
}

// Tests for BaseWorkload.Original()
func TestDeploymentWorkload_Original(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Annotations = map[string]string{"initial": "value"}

	w := NewDeploymentWorkload(deploy)

	// Modify the current object
	w.SetPodTemplateAnnotation("new", "annotation")

	// Original should still have only the initial annotation
	original := w.Original()
	if original.Spec.Template.Annotations["new"] == "annotation" {
		t.Error("Original should not reflect changes to current object")
	}
	if original.Spec.Template.Annotations["initial"] != "value" {
		t.Error("Original should retain initial state")
	}
}

// Tests for PerformSpecialUpdate returning false for standard workloads
func TestDeploymentWorkload_PerformSpecialUpdate(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	w := NewDeploymentWorkload(deploy)

	updated, err := w.PerformSpecialUpdate(t.Context(), nil)
	if err != nil {
		t.Errorf("PerformSpecialUpdate() error = %v", err)
	}
	if updated {
		t.Error("PerformSpecialUpdate() should return false for Deployment")
	}
}

func TestDaemonSetWorkload_PerformSpecialUpdate(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	w := NewDaemonSetWorkload(ds)

	updated, err := w.PerformSpecialUpdate(t.Context(), nil)
	if err != nil {
		t.Errorf("PerformSpecialUpdate() error = %v", err)
	}
	if updated {
		t.Error("PerformSpecialUpdate() should return false for DaemonSet")
	}
}

func TestStatefulSetWorkload_PerformSpecialUpdate(t *testing.T) {
	ss := testutil.NewStatefulSet("test", "default", nil)
	w := NewStatefulSetWorkload(ss)

	updated, err := w.PerformSpecialUpdate(t.Context(), nil)
	if err != nil {
		t.Errorf("PerformSpecialUpdate() error = %v", err)
	}
	if updated {
		t.Error("PerformSpecialUpdate() should return false for StatefulSet")
	}
}

// Test Update returns nil for Job (no-op, uses PerformSpecialUpdate instead)
func TestJobWorkload_Update(t *testing.T) {
	job := testutil.NewJob("test", "default")
	w := NewJobWorkload(job)

	err := w.Update(t.Context(), nil)
	if err != nil {
		t.Errorf("Update() should return nil for Job, got %v", err)
	}
}

// Test Update returns nil for CronJob (no-op, uses PerformSpecialUpdate instead)
func TestCronJobWorkload_Update(t *testing.T) {
	cj := testutil.NewCronJob("test", "default")
	w := NewCronJobWorkload(cj)

	err := w.Update(t.Context(), nil)
	if err != nil {
		t.Errorf("Update() should return nil for CronJob, got %v", err)
	}
}

// Test GetJob and GetCronJob accessors
func TestJobWorkload_GetJob(t *testing.T) {
	job := testutil.NewJob("test", "default")
	w := NewJobWorkload(job)

	if w.GetJob() != job {
		t.Error("GetJob should return the underlying Job")
	}
}

func TestCronJobWorkload_GetCronJob(t *testing.T) {
	cj := testutil.NewCronJob("test", "default")
	w := NewCronJobWorkload(cj)

	if w.GetCronJob() != cj {
		t.Error("GetCronJob should return the underlying CronJob")
	}
}

func TestDeploymentWorkload_GetDeployment(t *testing.T) {
	deploy := testutil.NewDeployment("test", "default", nil)
	w := NewDeploymentWorkload(deploy)

	if w.GetDeployment() != deploy {
		t.Error("GetDeployment should return the underlying Deployment")
	}
}

func TestDaemonSetWorkload_GetDaemonSet(t *testing.T) {
	ds := testutil.NewDaemonSet("test", "default", nil)
	w := NewDaemonSetWorkload(ds)

	if w.GetDaemonSet() != ds {
		t.Error("GetDaemonSet should return the underlying DaemonSet")
	}
}

func TestStatefulSetWorkload_GetStatefulSet(t *testing.T) {
	ss := testutil.NewStatefulSet("test", "default", nil)
	w := NewStatefulSetWorkload(ss)

	if w.GetStatefulSet() != ss {
		t.Error("GetStatefulSet should return the underlying StatefulSet")
	}
}

func TestRolloutWorkload_GetRollout(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	w := NewRolloutWorkload(rollout, testRolloutStrategyAnnotation)

	if w.GetRollout() != rollout {
		t.Error("GetRollout should return the underlying Rollout")
	}
}
