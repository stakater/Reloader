package workload

import (
	"context"

	openshiftv1 "github.com/openshift/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentConfigWorkload wraps an OpenShift DeploymentConfig.
type DeploymentConfigWorkload struct {
	dc       *openshiftv1.DeploymentConfig
	original *openshiftv1.DeploymentConfig
}

// NewDeploymentConfigWorkload creates a new DeploymentConfigWorkload.
func NewDeploymentConfigWorkload(dc *openshiftv1.DeploymentConfig) *DeploymentConfigWorkload {
	return &DeploymentConfigWorkload{
		dc:       dc,
		original: dc.DeepCopy(),
	}
}

// Ensure DeploymentConfigWorkload implements WorkloadAccessor.
var _ WorkloadAccessor = (*DeploymentConfigWorkload)(nil)

func (w *DeploymentConfigWorkload) Kind() Kind {
	return KindDeploymentConfig
}

func (w *DeploymentConfigWorkload) GetObject() client.Object {
	return w.dc
}

func (w *DeploymentConfigWorkload) GetName() string {
	return w.dc.Name
}

func (w *DeploymentConfigWorkload) GetNamespace() string {
	return w.dc.Namespace
}

func (w *DeploymentConfigWorkload) GetAnnotations() map[string]string {
	return w.dc.Annotations
}

func (w *DeploymentConfigWorkload) GetPodTemplateAnnotations() map[string]string {
	if w.dc.Spec.Template == nil {
		return nil
	}
	if w.dc.Spec.Template.Annotations == nil {
		w.dc.Spec.Template.Annotations = make(map[string]string)
	}
	return w.dc.Spec.Template.Annotations
}

func (w *DeploymentConfigWorkload) SetPodTemplateAnnotation(key, value string) {
	if w.dc.Spec.Template == nil {
		w.dc.Spec.Template = &corev1.PodTemplateSpec{}
	}
	if w.dc.Spec.Template.Annotations == nil {
		w.dc.Spec.Template.Annotations = make(map[string]string)
	}
	w.dc.Spec.Template.Annotations[key] = value
}

func (w *DeploymentConfigWorkload) GetContainers() []corev1.Container {
	if w.dc.Spec.Template == nil {
		return nil
	}
	return w.dc.Spec.Template.Spec.Containers
}

func (w *DeploymentConfigWorkload) SetContainers(containers []corev1.Container) {
	if w.dc.Spec.Template == nil {
		w.dc.Spec.Template = &corev1.PodTemplateSpec{}
	}
	w.dc.Spec.Template.Spec.Containers = containers
}

func (w *DeploymentConfigWorkload) GetInitContainers() []corev1.Container {
	if w.dc.Spec.Template == nil {
		return nil
	}
	return w.dc.Spec.Template.Spec.InitContainers
}

func (w *DeploymentConfigWorkload) SetInitContainers(containers []corev1.Container) {
	if w.dc.Spec.Template == nil {
		w.dc.Spec.Template = &corev1.PodTemplateSpec{}
	}
	w.dc.Spec.Template.Spec.InitContainers = containers
}

func (w *DeploymentConfigWorkload) GetVolumes() []corev1.Volume {
	if w.dc.Spec.Template == nil {
		return nil
	}
	return w.dc.Spec.Template.Spec.Volumes
}

func (w *DeploymentConfigWorkload) Update(ctx context.Context, c client.Client) error {
	return c.Patch(ctx, w.dc, client.StrategicMergeFrom(w.original), client.FieldOwner(FieldManager))
}

func (w *DeploymentConfigWorkload) DeepCopy() Workload {
	return &DeploymentConfigWorkload{
		dc:       w.dc.DeepCopy(),
		original: w.original.DeepCopy(),
	}
}

func (w *DeploymentConfigWorkload) ResetOriginal() {
	w.original = w.dc.DeepCopy()
}

func (w *DeploymentConfigWorkload) GetEnvFromSources() []corev1.EnvFromSource {
	if w.dc.Spec.Template == nil {
		return nil
	}
	var sources []corev1.EnvFromSource
	for _, container := range w.dc.Spec.Template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range w.dc.Spec.Template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (w *DeploymentConfigWorkload) UsesConfigMap(name string) bool {
	if w.dc.Spec.Template == nil {
		return false
	}
	return SpecUsesConfigMap(&w.dc.Spec.Template.Spec, name)
}

func (w *DeploymentConfigWorkload) UsesSecret(name string) bool {
	if w.dc.Spec.Template == nil {
		return false
	}
	return SpecUsesSecret(&w.dc.Spec.Template.Spec, name)
}

func (w *DeploymentConfigWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.dc.OwnerReferences
}

// GetDeploymentConfig returns the underlying DeploymentConfig for special handling.
func (w *DeploymentConfigWorkload) GetDeploymentConfig() *openshiftv1.DeploymentConfig {
	return w.dc
}
