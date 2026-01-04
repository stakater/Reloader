package workload

import (
	openshiftv1 "github.com/openshift/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// deploymentConfigAccessor implements PodTemplateAccessor for DeploymentConfig.
type deploymentConfigAccessor struct {
	dc *openshiftv1.DeploymentConfig
}

func (a *deploymentConfigAccessor) GetPodTemplateSpec() *corev1.PodTemplateSpec {
	// DeploymentConfig has a pointer to PodTemplateSpec which may be nil
	return a.dc.Spec.Template
}

func (a *deploymentConfigAccessor) GetObjectMeta() *metav1.ObjectMeta {
	return &a.dc.ObjectMeta
}

// DeploymentConfigWorkload wraps an OpenShift DeploymentConfig.
type DeploymentConfigWorkload struct {
	*BaseWorkload[*openshiftv1.DeploymentConfig]
}

// NewDeploymentConfigWorkload creates a new DeploymentConfigWorkload.
func NewDeploymentConfigWorkload(dc *openshiftv1.DeploymentConfig) *DeploymentConfigWorkload {
	original := dc.DeepCopy()
	accessor := &deploymentConfigAccessor{dc: dc}
	return &DeploymentConfigWorkload{
		BaseWorkload: NewBaseWorkload(dc, original, accessor, KindDeploymentConfig),
	}
}

// Ensure DeploymentConfigWorkload implements Workload.
var _ Workload = (*DeploymentConfigWorkload)(nil)

// SetPodTemplateAnnotation overrides the base to ensure Template is initialized.
func (w *DeploymentConfigWorkload) SetPodTemplateAnnotation(key, value string) {
	dc := w.Object()
	if dc.Spec.Template == nil {
		dc.Spec.Template = &corev1.PodTemplateSpec{}
	}
	if dc.Spec.Template.Annotations == nil {
		dc.Spec.Template.Annotations = make(map[string]string)
	}
	dc.Spec.Template.Annotations[key] = value
}

// SetContainers overrides the base to ensure Template is initialized.
func (w *DeploymentConfigWorkload) SetContainers(containers []corev1.Container) {
	dc := w.Object()
	if dc.Spec.Template == nil {
		dc.Spec.Template = &corev1.PodTemplateSpec{}
	}
	dc.Spec.Template.Spec.Containers = containers
}

// SetInitContainers overrides the base to ensure Template is initialized.
func (w *DeploymentConfigWorkload) SetInitContainers(containers []corev1.Container) {
	dc := w.Object()
	if dc.Spec.Template == nil {
		dc.Spec.Template = &corev1.PodTemplateSpec{}
	}
	dc.Spec.Template.Spec.InitContainers = containers
}

func (w *DeploymentConfigWorkload) DeepCopy() Workload {
	return NewDeploymentConfigWorkload(w.Object().DeepCopy())
}

// GetDeploymentConfig returns the underlying DeploymentConfig for special handling.
func (w *DeploymentConfigWorkload) GetDeploymentConfig() *openshiftv1.DeploymentConfig {
	return w.Object()
}
