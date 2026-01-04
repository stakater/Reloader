package workload

import (
	"context"
	"fmt"
	"time"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RolloutStrategy defines how Argo Rollouts are updated.
type RolloutStrategy string

const (
	// RolloutStrategyRollout performs a standard rollout update.
	RolloutStrategyRollout RolloutStrategy = "rollout"

	// RolloutStrategyRestart sets the restartAt field to trigger a restart.
	RolloutStrategyRestart RolloutStrategy = "restart"
)

// RolloutStrategyAnnotation is the annotation key for specifying the rollout strategy.
const RolloutStrategyAnnotation = "reloader.stakater.com/rollout-strategy"

// RolloutWorkload wraps an Argo Rollout.
type RolloutWorkload struct {
	rollout  *argorolloutv1alpha1.Rollout
	original *argorolloutv1alpha1.Rollout
}

// NewRolloutWorkload creates a new RolloutWorkload.
func NewRolloutWorkload(r *argorolloutv1alpha1.Rollout) *RolloutWorkload {
	return &RolloutWorkload{
		rollout:  r,
		original: r.DeepCopy(),
	}
}

// Ensure RolloutWorkload implements WorkloadAccessor.
var _ WorkloadAccessor = (*RolloutWorkload)(nil)

func (w *RolloutWorkload) Kind() Kind {
	return KindArgoRollout
}

func (w *RolloutWorkload) GetObject() client.Object {
	return w.rollout
}

func (w *RolloutWorkload) GetName() string {
	return w.rollout.Name
}

func (w *RolloutWorkload) GetNamespace() string {
	return w.rollout.Namespace
}

func (w *RolloutWorkload) GetAnnotations() map[string]string {
	return w.rollout.Annotations
}

func (w *RolloutWorkload) GetPodTemplateAnnotations() map[string]string {
	if w.rollout.Spec.Template.Annotations == nil {
		w.rollout.Spec.Template.Annotations = make(map[string]string)
	}
	return w.rollout.Spec.Template.Annotations
}

func (w *RolloutWorkload) SetPodTemplateAnnotation(key, value string) {
	if w.rollout.Spec.Template.Annotations == nil {
		w.rollout.Spec.Template.Annotations = make(map[string]string)
	}
	w.rollout.Spec.Template.Annotations[key] = value
}

func (w *RolloutWorkload) GetContainers() []corev1.Container {
	return w.rollout.Spec.Template.Spec.Containers
}

func (w *RolloutWorkload) SetContainers(containers []corev1.Container) {
	w.rollout.Spec.Template.Spec.Containers = containers
}

func (w *RolloutWorkload) GetInitContainers() []corev1.Container {
	return w.rollout.Spec.Template.Spec.InitContainers
}

func (w *RolloutWorkload) SetInitContainers(containers []corev1.Container) {
	w.rollout.Spec.Template.Spec.InitContainers = containers
}

func (w *RolloutWorkload) GetVolumes() []corev1.Volume {
	return w.rollout.Spec.Template.Spec.Volumes
}

// Update updates the Rollout. It uses the rollout strategy annotation to determine
// whether to do a standard rollout or set the restartAt field.
func (w *RolloutWorkload) Update(ctx context.Context, c client.Client) error {
	strategy := w.getStrategy()
	switch strategy {
	case RolloutStrategyRestart:
		// Set restartAt field to trigger a restart
		restartAt := metav1.NewTime(time.Now())
		w.rollout.Spec.RestartAt = &restartAt
	}
	return c.Patch(ctx, w.rollout, client.StrategicMergeFrom(w.original), client.FieldOwner(FieldManager))
}

// getStrategy returns the rollout strategy from the annotation.
func (w *RolloutWorkload) getStrategy() RolloutStrategy {
	annotations := w.rollout.GetAnnotations()
	if annotations == nil {
		return RolloutStrategyRollout
	}
	strategy := annotations[RolloutStrategyAnnotation]
	switch RolloutStrategy(strategy) {
	case RolloutStrategyRestart:
		return RolloutStrategyRestart
	default:
		return RolloutStrategyRollout
	}
}

func (w *RolloutWorkload) DeepCopy() Workload {
	return &RolloutWorkload{
		rollout:  w.rollout.DeepCopy(),
		original: w.original.DeepCopy(),
	}
}

func (w *RolloutWorkload) ResetOriginal() {
	w.original = w.rollout.DeepCopy()
}

func (w *RolloutWorkload) GetEnvFromSources() []corev1.EnvFromSource {
	var sources []corev1.EnvFromSource
	for _, container := range w.rollout.Spec.Template.Spec.Containers {
		sources = append(sources, container.EnvFrom...)
	}
	for _, container := range w.rollout.Spec.Template.Spec.InitContainers {
		sources = append(sources, container.EnvFrom...)
	}
	return sources
}

func (w *RolloutWorkload) UsesConfigMap(name string) bool {
	return SpecUsesConfigMap(&w.rollout.Spec.Template.Spec, name)
}

func (w *RolloutWorkload) UsesSecret(name string) bool {
	return SpecUsesSecret(&w.rollout.Spec.Template.Spec, name)
}

func (w *RolloutWorkload) GetOwnerReferences() []metav1.OwnerReference {
	return w.rollout.OwnerReferences
}

// GetRollout returns the underlying Rollout for special handling.
func (w *RolloutWorkload) GetRollout() *argorolloutv1alpha1.Rollout {
	return w.rollout
}

// GetStrategy returns the configured rollout strategy.
func (w *RolloutWorkload) GetStrategy() RolloutStrategy {
	return w.getStrategy()
}

// String returns a string representation of the strategy.
func (s RolloutStrategy) String() string {
	return string(s)
}

// ToRolloutStrategy converts a string to RolloutStrategy.
func ToRolloutStrategy(s string) RolloutStrategy {
	switch RolloutStrategy(s) {
	case RolloutStrategyRestart:
		return RolloutStrategyRestart
	case RolloutStrategyRollout:
		return RolloutStrategyRollout
	default:
		return RolloutStrategyRollout
	}
}

// Validate checks if the rollout strategy is valid.
func (s RolloutStrategy) Validate() error {
	switch s {
	case RolloutStrategyRollout, RolloutStrategyRestart:
		return nil
	default:
		return fmt.Errorf("invalid rollout strategy: %s", s)
	}
}
