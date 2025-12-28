package workload

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatefulSetWorkload wraps a Kubernetes StatefulSet.
type StatefulSetWorkload struct {
	statefulset *appsv1.StatefulSet
}

// NewStatefulSetWorkload creates a new StatefulSetWorkload.
func NewStatefulSetWorkload(s *appsv1.StatefulSet) *StatefulSetWorkload {
	return &StatefulSetWorkload{statefulset: s}
}

// Ensure StatefulSetWorkload implements WorkloadAccessor.
var _ WorkloadAccessor = (*StatefulSetWorkload)(nil)

func (w *StatefulSetWorkload) Kind() Kind {
	return KindStatefulSet
}

func (w *StatefulSetWorkload) GetObject() client.Object {
	return w.statefulset
}

func (w *StatefulSetWorkload) GetName() string {
	return w.statefulset.Name
}

func (w *StatefulSetWorkload) GetNamespace() string {
	return w.statefulset.Namespace
}

func (w *StatefulSetWorkload) GetAnnotations() map[string]string {
	return w.statefulset.Annotations
}

func (w *StatefulSetWorkload) GetPodTemplateAnnotations() map[string]string {
	if w.statefulset.Spec.Template.Annotations == nil {
		w.statefulset.Spec.Template.Annotations = make(map[string]string)
	}
	return w.statefulset.Spec.Template.Annotations
}

func (w *StatefulSetWorkload) SetPodTemplateAnnotation(key, value string) {
	if w.statefulset.Spec.Template.Annotations == nil {
		w.statefulset.Spec.Template.Annotations = make(map[string]string)
	}
	w.statefulset.Spec.Template.Annotations[key] = value
}

func (w *StatefulSetWorkload) GetContainers() []corev1.Container {
	return w.statefulset.Spec.Template.Spec.Containers
}

func (w *StatefulSetWorkload) SetContainers(containers []corev1.Container) {
	w.statefulset.Spec.Template.Spec.Containers = containers
}

func (w *StatefulSetWorkload) GetInitContainers() []corev1.Container {
	return w.statefulset.Spec.Template.Spec.InitContainers
}

func (w *StatefulSetWorkload) SetInitContainers(containers []corev1.Container) {
	w.statefulset.Spec.Template.Spec.InitContainers = containers
}

func (w *StatefulSetWorkload) GetVolumes() []corev1.Volume {
	return w.statefulset.Spec.Template.Spec.Volumes
}

func (w *StatefulSetWorkload) Update(ctx context.Context, c client.Client) error {
	return c.Update(ctx, w.statefulset)
}

func (w *StatefulSetWorkload) DeepCopy() Workload {
	return &StatefulSetWorkload{statefulset: w.statefulset.DeepCopy()}
}

func (w *StatefulSetWorkload) GetEnvFromSources() []corev1.EnvFromSource {
	var sources []corev1.EnvFromSource
	for _, container := range w.statefulset.Spec.Template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range w.statefulset.Spec.Template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (w *StatefulSetWorkload) UsesConfigMap(name string) bool {
	return SpecUsesConfigMap(&w.statefulset.Spec.Template.Spec, name)
}

func (w *StatefulSetWorkload) UsesSecret(name string) bool {
	return SpecUsesSecret(&w.statefulset.Spec.Template.Spec, name)
}

func (w *StatefulSetWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.statefulset.OwnerReferences
}
