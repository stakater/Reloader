package utils

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// StatefulSetAdapter implements WorkloadAdapter for Kubernetes StatefulSets.
type StatefulSetAdapter struct {
	client kubernetes.Interface
}

// NewStatefulSetAdapter creates a new StatefulSetAdapter.
func NewStatefulSetAdapter(client kubernetes.Interface) *StatefulSetAdapter {
	return &StatefulSetAdapter{client: client}
}

// Type returns the workload type.
func (a *StatefulSetAdapter) Type() WorkloadType {
	return WorkloadStatefulSet
}

// Create creates a StatefulSet with the given config.
func (a *StatefulSetAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildStatefulSetOptions(cfg)
	_, err := CreateStatefulSet(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the StatefulSet.
func (a *StatefulSetAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteStatefulSet(ctx, a.client, namespace, name)
}

// WaitReady waits for the StatefulSet to be ready.
func (a *StatefulSetAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForStatefulSetReady(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the StatefulSet to have the reload annotation.
func (a *StatefulSetAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForStatefulSetReloaded(ctx, a.client, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the StatefulSet to have a STAKATER_ env var.
func (a *StatefulSetAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForStatefulSetEnvVar(ctx, a.client, namespace, name, prefix, timeout)
}

// SupportsEnvVarStrategy returns true as StatefulSets support env var reload strategy.
func (a *StatefulSetAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as StatefulSets use standard rolling restart.
func (a *StatefulSetAdapter) RequiresSpecialHandling() bool {
	return false
}

// buildStatefulSetOptions converts WorkloadConfig to StatefulSetOption slice.
func buildStatefulSetOptions(cfg WorkloadConfig) []StatefulSetOption {
	var opts []StatefulSetOption

	// Add annotations
	if len(cfg.Annotations) > 0 {
		opts = append(opts, WithStatefulSetAnnotations(cfg.Annotations))
	}

	// Add envFrom references
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		opts = append(opts, WithStatefulSetConfigMapEnvFrom(cfg.ConfigMapName))
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		opts = append(opts, WithStatefulSetSecretEnvFrom(cfg.SecretName))
	}

	// Add volume mounts
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		opts = append(opts, WithStatefulSetConfigMapVolume(cfg.ConfigMapName))
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		opts = append(opts, WithStatefulSetSecretVolume(cfg.SecretName))
	}

	// Add projected volume
	if cfg.UseProjectedVolume {
		opts = append(opts, WithStatefulSetProjectedVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add valueFrom references
	if cfg.UseConfigMapKeyRef && cfg.ConfigMapName != "" {
		key := cfg.ConfigMapKey
		if key == "" {
			key = "key"
		}
		envVar := cfg.EnvVarName
		if envVar == "" {
			envVar = "CONFIG_VAR"
		}
		opts = append(opts, WithStatefulSetConfigMapKeyRef(cfg.ConfigMapName, key, envVar))
	}
	if cfg.UseSecretKeyRef && cfg.SecretName != "" {
		key := cfg.SecretKey
		if key == "" {
			key = "key"
		}
		envVar := cfg.EnvVarName
		if envVar == "" {
			envVar = "SECRET_VAR"
		}
		opts = append(opts, WithStatefulSetSecretKeyRef(cfg.SecretName, key, envVar))
	}

	// Add init container with envFrom
	if cfg.UseInitContainer {
		opts = append(opts, WithStatefulSetInitContainer(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add init container with volume mount
	if cfg.UseInitContainerVolume {
		opts = append(opts, WithStatefulSetInitContainerVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	return opts
}

// WithStatefulSetConfigMapVolume adds a volume mount for a ConfigMap to a StatefulSet.
func WithStatefulSetConfigMapVolume(name string) StatefulSetOption {
	return func(ss *appsv1.StatefulSet) {
		volumeName := fmt.Sprintf("cm-%s", name)
		ss.Spec.Template.Spec.Volumes = append(ss.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		})
		ss.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			ss.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/config/%s", name),
			},
		)
	}
}

// WithStatefulSetSecretVolume adds a volume mount for a Secret to a StatefulSet.
func WithStatefulSetSecretVolume(name string) StatefulSetOption {
	return func(ss *appsv1.StatefulSet) {
		volumeName := fmt.Sprintf("secret-%s", name)
		ss.Spec.Template.Spec.Volumes = append(ss.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: name,
				},
			},
		})
		ss.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			ss.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/secrets/%s", name),
			},
		)
	}
}

// WithStatefulSetInitContainer adds an init container that references ConfigMap and/or Secret.
func WithStatefulSetInitContainer(cmName, secretName string) StatefulSetOption {
	return func(ss *appsv1.StatefulSet) {
		initContainer := corev1.Container{
			Name:    "init",
			Image:   DefaultImage,
			Command: []string{"sh", "-c", "echo init done"},
		}

		if cmName != "" {
			initContainer.EnvFrom = append(initContainer.EnvFrom, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			})
		}
		if secretName != "" {
			initContainer.EnvFrom = append(initContainer.EnvFrom, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			})
		}

		ss.Spec.Template.Spec.InitContainers = append(ss.Spec.Template.Spec.InitContainers, initContainer)
	}
}

// WithStatefulSetInitContainerVolume adds an init container with ConfigMap/Secret volume mounts.
func WithStatefulSetInitContainerVolume(cmName, secretName string) StatefulSetOption {
	return func(ss *appsv1.StatefulSet) {
		initContainer := corev1.Container{
			Name:    "init",
			Image:   DefaultImage,
			Command: []string{"sh", "-c", "echo init done"},
		}

		if cmName != "" {
			volumeName := fmt.Sprintf("init-cm-%s", cmName)
			ss.Spec.Template.Spec.Volumes = append(ss.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
					},
				},
			})
			initContainer.VolumeMounts = append(initContainer.VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/init-config/%s", cmName),
			})
		}
		if secretName != "" {
			volumeName := fmt.Sprintf("init-secret-%s", secretName)
			ss.Spec.Template.Spec.Volumes = append(ss.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
					},
				},
			})
			initContainer.VolumeMounts = append(initContainer.VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/init-secrets/%s", secretName),
			})
		}

		ss.Spec.Template.Spec.InitContainers = append(ss.Spec.Template.Spec.InitContainers, initContainer)
	}
}
