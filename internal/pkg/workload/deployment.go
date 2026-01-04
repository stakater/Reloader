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
	original   *appsv1.Deployment
}

// NewDeploymentWorkload creates a new DeploymentWorkload.
func NewDeploymentWorkload(d *appsv1.Deployment) *DeploymentWorkload {
	return &DeploymentWorkload{
		deployment: d,
		original:   d.DeepCopy(),
	}
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
	return c.Patch(ctx, w.deployment, client.StrategicMergeFrom(w.original), client.FieldOwner(FieldManager))
}

func (w *DeploymentWorkload) DeepCopy() Workload {
	return &DeploymentWorkload{
		deployment: w.deployment.DeepCopy(),
		original:   w.original.DeepCopy(),
	}
}

func (w *DeploymentWorkload) ResetOriginal() {
	w.original = w.deployment.DeepCopy()
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
	return SpecUsesConfigMap(&w.deployment.Spec.Template.Spec, name)
}

func (w *DeploymentWorkload) UsesSecret(name string) bool {
	return SpecUsesSecret(&w.deployment.Spec.Template.Spec, name)
}

func (w *DeploymentWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.deployment.OwnerReferences
}

// GetDeployment returns the underlying Deployment for special handling.
func (w *DeploymentWorkload) GetDeployment() *appsv1.Deployment {
	return w.deployment
}
