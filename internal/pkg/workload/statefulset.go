package workload

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// statefulSetAccessor implements PodTemplateAccessor for StatefulSet.
type statefulSetAccessor struct {
	statefulset *appsv1.StatefulSet
}

func (a *statefulSetAccessor) GetPodTemplateSpec() *corev1.PodTemplateSpec {
	return &a.statefulset.Spec.Template
}

func (a *statefulSetAccessor) GetObjectMeta() *metav1.ObjectMeta {
	return &a.statefulset.ObjectMeta
}

// StatefulSetWorkload wraps a Kubernetes StatefulSet.
type StatefulSetWorkload struct {
	*BaseWorkload[*appsv1.StatefulSet]
}

// NewStatefulSetWorkload creates a new StatefulSetWorkload.
func NewStatefulSetWorkload(s *appsv1.StatefulSet) *StatefulSetWorkload {
	original := s.DeepCopy()
	accessor := &statefulSetAccessor{statefulset: s}
	return &StatefulSetWorkload{
		BaseWorkload: NewBaseWorkload(s, original, accessor, KindStatefulSet),
	}
}

// Ensure StatefulSetWorkload implements Workload.
var _ Workload = (*StatefulSetWorkload)(nil)

func (w *StatefulSetWorkload) DeepCopy() Workload {
	return NewStatefulSetWorkload(w.Object().DeepCopy())
}

// GetStatefulSet returns the underlying StatefulSet for special handling.
func (w *StatefulSetWorkload) GetStatefulSet() *appsv1.StatefulSet {
	return w.Object()
}
