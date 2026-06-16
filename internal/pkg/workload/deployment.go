package workload

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// deploymentAccessor implements PodTemplateAccessor for Deployment.
type deploymentAccessor struct {
	deployment *appsv1.Deployment
}

func (a *deploymentAccessor) GetPodTemplateSpec() *corev1.PodTemplateSpec {
	return &a.deployment.Spec.Template
}

func (a *deploymentAccessor) GetObjectMeta() *metav1.ObjectMeta {
	return &a.deployment.ObjectMeta
}

// DeploymentWorkload wraps a Kubernetes Deployment.
type DeploymentWorkload struct {
	*BaseWorkload[*appsv1.Deployment]
}

// NewDeploymentWorkload creates a new DeploymentWorkload.
func NewDeploymentWorkload(d *appsv1.Deployment) *DeploymentWorkload {
	original := d.DeepCopy()
	accessor := &deploymentAccessor{deployment: d}
	return &DeploymentWorkload{
		BaseWorkload: NewBaseWorkload(d, original, accessor, KindDeployment),
	}
}

// Ensure DeploymentWorkload implements Workload.
var _ Workload = (*DeploymentWorkload)(nil)

func (w *DeploymentWorkload) DeepCopy() Workload {
	return NewDeploymentWorkload(w.Object().DeepCopy())
}

// GetDeployment returns the underlying Deployment for special handling.
func (w *DeploymentWorkload) GetDeployment() *appsv1.Deployment {
	return w.Object()
}
