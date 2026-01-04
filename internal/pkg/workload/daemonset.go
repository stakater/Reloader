package workload

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// daemonSetAccessor implements PodTemplateAccessor for DaemonSet.
type daemonSetAccessor struct {
	daemonset *appsv1.DaemonSet
}

func (a *daemonSetAccessor) GetPodTemplateSpec() *corev1.PodTemplateSpec {
	return &a.daemonset.Spec.Template
}

func (a *daemonSetAccessor) GetObjectMeta() *metav1.ObjectMeta {
	return &a.daemonset.ObjectMeta
}

// DaemonSetWorkload wraps a Kubernetes DaemonSet.
type DaemonSetWorkload struct {
	*BaseWorkload[*appsv1.DaemonSet]
}

// NewDaemonSetWorkload creates a new DaemonSetWorkload.
func NewDaemonSetWorkload(d *appsv1.DaemonSet) *DaemonSetWorkload {
	original := d.DeepCopy()
	accessor := &daemonSetAccessor{daemonset: d}
	return &DaemonSetWorkload{
		BaseWorkload: NewBaseWorkload(d, original, accessor, KindDaemonSet),
	}
}

// Ensure DaemonSetWorkload implements Workload.
var _ Workload = (*DaemonSetWorkload)(nil)

func (w *DaemonSetWorkload) DeepCopy() Workload {
	return NewDaemonSetWorkload(w.Object().DeepCopy())
}

// GetDaemonSet returns the underlying DaemonSet for special handling.
func (w *DaemonSetWorkload) GetDaemonSet() *appsv1.DaemonSet {
	return w.Object()
}
