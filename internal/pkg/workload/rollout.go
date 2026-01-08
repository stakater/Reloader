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

// rolloutAccessor implements PodTemplateAccessor for Rollout.
type rolloutAccessor struct {
	rollout *argorolloutv1alpha1.Rollout
}

func (a *rolloutAccessor) GetPodTemplateSpec() *corev1.PodTemplateSpec {
	return &a.rollout.Spec.Template
}

func (a *rolloutAccessor) GetObjectMeta() *metav1.ObjectMeta {
	return &a.rollout.ObjectMeta
}

// RolloutWorkload wraps an Argo Rollout.
type RolloutWorkload struct {
	*BaseWorkload[*argorolloutv1alpha1.Rollout]
	strategyAnnotation string
}

// NewRolloutWorkload creates a new RolloutWorkload.
// The strategyAnnotation parameter specifies the annotation key used to determine
// the rollout strategy (from config.Annotations.RolloutStrategy).
func NewRolloutWorkload(r *argorolloutv1alpha1.Rollout, strategyAnnotation string) *RolloutWorkload {
	original := r.DeepCopy()
	accessor := &rolloutAccessor{rollout: r}
	return &RolloutWorkload{
		BaseWorkload:       NewBaseWorkload(r, original, accessor, KindArgoRollout),
		strategyAnnotation: strategyAnnotation,
	}
}

// Ensure RolloutWorkload implements Workload.
var _ Workload = (*RolloutWorkload)(nil)

// Update updates the Rollout. It uses the rollout strategy annotation to determine
// whether to do a standard rollout or set the restartAt field.
func (w *RolloutWorkload) Update(ctx context.Context, c client.Client) error {
	strategy := w.getStrategy()
	switch strategy {
	case RolloutStrategyRestart:
		// Set restartAt field to trigger a restart
		restartAt := metav1.NewTime(time.Now())
		w.Object().Spec.RestartAt = &restartAt
	}
	return c.Patch(ctx, w.Object(), client.MergeFrom(w.Original()), client.FieldOwner(FieldManager))
}

// getStrategy returns the rollout strategy from the annotation.
func (w *RolloutWorkload) getStrategy() RolloutStrategy {
	annotations := w.Object().GetAnnotations()
	if annotations == nil {
		return RolloutStrategyRollout
	}
	strategy := annotations[w.strategyAnnotation]
	switch RolloutStrategy(strategy) {
	case RolloutStrategyRestart:
		return RolloutStrategyRestart
	default:
		return RolloutStrategyRollout
	}
}

func (w *RolloutWorkload) DeepCopy() Workload {
	return NewRolloutWorkload(w.Object().DeepCopy(), w.strategyAnnotation)
}

// GetRollout returns the underlying Rollout for special handling.
func (w *RolloutWorkload) GetRollout() *argorolloutv1alpha1.Rollout {
	return w.Object()
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
