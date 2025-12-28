package workload

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentWorkload wraps a Kubernetes Deployment.
type DeploymentWorkload struct {
	deployment *appsv1.Deployment
}

// NewDeploymentWorkload creates a new DeploymentWorkload.
func NewDeploymentWorkload(d *appsv1.Deployment) *DeploymentWorkload {
	return &DeploymentWorkload{deployment: d}
}

// Ensure DeploymentWorkload implements WorkloadAccessor.
var _ WorkloadAccessor = (*DeploymentWorkload)(nil)

func (w *DeploymentWorkload) Kind() Kind {
	return KindDeployment
}

func (w *DeploymentWorkload) GetObject() client.Object {
	return w.deployment
}

func (w *DeploymentWorkload) GetName() string {
	return w.deployment.Name
}

func (w *DeploymentWorkload) GetNamespace() string {
	return w.deployment.Namespace
}

func (w *DeploymentWorkload) GetAnnotations() map[string]string {
	return w.deployment.Annotations
}

func (w *DeploymentWorkload) GetPodTemplateAnnotations() map[string]string {
	if w.deployment.Spec.Template.Annotations == nil {
		w.deployment.Spec.Template.Annotations = make(map[string]string)
	}
	return w.deployment.Spec.Template.Annotations
}

func (w *DeploymentWorkload) SetPodTemplateAnnotation(key, value string) {
	if w.deployment.Spec.Template.Annotations == nil {
		w.deployment.Spec.Template.Annotations = make(map[string]string)
	}
	w.deployment.Spec.Template.Annotations[key] = value
}

func (w *DeploymentWorkload) GetContainers() []corev1.Container {
	return w.deployment.Spec.Template.Spec.Containers
}

func (w *DeploymentWorkload) SetContainers(containers []corev1.Container) {
	w.deployment.Spec.Template.Spec.Containers = containers
}

func (w *DeploymentWorkload) GetInitContainers() []corev1.Container {
	return w.deployment.Spec.Template.Spec.InitContainers
}

func (w *DeploymentWorkload) SetInitContainers(containers []corev1.Container) {
	w.deployment.Spec.Template.Spec.InitContainers = containers
}

func (w *DeploymentWorkload) GetVolumes() []corev1.Volume {
	return w.deployment.Spec.Template.Spec.Volumes
}

func (w *DeploymentWorkload) Update(ctx context.Context, c client.Client) error {
	return c.Update(ctx, w.deployment)
}

func (w *DeploymentWorkload) DeepCopy() Workload {
	return &DeploymentWorkload{deployment: w.deployment.DeepCopy()}
}

func (w *DeploymentWorkload) GetEnvFromSources() []corev1.EnvFromSource {
	var sources []corev1.EnvFromSource
	for _, container := range w.deployment.Spec.Template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range w.deployment.Spec.Template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (w *DeploymentWorkload) UsesConfigMap(name string) bool {
	// Check volumes
	for _, vol := range w.deployment.Spec.Template.Spec.Volumes {
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
	for _, container := range w.deployment.Spec.Template.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == name {
				return true
			}
		}
		// Check individual env vars
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == name {
				return true
			}
		}
	}

	// Check init containers
	for _, container := range w.deployment.Spec.Template.Spec.InitContainers {
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

func (w *DeploymentWorkload) UsesSecret(name string) bool {
	// Check volumes
	for _, vol := range w.deployment.Spec.Template.Spec.Volumes {
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
	for _, container := range w.deployment.Spec.Template.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name == name {
				return true
			}
		}
		// Check individual env vars
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == name {
				return true
			}
		}
	}

	// Check init containers
	for _, container := range w.deployment.Spec.Template.Spec.InitContainers {
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

func (w *DeploymentWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.deployment.OwnerReferences
}
