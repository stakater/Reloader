package utils

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CronJobAdapter implements WorkloadAdapter for Kubernetes CronJobs.
type CronJobAdapter struct {
	client kubernetes.Interface
}

// NewCronJobAdapter creates a new CronJobAdapter.
func NewCronJobAdapter(client kubernetes.Interface) *CronJobAdapter {
	return &CronJobAdapter{client: client}
}

// Type returns the workload type.
func (a *CronJobAdapter) Type() WorkloadType {
	return WorkloadCronJob
}

// Create creates a CronJob with the given config.
func (a *CronJobAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildCronJobOptions(cfg)
	_, err := CreateCronJob(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the CronJob.
func (a *CronJobAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteCronJob(ctx, a.client, namespace, name)
}

// WaitReady waits for the CronJob to exist (CronJobs are "ready" immediately after creation).
func (a *CronJobAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForCronJobExists(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the CronJob to have the reload annotation OR for a triggered Job.
// For CronJobs, Reloader can either:
// 1. Add an annotation to the pod template
// 2. Trigger a new Job (which is the special handling case)
func (a *CronJobAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForCronJobReloaded(ctx, a.client, namespace, name, annotationKey, timeout)
}

// WaitEnvVar is not supported for CronJobs as they don't use env var reload strategy.
func (a *CronJobAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	// CronJobs don't support env var strategy
	return false, nil
}

// SupportsEnvVarStrategy returns false as CronJobs don't support env var reload strategy.
func (a *CronJobAdapter) SupportsEnvVarStrategy() bool {
	return false
}

// RequiresSpecialHandling returns true as CronJobs use job triggering instead of rolling restart.
func (a *CronJobAdapter) RequiresSpecialHandling() bool {
	return true
}

// WaitForTriggeredJob waits for Reloader to trigger a new Job from this CronJob.
func (a *CronJobAdapter) WaitForTriggeredJob(ctx context.Context, namespace, cronJobName string, timeout time.Duration) (bool, error) {
	return WaitForCronJobTriggeredJob(ctx, a.client, namespace, cronJobName, timeout)
}

// buildCronJobOptions converts WorkloadConfig to CronJobOption slice.
func buildCronJobOptions(cfg WorkloadConfig) []CronJobOption {
	var opts []CronJobOption

	// Add annotations
	if len(cfg.Annotations) > 0 {
		opts = append(opts, WithCronJobAnnotations(cfg.Annotations))
	}

	// Add envFrom references
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		opts = append(opts, WithCronJobConfigMapEnvFrom(cfg.ConfigMapName))
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		opts = append(opts, WithCronJobSecretEnvFrom(cfg.SecretName))
	}

	// Add volume mounts
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		opts = append(opts, WithCronJobConfigMapVolume(cfg.ConfigMapName))
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		opts = append(opts, WithCronJobSecretVolume(cfg.SecretName))
	}

	// Add projected volume
	if cfg.UseProjectedVolume {
		opts = append(opts, WithCronJobProjectedVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	return opts
}

// WithCronJobConfigMapVolume adds a volume mount for a ConfigMap to a CronJob.
func WithCronJobConfigMapVolume(name string) CronJobOption {
	return func(cj *batchv1.CronJob) {
		volumeName := "cm-" + name
		cj.Spec.JobTemplate.Spec.Template.Spec.Volumes = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: name},
					},
				},
			},
		)
		cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/etc/config/" + name,
			},
		)
	}
}

// WithCronJobSecretVolume adds a volume mount for a Secret to a CronJob.
func WithCronJobSecretVolume(name string) CronJobOption {
	return func(cj *batchv1.CronJob) {
		volumeName := "secret-" + name
		cj.Spec.JobTemplate.Spec.Template.Spec.Volumes = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: name,
					},
				},
			},
		)
		cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/etc/secrets/" + name,
			},
		)
	}
}

// WithCronJobProjectedVolume adds a projected volume with ConfigMap and/or Secret sources to a CronJob.
func WithCronJobProjectedVolume(cmName, secretName string) CronJobOption {
	return func(cj *batchv1.CronJob) {
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

		cj.Spec.JobTemplate.Spec.Template.Spec.Volumes = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: sources,
					},
				},
			},
		)
		cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/etc/projected",
			},
		)
	}
}

// WaitForCronJobEnvVar waits for a CronJob's containers to have an environment variable
// with the given prefix. Note: CronJobs don't typically use this strategy.
func WaitForCronJobEnvVar(ctx context.Context, client kubernetes.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		cj, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if hasEnvVarWithPrefix(cj.Spec.JobTemplate.Spec.Template.Spec.Containers, prefix) {
			found = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}
