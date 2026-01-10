package utils

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
)

// DaemonSetAdapter implements WorkloadAdapter for Kubernetes DaemonSets.
type DaemonSetAdapter struct {
	client kubernetes.Interface
}

// NewDaemonSetAdapter creates a new DaemonSetAdapter.
func NewDaemonSetAdapter(client kubernetes.Interface) *DaemonSetAdapter {
	return &DaemonSetAdapter{client: client}
}

// Type returns the workload type.
func (a *DaemonSetAdapter) Type() WorkloadType {
	return WorkloadDaemonSet
}

// Create creates a DaemonSet with the given config.
func (a *DaemonSetAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildDaemonSetOptions(cfg)
	_, err := CreateDaemonSet(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the DaemonSet.
func (a *DaemonSetAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteDaemonSet(ctx, a.client, namespace, name)
}

// WaitReady waits for the DaemonSet to be ready.
func (a *DaemonSetAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForDaemonSetReady(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the DaemonSet to have the reload annotation.
func (a *DaemonSetAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForDaemonSetReloaded(ctx, a.client, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the DaemonSet to have a STAKATER_ env var.
func (a *DaemonSetAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForDaemonSetEnvVar(ctx, a.client, namespace, name, prefix, timeout)
}

// SupportsEnvVarStrategy returns true as DaemonSets support env var reload strategy.
func (a *DaemonSetAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as DaemonSets use standard rolling restart.
func (a *DaemonSetAdapter) RequiresSpecialHandling() bool {
	return false
}

// buildDaemonSetOptions converts WorkloadConfig to DaemonSetOption slice.
func buildDaemonSetOptions(cfg WorkloadConfig) []DaemonSetOption {
	return []DaemonSetOption{
		func(ds *appsv1.DaemonSet) {
			// Set annotations on DaemonSet level (where Reloader checks them)
			if len(cfg.Annotations) > 0 {
				if ds.Annotations == nil {
					ds.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					ds.Annotations[k] = v
				}
			}
			ApplyWorkloadConfig(&ds.Spec.Template.Spec, cfg)
		},
	}
}
