package workload

import (
	"testing"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeploymentWorkload_BasicGetters(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"key": "value",
			},
		},
	}

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
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"existing": "annotation",
					},
				},
			},
		},
	}

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
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				// No annotations set
			},
		},
	}

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
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "nginx"},
					},
					InitContainers: []corev1.Container{
						{Name: "init", Image: "busybox"},
					},
				},
			},
		},
	}

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
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{Name: "config-vol"},
						{Name: "secret-vol"},
					},
				},
			},
		},
	}

	w := NewDeploymentWorkload(deploy)

	volumes := w.GetVolumes()
	if len(volumes) != 2 {
		t.Errorf("GetVolumes() length = %d, want 2", len(volumes))
	}
}

func TestDeploymentWorkload_UsesConfigMap_Volume(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "config-vol",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "my-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("my-config") {
		t.Error("UsesConfigMap should return true for ConfigMap volume")
	}
	if w.UsesConfigMap("other-config") {
		t.Error("UsesConfigMap should return false for non-existent ConfigMap")
	}
}

func TestDeploymentWorkload_UsesConfigMap_ProjectedVolume(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "projected-vol",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											ConfigMap: &corev1.ConfigMapProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "projected-config",
												},
											},
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("projected-config") {
		t.Error("UsesConfigMap should return true for projected ConfigMap volume")
	}
}

func TestDeploymentWorkload_UsesConfigMap_EnvFrom(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "env-config",
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("env-config") {
		t.Error("UsesConfigMap should return true for envFrom ConfigMap")
	}
}

func TestDeploymentWorkload_UsesConfigMap_EnvVar(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Env: []corev1.EnvVar{
								{
									Name: "CONFIG_VALUE",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "var-config",
											},
											Key: "some-key",
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("var-config") {
		t.Error("UsesConfigMap should return true for env var ConfigMapKeyRef")
	}
}

func TestDeploymentWorkload_UsesConfigMap_InitContainer(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "init",
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "init-config",
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesConfigMap("init-config") {
		t.Error("UsesConfigMap should return true for init container ConfigMap")
	}
}

func TestDeploymentWorkload_UsesSecret_Volume(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "secret-vol",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "my-secret",
								},
							},
						},
					},
				},
			},
		},
	}

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("my-secret") {
		t.Error("UsesSecret should return true for Secret volume")
	}
	if w.UsesSecret("other-secret") {
		t.Error("UsesSecret should return false for non-existent Secret")
	}
}

func TestDeploymentWorkload_UsesSecret_ProjectedVolume(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "projected-vol",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "projected-secret",
												},
											},
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("projected-secret") {
		t.Error("UsesSecret should return true for projected Secret volume")
	}
}

func TestDeploymentWorkload_UsesSecret_EnvFrom(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "env-secret",
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("env-secret") {
		t.Error("UsesSecret should return true for envFrom Secret")
	}
}

func TestDeploymentWorkload_UsesSecret_EnvVar(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Env: []corev1.EnvVar{
								{
									Name: "SECRET_VALUE",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "var-secret",
											},
											Key: "some-key",
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("var-secret") {
		t.Error("UsesSecret should return true for env var SecretKeyRef")
	}
}

func TestDeploymentWorkload_UsesSecret_InitContainer(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "init",
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "init-secret",
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

	w := NewDeploymentWorkload(deploy)

	if !w.UsesSecret("init-secret") {
		t.Error("UsesSecret should return true for init container Secret")
	}
}

func TestDeploymentWorkload_GetEnvFromSources(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							EnvFrom: []corev1.EnvFromSource{
								{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}},
							},
						},
						{
							Name: "sidecar",
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "secret1"}}},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name: "init",
							EnvFrom: []corev1.EnvFromSource{
								{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "init-cm"}}},
							},
						},
					},
				},
			},
		},
	}

	w := NewDeploymentWorkload(deploy)

	sources := w.GetEnvFromSources()
	if len(sources) != 3 {
		t.Errorf("GetEnvFromSources() returned %d sources, want 3", len(sources))
	}
}

func TestDeploymentWorkload_DeepCopy(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "nginx"},
					},
				},
			},
		},
	}

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
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-rs",
				},
			},
		},
	}

	w := NewDeploymentWorkload(deploy)

	refs := w.GetOwnerReferences()
	if len(refs) != 1 || refs[0].Name != "test-rs" {
		t.Errorf("GetOwnerReferences() = %v, want owner ref to test-rs", refs)
	}
}

// DaemonSet tests
func TestDaemonSetWorkload_BasicGetters(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ds",
			Namespace: "test-ns",
		},
	}

	w := NewDaemonSetWorkload(ds)

	if w.Kind() != KindDaemonSet {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindDaemonSet)
	}
	if w.GetName() != "test-ds" {
		t.Errorf("GetName() = %v, want test-ds", w.GetName())
	}
}

func TestDaemonSetWorkload_UsesConfigMap(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "config-vol",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "ds-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	w := NewDaemonSetWorkload(ds)

	if !w.UsesConfigMap("ds-config") {
		t.Error("DaemonSet UsesConfigMap should return true for ConfigMap volume")
	}
}

// StatefulSet tests
func TestStatefulSetWorkload_BasicGetters(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "test-ns",
		},
	}

	w := NewStatefulSetWorkload(sts)

	if w.Kind() != KindStatefulSet {
		t.Errorf("Kind() = %v, want %v", w.Kind(), KindStatefulSet)
	}
	if w.GetName() != "test-sts" {
		t.Errorf("GetName() = %v, want test-sts", w.GetName())
	}
}

func TestStatefulSetWorkload_UsesSecret(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "secret-vol",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "sts-secret",
								},
							},
						},
					},
				},
			},
		},
	}

	w := NewStatefulSetWorkload(sts)

	if !w.UsesSecret("sts-secret") {
		t.Error("StatefulSet UsesSecret should return true for Secret volume")
	}
}

// Test that workloads implement the interface
func TestWorkloadInterface(t *testing.T) {
	var _ WorkloadAccessor = (*DeploymentWorkload)(nil)
	var _ WorkloadAccessor = (*DaemonSetWorkload)(nil)
	var _ WorkloadAccessor = (*StatefulSetWorkload)(nil)
	var _ WorkloadAccessor = (*RolloutWorkload)(nil)
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

	w := NewRolloutWorkload(rollout)

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

	w := NewRolloutWorkload(rollout)

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

	w := NewRolloutWorkload(rollout)

	if w.GetStrategy() != RolloutStrategyRollout {
		t.Errorf("GetStrategy() = %v, want %v (default)", w.GetStrategy(), RolloutStrategyRollout)
	}
}

func TestRolloutWorkload_GetStrategy_Restart(t *testing.T) {
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				RolloutStrategyAnnotation: "restart",
			},
		},
	}

	w := NewRolloutWorkload(rollout)

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

	w := NewRolloutWorkload(rollout)

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

	w := NewRolloutWorkload(rollout)

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

	w := NewRolloutWorkload(rollout)
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
