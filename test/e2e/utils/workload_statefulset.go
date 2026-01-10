package utils

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
)

// StatefulSetAdapter implements WorkloadAdapter for Kubernetes StatefulSets.
type StatefulSetAdapter struct {
	client kubernetes.Interface
}

// NewStatefulSetAdapter creates a new StatefulSetAdapter.
func NewStatefulSetAdapter(client kubernetes.Interface) *StatefulSetAdapter {
	return &StatefulSetAdapter{client: client}
}

// Type returns the workload type.
func (a *StatefulSetAdapter) Type() WorkloadType {
	return WorkloadStatefulSet
}

// Create creates a StatefulSet with the given config.
func (a *StatefulSetAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildStatefulSetOptions(cfg)
	_, err := CreateStatefulSet(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the StatefulSet.
func (a *StatefulSetAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteStatefulSet(ctx, a.client, namespace, name)
}

// WaitReady waits for the StatefulSet to be ready.
func (a *StatefulSetAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForStatefulSetReady(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the StatefulSet to have the reload annotation.
func (a *StatefulSetAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForStatefulSetReloaded(ctx, a.client, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the StatefulSet to have a STAKATER_ env var.
func (a *StatefulSetAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForStatefulSetEnvVar(ctx, a.client, namespace, name, prefix, timeout)
}

// SupportsEnvVarStrategy returns true as StatefulSets support env var reload strategy.
func (a *StatefulSetAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as StatefulSets use standard rolling restart.
func (a *StatefulSetAdapter) RequiresSpecialHandling() bool {
	return false
}

// buildStatefulSetOptions converts WorkloadConfig to StatefulSetOption slice.
func buildStatefulSetOptions(cfg WorkloadConfig) []StatefulSetOption {
	return []StatefulSetOption{
		func(sts *appsv1.StatefulSet) {
			// Set annotations on StatefulSet level (where Reloader checks them)
			if len(cfg.Annotations) > 0 {
				if sts.Annotations == nil {
					sts.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					sts.Annotations[k] = v
				}
			}
			ApplyWorkloadConfig(&sts.Spec.Template.Spec, cfg)
		},
	}
}
