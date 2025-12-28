package workload

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// JobWorkload wraps a Kubernetes Job.
// Note: Jobs have a special update mechanism - instead of updating the Job,
// Reloader deletes and recreates it with the same spec.
type JobWorkload struct {
	job *batchv1.Job
}

// NewJobWorkload creates a new JobWorkload.
func NewJobWorkload(j *batchv1.Job) *JobWorkload {
	return &JobWorkload{job: j}
}

// Ensure JobWorkload implements WorkloadAccessor.
var _ WorkloadAccessor = (*JobWorkload)(nil)

func (w *JobWorkload) Kind() Kind {
	return KindJob
}

func (w *JobWorkload) GetObject() client.Object {
	return w.job
}

func (w *JobWorkload) GetName() string {
	return w.job.Name
}

func (w *JobWorkload) GetNamespace() string {
	return w.job.Namespace
}

func (w *JobWorkload) GetAnnotations() map[string]string {
	return w.job.Annotations
}

func (w *JobWorkload) GetPodTemplateAnnotations() map[string]string {
	if w.job.Spec.Template.Annotations == nil {
		w.job.Spec.Template.Annotations = make(map[string]string)
	}
	return w.job.Spec.Template.Annotations
}

func (w *JobWorkload) SetPodTemplateAnnotation(key, value string) {
	if w.job.Spec.Template.Annotations == nil {
		w.job.Spec.Template.Annotations = make(map[string]string)
	}
	w.job.Spec.Template.Annotations[key] = value
}

func (w *JobWorkload) GetContainers() []corev1.Container {
	return w.job.Spec.Template.Spec.Containers
}

func (w *JobWorkload) SetContainers(containers []corev1.Container) {
	w.job.Spec.Template.Spec.Containers = containers
}

func (w *JobWorkload) GetInitContainers() []corev1.Container {
	return w.job.Spec.Template.Spec.InitContainers
}

func (w *JobWorkload) SetInitContainers(containers []corev1.Container) {
	w.job.Spec.Template.Spec.InitContainers = containers
}

func (w *JobWorkload) GetVolumes() []corev1.Volume {
	return w.job.Spec.Template.Spec.Volumes
}

// Update for Job is a no-op - use RecreateJob instead.
// Jobs trigger reloads by being deleted and recreated.
func (w *JobWorkload) Update(ctx context.Context, c client.Client) error {
	// Jobs don't get updated directly - they are deleted and recreated
	// This is handled by the reload package's special Job logic
	return nil
}

func (w *JobWorkload) DeepCopy() Workload {
	return &JobWorkload{job: w.job.DeepCopy()}
}

func (w *JobWorkload) GetEnvFromSources() []corev1.EnvFromSource {
	var sources []corev1.EnvFromSource
	for _, container := range w.job.Spec.Template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range w.job.Spec.Template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (w *JobWorkload) UsesConfigMap(name string) bool {
	// Check volumes
	for _, vol := range w.job.Spec.Template.Spec.Volumes {
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
	for _, container := range w.job.Spec.Template.Spec.Containers {
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
	for _, container := range w.job.Spec.Template.Spec.InitContainers {
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

func (w *JobWorkload) UsesSecret(name string) bool {
	// Check volumes
	for _, vol := range w.job.Spec.Template.Spec.Volumes {
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
	for _, container := range w.job.Spec.Template.Spec.Containers {
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
	for _, container := range w.job.Spec.Template.Spec.InitContainers {
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

func (w *JobWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.job.OwnerReferences
}

// GetJob returns the underlying Job for special handling.
func (w *JobWorkload) GetJob() *batchv1.Job {
	return w.job
}
