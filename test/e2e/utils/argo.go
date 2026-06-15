package utils

import (
	"context"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsclient "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// RolloutOption is a function that modifies a Rollout.
type RolloutOption func(*rolloutv1alpha1.Rollout)

// IsArgoRolloutsInstalled checks if Argo Rollouts CRD is installed in the cluster.
func IsArgoRolloutsInstalled(ctx context.Context, client rolloutsclient.Interface) bool {
	if client == nil {
		return false
	}
	_, err := client.ArgoprojV1alpha1().Rollouts("default").List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// CreateRollout creates an Argo Rollout with the given options.
func CreateRollout(ctx context.Context, client rolloutsclient.Interface, namespace, name string, opts ...RolloutOption) (*rolloutv1alpha1.Rollout, error) {
	rollout := &rolloutv1alpha1.Rollout{
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

	for _, opt := range opts {
		opt(rollout)
	}

	return client.ArgoprojV1alpha1().Rollouts(namespace).Create(ctx, rollout, metav1.CreateOptions{})
}

// DeleteRollout deletes an Argo Rollout using typed client.
func DeleteRollout(ctx context.Context, client rolloutsclient.Interface, namespace, name string) error {
	return client.ArgoprojV1alpha1().Rollouts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// WithRolloutConfigMapEnvFrom adds a ConfigMap envFrom to the Rollout.
func WithRolloutConfigMapEnvFrom(configMapName string) RolloutOption {
	return func(r *rolloutv1alpha1.Rollout) {
		AddEnvFromSource(&r.Spec.Template.Spec, 0, configMapName, false)
	}
}

// WithRolloutSecretEnvFrom adds a Secret envFrom to the Rollout.
func WithRolloutSecretEnvFrom(secretName string) RolloutOption {
	return func(r *rolloutv1alpha1.Rollout) {
		AddEnvFromSource(&r.Spec.Template.Spec, 0, secretName, true)
	}
}

// WithRolloutConfigMapVolume adds a ConfigMap volume to the Rollout.
func WithRolloutConfigMapVolume(configMapName string) RolloutOption {
	return func(r *rolloutv1alpha1.Rollout) {
		AddConfigMapVolume(&r.Spec.Template.Spec, 0, configMapName)
	}
}

// WithRolloutSecretVolume adds a Secret volume to the Rollout.
func WithRolloutSecretVolume(secretName string) RolloutOption {
	return func(r *rolloutv1alpha1.Rollout) {
		AddSecretVolume(&r.Spec.Template.Spec, 0, secretName)
	}
}

// WithRolloutAnnotations adds annotations to the Rollout level (where Reloader checks them).
func WithRolloutAnnotations(annotations map[string]string) RolloutOption {
	return func(r *rolloutv1alpha1.Rollout) {
		if len(annotations) > 0 {
			if r.Annotations == nil {
				r.Annotations = make(map[string]string)
			}
			for k, v := range annotations {
				r.Annotations[k] = v
			}
		}
	}
}

// WithRolloutObjectAnnotations adds annotations to the Rollout's top-level metadata.
func WithRolloutObjectAnnotations(annotations map[string]string) RolloutOption {
	return func(r *rolloutv1alpha1.Rollout) {
		if r.Annotations == nil {
			r.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			r.Annotations[k] = v
		}
	}
}
