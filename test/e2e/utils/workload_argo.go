package utils

import (
	"context"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsclient "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/ptr"
)

// ArgoRolloutAdapter implements WorkloadAdapter for Argo Rollouts.
type ArgoRolloutAdapter struct {
	rolloutsClient rolloutsclient.Interface
}

// NewArgoRolloutAdapter creates a new ArgoRolloutAdapter.
func NewArgoRolloutAdapter(rolloutsClient rolloutsclient.Interface) *ArgoRolloutAdapter {
	return &ArgoRolloutAdapter{
		rolloutsClient: rolloutsClient,
	}
}

// Type returns the workload type.
func (a *ArgoRolloutAdapter) Type() WorkloadType {
	return WorkloadArgoRollout
}

// Create creates an Argo Rollout with the given config.
func (a *ArgoRolloutAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	rollout := baseRollout(name)
	opts := buildRolloutOptions(cfg)
	for _, opt := range opts {
		opt(rollout)
	}
	_, err := a.rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Create(ctx, rollout, metav1.CreateOptions{})
	return err
}

// Delete removes the Argo Rollout.
func (a *ArgoRolloutAdapter) Delete(ctx context.Context, namespace, name string) error {
	return a.rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// WaitReady waits for the Argo Rollout to be ready using watches.
func (a *ArgoRolloutAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, IsReady(RolloutIsReady), timeout)
	return err
}

// WaitReloaded waits for the Argo Rollout to have the reload annotation using watches.
func (a *ArgoRolloutAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasPodTemplateAnnotation(RolloutPodTemplate, annotationKey), timeout)
	return HandleWatchResult(err)
}

// WaitEnvVar waits for the Argo Rollout to have a STAKATER_ env var using watches.
func (a *ArgoRolloutAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasEnvVarPrefix(RolloutContainers, prefix), timeout)
	return HandleWatchResult(err)
}

// WaitRestartAt waits for the Argo Rollout to have the restartAt field set using watches.
// This is used when Reloader is configured with rollout strategy=restart.
func (a *ArgoRolloutAdapter) WaitRestartAt(ctx context.Context, namespace, name string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, IsReady(RolloutHasRestartAt), timeout)
	return HandleWatchResult(err)
}

// SupportsEnvVarStrategy returns true as Argo Rollouts support env var reload strategy.
func (a *ArgoRolloutAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as Argo Rollouts use standard rolling restart.
func (a *ArgoRolloutAdapter) RequiresSpecialHandling() bool {
	return false
}

// GetPodTemplateAnnotation returns the value of a pod template annotation.
func (a *ArgoRolloutAdapter) GetPodTemplateAnnotation(ctx context.Context, namespace, name, annotationKey string) (string, error) {
	rollout, err := a.rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return rollout.Spec.Template.Annotations[annotationKey], nil
}

// baseRollout returns a minimal Rollout template.
func baseRollout(name string) *rolloutv1alpha1.Rollout {
	return &rolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: rolloutv1alpha1.RolloutSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "main",
						Image:   DefaultImage,
						Command: []string{"sh", "-c", DefaultCommand},
					}},
				},
			},
			Strategy: rolloutv1alpha1.RolloutStrategy{
				Canary: &rolloutv1alpha1.CanaryStrategy{
					Steps: []rolloutv1alpha1.CanaryStep{
						{SetWeight: ptr.To[int32](100)},
					},
				},
			},
		},
	}
}

// buildRolloutOptions converts WorkloadConfig to RolloutOption slice.
func buildRolloutOptions(cfg WorkloadConfig) []RolloutOption {
	return []RolloutOption{
		func(r *rolloutv1alpha1.Rollout) {
			if len(cfg.Annotations) > 0 {
				if r.Annotations == nil {
					r.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					r.Annotations[k] = v
				}
			}
			ApplyWorkloadConfig(&r.Spec.Template, cfg)
		},
	}
}
