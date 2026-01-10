package utils

import (
	"context"
	"errors"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsclient "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

// WaitReady waits for the Argo Rollout to be ready.
func (a *ArgoRolloutAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForRolloutReady(ctx, a.rolloutsClient, namespace, name, timeout)
}

// WaitReloaded waits for the Argo Rollout to have the reload annotation.
func (a *ArgoRolloutAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForRolloutReloaded(ctx, a.rolloutsClient, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the Argo Rollout to have a STAKATER_ env var.
func (a *ArgoRolloutAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForRolloutEnvVar(ctx, a.rolloutsClient, namespace, name, prefix, timeout)
}

// SupportsEnvVarStrategy returns true as Argo Rollouts support env var reload strategy.
func (a *ArgoRolloutAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as Argo Rollouts use standard rolling restart.
func (a *ArgoRolloutAdapter) RequiresSpecialHandling() bool {
	return false
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
			// Set annotations on Rollout level (where Reloader checks them)
			if len(cfg.Annotations) > 0 {
				if r.Annotations == nil {
					r.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					r.Annotations[k] = v
				}
			}
			ApplyWorkloadConfig(&r.Spec.Template.Spec, cfg)
		},
	}
}

// WaitForRolloutReady waits for an Argo Rollout to be ready using typed client.
func WaitForRolloutReady(ctx context.Context, client rolloutsclient.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		rollout, err := client.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check status.phase == "Healthy" or replicas == availableReplicas
		if rollout.Status.Phase == rolloutv1alpha1.RolloutPhaseHealthy {
			return true, nil
		}

		if rollout.Spec.Replicas != nil && *rollout.Spec.Replicas > 0 &&
			rollout.Status.AvailableReplicas == *rollout.Spec.Replicas {
			return true, nil
		}

		return false, nil
	})
}

// WaitForRolloutReloaded waits for an Argo Rollout's pod template to have the reloader annotation.
func WaitForRolloutReloaded(ctx context.Context, client rolloutsclient.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForAnnotation(ctx, func(ctx context.Context) (map[string]string, error) {
		rollout, err := client.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return rollout.Spec.Template.Annotations, nil
	}, annotationKey, timeout)
}

// WaitForRolloutEnvVar waits for an Argo Rollout's container to have an env var with the given prefix.
func WaitForRolloutEnvVar(ctx context.Context, client rolloutsclient.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForEnvVarPrefix(ctx, func(ctx context.Context) ([]corev1.Container, error) {
		rollout, err := client.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return rollout.Spec.Template.Spec.Containers, nil
	}, prefix, timeout)
}

// WaitForRolloutRestartAt waits for an Argo Rollout's spec.restartAt field to be set.
func WaitForRolloutRestartAt(ctx context.Context, client rolloutsclient.Interface, namespace, name string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		rollout, err := client.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if rollout.Spec.RestartAt != nil && !rollout.Spec.RestartAt.IsZero() {
			found = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	return found, nil
}
