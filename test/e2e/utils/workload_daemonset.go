package utils

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// DaemonSetAdapter implements WorkloadAdapter for Kubernetes DaemonSets.
type DaemonSetAdapter struct {
	client kubernetes.Interface
}

// NewDaemonSetAdapter creates a new DaemonSetAdapter.
func NewDaemonSetAdapter(client kubernetes.Interface) *DaemonSetAdapter {
	return &DaemonSetAdapter{client: client}
}

// Type returns the workload type.
func (a *DaemonSetAdapter) Type() WorkloadType {
	return WorkloadDaemonSet
}

// Create creates a DaemonSet with the given config.
func (a *DaemonSetAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildDaemonSetOptions(cfg)
	_, err := CreateDaemonSet(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the DaemonSet.
func (a *DaemonSetAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteDaemonSet(ctx, a.client, namespace, name)
}

// WaitReady waits for the DaemonSet to be ready.
func (a *DaemonSetAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForDaemonSetReady(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the DaemonSet to have the reload annotation.
func (a *DaemonSetAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForDaemonSetReloaded(ctx, a.client, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the DaemonSet to have a STAKATER_ env var.
func (a *DaemonSetAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForDaemonSetEnvVar(ctx, a.client, namespace, name, prefix, timeout)
}

// SupportsEnvVarStrategy returns true as DaemonSets support env var reload strategy.
func (a *DaemonSetAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as DaemonSets use standard rolling restart.
func (a *DaemonSetAdapter) RequiresSpecialHandling() bool {
	return false
}

// buildDaemonSetOptions converts WorkloadConfig to DaemonSetOption slice.
func buildDaemonSetOptions(cfg WorkloadConfig) []DaemonSetOption {
	var opts []DaemonSetOption

	// Add annotations
	if len(cfg.Annotations) > 0 {
		opts = append(opts, WithDaemonSetAnnotations(cfg.Annotations))
	}

	// Add envFrom references
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		opts = append(opts, WithDaemonSetConfigMapEnvFrom(cfg.ConfigMapName))
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		opts = append(opts, WithDaemonSetSecretEnvFrom(cfg.SecretName))
	}

	// Add volume mounts
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		opts = append(opts, WithDaemonSetConfigMapVolume(cfg.ConfigMapName))
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		opts = append(opts, WithDaemonSetSecretVolume(cfg.SecretName))
	}

	// Add projected volume
	if cfg.UseProjectedVolume {
		opts = append(opts, WithDaemonSetProjectedVolume(cfg.ConfigMapName, cfg.SecretName))
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
		opts = append(opts, WithDaemonSetConfigMapKeyRef(cfg.ConfigMapName, key, envVar))
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
		opts = append(opts, WithDaemonSetSecretKeyRef(cfg.SecretName, key, envVar))
	}

	// Add init container with envFrom
	if cfg.UseInitContainer {
		opts = append(opts, WithDaemonSetInitContainer(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add init container with volume mount
	if cfg.UseInitContainerVolume {
		opts = append(opts, WithDaemonSetInitContainerVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	return opts
}

// WithDaemonSetConfigMapVolume adds a volume mount for a ConfigMap to a DaemonSet.
func WithDaemonSetConfigMapVolume(name string) DaemonSetOption {
	return func(ds *appsv1.DaemonSet) {
		volumeName := fmt.Sprintf("cm-%s", name)
		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		})
		ds.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			ds.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/config/%s", name),
			},
		)
	}
}

// WithDaemonSetSecretVolume adds a volume mount for a Secret to a DaemonSet.
func WithDaemonSetSecretVolume(name string) DaemonSetOption {
	return func(ds *appsv1.DaemonSet) {
		volumeName := fmt.Sprintf("secret-%s", name)
		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: name,
				},
			},
		})
		ds.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			ds.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/secrets/%s", name),
			},
		)
	}
}

// WithDaemonSetInitContainer adds an init container that references ConfigMap and/or Secret.
func WithDaemonSetInitContainer(cmName, secretName string) DaemonSetOption {
	return func(ds *appsv1.DaemonSet) {
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

		ds.Spec.Template.Spec.InitContainers = append(ds.Spec.Template.Spec.InitContainers, initContainer)
	}
}

// WithDaemonSetInitContainerVolume adds an init container with ConfigMap/Secret volume mounts.
func WithDaemonSetInitContainerVolume(cmName, secretName string) DaemonSetOption {
	return func(ds *appsv1.DaemonSet) {
		initContainer := corev1.Container{
			Name:    "init",
			Image:   DefaultImage,
			Command: []string{"sh", "-c", "echo init done"},
		}

		if cmName != "" {
			volumeName := fmt.Sprintf("init-cm-%s", cmName)
			ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
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
			ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
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

		ds.Spec.Template.Spec.InitContainers = append(ds.Spec.Template.Spec.InitContainers, initContainer)
	}
}
