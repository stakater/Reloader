package utils

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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

// WaitReady waits for the DaemonSet to be ready using watches.
func (a *DaemonSetAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().DaemonSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, IsReady(DaemonSetIsReady), timeout)
	return err
}

// WaitReloaded waits for the DaemonSet to have the reload annotation using watches.
func (a *DaemonSetAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().DaemonSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasPodTemplateAnnotation(DaemonSetPodTemplate, annotationKey), timeout)
	return HandleWatchResult(err)
}

// WaitEnvVar waits for the DaemonSet to have a STAKATER_ env var using watches.
func (a *DaemonSetAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().DaemonSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasEnvVarPrefix(DaemonSetContainers, prefix), timeout)
	return HandleWatchResult(err)
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
			ApplyWorkloadConfig(&ds.Spec.Template, cfg)
		},
	}
}
