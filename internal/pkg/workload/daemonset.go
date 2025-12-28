package workload

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DaemonSetWorkload wraps a Kubernetes DaemonSet.
type DaemonSetWorkload struct {
	daemonset *appsv1.DaemonSet
}

// NewDaemonSetWorkload creates a new DaemonSetWorkload.
func NewDaemonSetWorkload(d *appsv1.DaemonSet) *DaemonSetWorkload {
	return &DaemonSetWorkload{daemonset: d}
}

// Ensure DaemonSetWorkload implements WorkloadAccessor.
var _ WorkloadAccessor = (*DaemonSetWorkload)(nil)

func (w *DaemonSetWorkload) Kind() Kind {
	return KindDaemonSet
}

func (w *DaemonSetWorkload) GetObject() client.Object {
	return w.daemonset
}

func (w *DaemonSetWorkload) GetName() string {
	return w.daemonset.Name
}

func (w *DaemonSetWorkload) GetNamespace() string {
	return w.daemonset.Namespace
}

func (w *DaemonSetWorkload) GetAnnotations() map[string]string {
	return w.daemonset.Annotations
}

func (w *DaemonSetWorkload) GetPodTemplateAnnotations() map[string]string {
	if w.daemonset.Spec.Template.Annotations == nil {
		w.daemonset.Spec.Template.Annotations = make(map[string]string)
	}
	return w.daemonset.Spec.Template.Annotations
}

func (w *DaemonSetWorkload) SetPodTemplateAnnotation(key, value string) {
	if w.daemonset.Spec.Template.Annotations == nil {
		w.daemonset.Spec.Template.Annotations = make(map[string]string)
	}
	w.daemonset.Spec.Template.Annotations[key] = value
}

func (w *DaemonSetWorkload) GetContainers() []corev1.Container {
	return w.daemonset.Spec.Template.Spec.Containers
}

func (w *DaemonSetWorkload) SetContainers(containers []corev1.Container) {
	w.daemonset.Spec.Template.Spec.Containers = containers
}

func (w *DaemonSetWorkload) GetInitContainers() []corev1.Container {
	return w.daemonset.Spec.Template.Spec.InitContainers
}

func (w *DaemonSetWorkload) SetInitContainers(containers []corev1.Container) {
	w.daemonset.Spec.Template.Spec.InitContainers = containers
}

func (w *DaemonSetWorkload) GetVolumes() []corev1.Volume {
	return w.daemonset.Spec.Template.Spec.Volumes
}

func (w *DaemonSetWorkload) Update(ctx context.Context, c client.Client) error {
	return c.Update(ctx, w.daemonset)
}

func (w *DaemonSetWorkload) DeepCopy() Workload {
	return &DaemonSetWorkload{daemonset: w.daemonset.DeepCopy()}
}

func (w *DaemonSetWorkload) GetEnvFromSources() []corev1.EnvFromSource {
	var sources []corev1.EnvFromSource
	for _, container := range w.daemonset.Spec.Template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range w.daemonset.Spec.Template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (w *DaemonSetWorkload) UsesConfigMap(name string) bool {
	// Check volumes
	for _, vol := range w.daemonset.Spec.Template.Spec.Volumes {
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

	// Check envFrom
	for _, container := range w.daemonset.Spec.Template.Spec.Containers {
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
	for _, container := range w.daemonset.Spec.Template.Spec.InitContainers {
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

func (w *DaemonSetWorkload) UsesSecret(name string) bool {
	// Check volumes
	for _, vol := range w.daemonset.Spec.Template.Spec.Volumes {
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

	// Check envFrom
	for _, container := range w.daemonset.Spec.Template.Spec.Containers {
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
	for _, container := range w.daemonset.Spec.Template.Spec.InitContainers {
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

func (w *DaemonSetWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.daemonset.OwnerReferences
}
