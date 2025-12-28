package testutil

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewScheme creates a scheme with common types for testing.
func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	return scheme
}

// NewDeployment creates a minimal Deployment for unit testing.
func NewDeployment(name, namespace string, annotations map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": name},
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}
}

// NewDeploymentWithEnvFrom creates a Deployment with EnvFrom referencing a ConfigMap or Secret.
func NewDeploymentWithEnvFrom(name, namespace string, configMapName, secretName string) *appsv1.Deployment {
	d := NewDeployment(name, namespace, nil)
	if configMapName != "" {
		d.Spec.Template.Spec.Containers[0].EnvFrom = append(
			d.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				},
			},
		)
	}
	if secretName != "" {
		d.Spec.Template.Spec.Containers[0].EnvFrom = append(
			d.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			},
		)
	}
	return d
}

// NewDeploymentWithVolume creates a Deployment with a volume from ConfigMap or Secret.
func NewDeploymentWithVolume(name, namespace string, configMapName, secretName string) *appsv1.Deployment {
	d := NewDeployment(name, namespace, nil)
	d.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{
		Name:      "config",
		MountPath: "/etc/config",
	}}

	if configMapName != "" {
		d.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				},
			},
		}}
	}
	if secretName != "" {
		d.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}}
	}
	return d
}

// NewDeploymentWithProjectedVolume creates a Deployment with a projected volume.
func NewDeploymentWithProjectedVolume(name, namespace string, configMapName, secretName string) *appsv1.Deployment {
	d := NewDeployment(name, namespace, nil)
	d.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{
		Name:      "config",
		MountPath: "/etc/config",
	}}

	sources := []corev1.VolumeProjection{}
	if configMapName != "" {
		sources = append(sources, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
			},
		})
	}
	if secretName != "" {
		sources = append(sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			},
		})
	}

	d.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{Sources: sources},
		},
	}}
	return d
}

// NewDaemonSet creates a minimal DaemonSet for unit testing.
func NewDaemonSet(name, namespace string, annotations map[string]string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": name},
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}
}

// NewStatefulSet creates a minimal StatefulSet for unit testing.
func NewStatefulSet(name, namespace string, annotations map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": name},
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}
}

// NewJob creates a minimal Job for unit testing.
func NewJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "busybox",
					}},
				},
			},
		},
	}
}

// NewCronJob creates a minimal CronJob for unit testing.
func NewCronJob(name, namespace string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{{
								Name:  "main",
								Image: "busybox",
							}},
						},
					},
				},
			},
		},
	}
}

// NewConfigMap creates a ConfigMap for unit testing.
func NewConfigMap(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{"key": "value"},
	}
}

// NewConfigMapWithAnnotations creates a ConfigMap with annotations.
func NewConfigMapWithAnnotations(name, namespace string, annotations map[string]string) *corev1.ConfigMap {
	cm := NewConfigMap(name, namespace)
	cm.Annotations = annotations
	return cm
}

// NewSecret creates a Secret for unit testing.
func NewSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{"key": []byte("value")},
	}
}

// NewSecretWithAnnotations creates a Secret with annotations.
func NewSecretWithAnnotations(name, namespace string, annotations map[string]string) *corev1.Secret {
	secret := NewSecret(name, namespace)
	secret.Annotations = annotations
	return secret
}

// NewNamespace creates a Namespace with optional labels.
func NewNamespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
