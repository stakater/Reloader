package utils

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// JobAdapter implements WorkloadAdapter for Kubernetes Jobs.
// Note: Jobs are handled specially by Reloader - they are recreated rather than updated.
type JobAdapter struct {
	client kubernetes.Interface
}

// NewJobAdapter creates a new JobAdapter.
func NewJobAdapter(client kubernetes.Interface) *JobAdapter {
	return &JobAdapter{client: client}
}

// Type returns the workload type.
func (a *JobAdapter) Type() WorkloadType {
	return WorkloadJob
}

// Create creates a Job with the given config.
func (a *JobAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildJobOptions(cfg)
	_, err := CreateJob(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the Job.
func (a *JobAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteJob(ctx, a.client, namespace, name)
}

// WaitReady waits for the Job to exist.
func (a *JobAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForJobExists(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the Job to be recreated (new UID).
// For Jobs, Reloader recreates the Job rather than updating annotations.
func (a *JobAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	// For Jobs, we check if it was recreated by looking for a new UID
	// This requires storing the original UID before the test
	// For simplicity, we use the same pattern as other workloads
	// The test should verify recreation using WaitForJobRecreated instead
	return false, nil
}

// WaitEnvVar is not supported for Jobs as they don't use env var reload strategy.
func (a *JobAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return false, nil
}

// SupportsEnvVarStrategy returns false as Jobs don't support env var reload strategy.
func (a *JobAdapter) SupportsEnvVarStrategy() bool {
	return false
}

// RequiresSpecialHandling returns true as Jobs are recreated by Reloader.
func (a *JobAdapter) RequiresSpecialHandling() bool {
	return true
}

// GetOriginalUID retrieves the current UID of the Job for recreation verification.
func (a *JobAdapter) GetOriginalUID(ctx context.Context, namespace, name string) (string, error) {
	job, err := GetJob(ctx, a.client, namespace, name)
	if err != nil {
		return "", err
	}
	return string(job.UID), nil
}

// WaitForRecreation waits for the Job to be recreated with a new UID.
func (a *JobAdapter) WaitForRecreation(ctx context.Context, namespace, name, originalUID string, timeout time.Duration) (string, bool, error) {
	return WaitForJobRecreated(ctx, a.client, namespace, name, originalUID, timeout)
}

// buildJobOptions converts WorkloadConfig to JobOption slice.
func buildJobOptions(cfg WorkloadConfig) []JobOption {
	var opts []JobOption

	// Add annotations
	if len(cfg.Annotations) > 0 {
		opts = append(opts, WithJobAnnotations(cfg.Annotations))
	}

	// Add envFrom references
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		opts = append(opts, WithJobConfigMapEnvFrom(cfg.ConfigMapName))
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		opts = append(opts, WithJobSecretEnvFrom(cfg.SecretName))
	}

	// Add volume mounts
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		opts = append(opts, WithJobConfigMapVolume(cfg.ConfigMapName))
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		opts = append(opts, WithJobSecretVolume(cfg.SecretName))
	}

	// Add projected volume
	if cfg.UseProjectedVolume {
		opts = append(opts, WithJobProjectedVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	return opts
}

// WithJobConfigMapVolume adds a volume mount for a ConfigMap to a Job.
func WithJobConfigMapVolume(name string) JobOption {
	return func(j *batchv1.Job) {
		volumeName := "cm-" + name
		j.Spec.Template.Spec.Volumes = append(
			j.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: name},
					},
				},
			},
		)
		j.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			j.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/etc/config/" + name,
			},
		)
	}
}

// WithJobSecretVolume adds a volume mount for a Secret to a Job.
func WithJobSecretVolume(name string) JobOption {
	return func(j *batchv1.Job) {
		volumeName := "secret-" + name
		j.Spec.Template.Spec.Volumes = append(
			j.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: name,
					},
				},
			},
		)
		j.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			j.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/etc/secrets/" + name,
			},
		)
	}
}

// WithJobProjectedVolume adds a projected volume with ConfigMap and/or Secret sources to a Job.
func WithJobProjectedVolume(cmName, secretName string) JobOption {
	return func(j *batchv1.Job) {
		volumeName := "projected-config"
		sources := []corev1.VolumeProjection{}

		if cmName != "" {
			sources = append(sources, corev1.VolumeProjection{
				ConfigMap: &corev1.ConfigMapProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
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

		j.Spec.Template.Spec.Volumes = append(
			j.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: sources,
					},
				},
			},
		)
		j.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			j.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/etc/projected",
			},
		)
	}
}
