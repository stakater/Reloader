package workload

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CronJobWorkload wraps a Kubernetes CronJob.
// Note: CronJobs have a special update mechanism - instead of updating the CronJob itself,
// Reloader creates a new Job from the CronJob's template.
type CronJobWorkload struct {
	cronjob *batchv1.CronJob
}

// NewCronJobWorkload creates a new CronJobWorkload.
func NewCronJobWorkload(c *batchv1.CronJob) *CronJobWorkload {
	return &CronJobWorkload{cronjob: c}
}

// Ensure CronJobWorkload implements WorkloadAccessor.
var _ WorkloadAccessor = (*CronJobWorkload)(nil)

func (w *CronJobWorkload) Kind() Kind {
	return KindCronJob
}

func (w *CronJobWorkload) GetObject() client.Object {
	return w.cronjob
}

func (w *CronJobWorkload) GetName() string {
	return w.cronjob.Name
}

func (w *CronJobWorkload) GetNamespace() string {
	return w.cronjob.Namespace
}

func (w *CronJobWorkload) GetAnnotations() map[string]string {
	return w.cronjob.Annotations
}

// GetPodTemplateAnnotations returns annotations from the JobTemplate's pod template.
func (w *CronJobWorkload) GetPodTemplateAnnotations() map[string]string {
	if w.cronjob.Spec.JobTemplate.Spec.Template.Annotations == nil {
		w.cronjob.Spec.JobTemplate.Spec.Template.Annotations = make(map[string]string)
	}
	return w.cronjob.Spec.JobTemplate.Spec.Template.Annotations
}

func (w *CronJobWorkload) SetPodTemplateAnnotation(key, value string) {
	if w.cronjob.Spec.JobTemplate.Spec.Template.Annotations == nil {
		w.cronjob.Spec.JobTemplate.Spec.Template.Annotations = make(map[string]string)
	}
	w.cronjob.Spec.JobTemplate.Spec.Template.Annotations[key] = value
}

func (w *CronJobWorkload) GetContainers() []corev1.Container {
	return w.cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers
}

func (w *CronJobWorkload) SetContainers(containers []corev1.Container) {
	w.cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers = containers
}

func (w *CronJobWorkload) GetInitContainers() []corev1.Container {
	return w.cronjob.Spec.JobTemplate.Spec.Template.Spec.InitContainers
}

func (w *CronJobWorkload) SetInitContainers(containers []corev1.Container) {
	w.cronjob.Spec.JobTemplate.Spec.Template.Spec.InitContainers = containers
}

func (w *CronJobWorkload) GetVolumes() []corev1.Volume {
	return w.cronjob.Spec.JobTemplate.Spec.Template.Spec.Volumes
}

// Update for CronJob is a no-op - use CreateJobFromCronJob instead.
// CronJobs trigger reloads by creating a new Job from their template.
func (w *CronJobWorkload) Update(ctx context.Context, c client.Client) error {
	// CronJobs don't get updated directly - a new Job is created instead
	// This is handled by the reload package's special CronJob logic
	return nil
}

func (w *CronJobWorkload) DeepCopy() Workload {
	return &CronJobWorkload{cronjob: w.cronjob.DeepCopy()}
}

func (w *CronJobWorkload) GetEnvFromSources() []corev1.EnvFromSource {
	var sources []corev1.EnvFromSource
	for _, container := range w.cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range w.cronjob.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (w *CronJobWorkload) UsesConfigMap(name string) bool {
	spec := &w.cronjob.Spec.JobTemplate.Spec.Template.Spec

	// Check volumes
	for _, vol := range spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == name {
			return true
		}
		if vol.Projected != nil {
			for _, source := range vol.Projected.Sources {
				if source.ConfigMap != nil && source.ConfigMap.Name == name {
					return true
				}
			}
		}
	}

	// Check containers
	for _, container := range spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == name {
				return true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == name {
				return true
			}
		}
	}

	// Check init containers
	for _, container := range spec.InitContainers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == name {
				return true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == name {
				return true
			}
		}
	}

	return false
}

func (w *CronJobWorkload) UsesSecret(name string) bool {
	spec := &w.cronjob.Spec.JobTemplate.Spec.Template.Spec

	// Check volumes
	for _, vol := range spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == name {
			return true
		}
		if vol.Projected != nil {
			for _, source := range vol.Projected.Sources {
				if source.Secret != nil && source.Secret.Name == name {
					return true
				}
			}
		}
	}

	// Check containers
	for _, container := range spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name == name {
				return true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == name {
				return true
			}
		}
	}

	// Check init containers
	for _, container := range spec.InitContainers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name == name {
				return true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == name {
				return true
			}
		}
	}

	return false
}

func (w *CronJobWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.cronjob.OwnerReferences
}

// GetCronJob returns the underlying CronJob for special handling.
func (w *CronJobWorkload) GetCronJob() *batchv1.CronJob {
	return w.cronjob
}
