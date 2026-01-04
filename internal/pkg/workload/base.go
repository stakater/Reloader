package workload

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodTemplateAccessor provides access to a workload's pod template.
// Each workload type implements this to provide access to its specific template location.
type PodTemplateAccessor interface {
	// GetPodTemplateSpec returns a pointer to the pod template spec.
	// Returns nil if the workload doesn't have a pod template
	GetPodTemplateSpec() *corev1.PodTemplateSpec

	// GetObjectMeta returns the workload's object metadata.
	GetObjectMeta() *metav1.ObjectMeta
}

// BaseWorkload provides common functionality for all workload types.
// It uses composition with a PodTemplateAccessor to access type-specific fields.
type BaseWorkload[T client.Object] struct {
	object   T
	original T
	accessor PodTemplateAccessor
	kind     Kind
}

// NewBaseWorkload creates a new BaseWorkload with the given object and accessor.
func NewBaseWorkload[T client.Object](obj T, original T, accessor PodTemplateAccessor, kind Kind) *BaseWorkload[T] {
	return &BaseWorkload[T]{
		object:   obj,
		original: original,
		accessor: accessor,
		kind:     kind,
	}
}

func (b *BaseWorkload[T]) Kind() Kind {
	return b.kind
}

func (b *BaseWorkload[T]) GetObject() client.Object {
	return b.object
}

func (b *BaseWorkload[T]) GetName() string {
	return b.accessor.GetObjectMeta().Name
}

func (b *BaseWorkload[T]) GetNamespace() string {
	return b.accessor.GetObjectMeta().Namespace
}

func (b *BaseWorkload[T]) GetAnnotations() map[string]string {
	return b.accessor.GetObjectMeta().Annotations
}

func (b *BaseWorkload[T]) GetPodTemplateAnnotations() map[string]string {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return nil
	}
	if template.Annotations == nil {
		template.Annotations = make(map[string]string)
	}
	return template.Annotations
}

func (b *BaseWorkload[T]) SetPodTemplateAnnotation(key, value string) {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return
	}
	if template.Annotations == nil {
		template.Annotations = make(map[string]string)
	}
	template.Annotations[key] = value
}

func (b *BaseWorkload[T]) GetContainers() []corev1.Container {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return nil
	}
	return template.Spec.Containers
}

func (b *BaseWorkload[T]) SetContainers(containers []corev1.Container) {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return
	}
	template.Spec.Containers = containers
}

func (b *BaseWorkload[T]) GetInitContainers() []corev1.Container {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return nil
	}
	return template.Spec.InitContainers
}

func (b *BaseWorkload[T]) SetInitContainers(containers []corev1.Container) {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return
	}
	template.Spec.InitContainers = containers
}

func (b *BaseWorkload[T]) GetVolumes() []corev1.Volume {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return nil
	}
	return template.Spec.Volumes
}

func (b *BaseWorkload[T]) GetEnvFromSources() []corev1.EnvFromSource {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return nil
	}
	var sources []corev1.EnvFromSource
	for _, container := range template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (b *BaseWorkload[T]) UsesConfigMap(name string) bool {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return false
	}
	return SpecUsesConfigMap(&template.Spec, name)
}

func (b *BaseWorkload[T]) UsesSecret(name string) bool {
	template := b.accessor.GetPodTemplateSpec()
	if template == nil {
		return false
	}
	return SpecUsesSecret(&template.Spec, name)
}

func (b *BaseWorkload[T]) GetOwnerReferences() []metav1.OwnerReference {
	return b.accessor.GetObjectMeta().OwnerReferences
}

// Update performs a strategic merge patch update.
func (b *BaseWorkload[T]) Update(ctx context.Context, c client.Client) error {
	return c.Patch(ctx, b.object, client.StrategicMergeFrom(b.original), client.FieldOwner(FieldManager))
}

// ResetOriginal resets the original state to the current object state.
func (b *BaseWorkload[T]) ResetOriginal() {
	b.original = b.object.DeepCopyObject().(T)
}

// UpdateStrategy returns the default patch strategy.
// Workloads with special update logic should override this.
func (b *BaseWorkload[T]) UpdateStrategy() UpdateStrategy {
	return UpdateStrategyPatch
}

// PerformSpecialUpdate returns false for standard workloads.
// Workloads with special update logic should override this.
func (b *BaseWorkload[T]) PerformSpecialUpdate(ctx context.Context, c client.Client) (bool, error) {
	return false, nil
}

// Object returns the underlying Kubernetes object.
func (b *BaseWorkload[T]) Object() T {
	return b.object
}

// Original returns the original state of the object.
func (b *BaseWorkload[T]) Original() T {
	return b.original
}
