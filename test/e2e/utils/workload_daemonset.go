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
// Captures the current annotation value first to avoid false positives from prior reloads.
func (a *DaemonSetAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	priorValue, _ := a.GetPodTemplateAnnotation(ctx, namespace, name, annotationKey)
	return a.WaitReloadedFrom(ctx, namespace, name, annotationKey, priorValue, timeout)
}

// WaitReloadedFrom waits for the reload annotation to be present with a value different from
// priorValue, which the caller captured before triggering the reload.
func (a *DaemonSetAdapter) WaitReloadedFrom(ctx context.Context, namespace, name, annotationKey, priorValue string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().DaemonSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasPodTemplateAnnotationChanged(DaemonSetPodTemplate, annotationKey, priorValue), timeout)
	return HandleWatchResult(err)
}

// WaitEnvVar waits for the DaemonSet to have a STAKATER_ env var using watches.
// Captures the current env var value first to avoid false positives from prior reloads.
func (a *DaemonSetAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	priorValue := ""
	if ds, err := a.client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{}); err == nil {
		priorValue = GetEnvVarValueByPrefix(ds.Spec.Template.Spec.Containers, prefix)
	}
	return a.WaitEnvVarFrom(ctx, namespace, name, prefix, priorValue, timeout)
}

// WaitEnvVarFrom waits for a STAKATER_ env var whose value differs from priorValue, which the
// caller captured before triggering the reload.
func (a *DaemonSetAdapter) WaitEnvVarFrom(ctx context.Context, namespace, name, prefix, priorValue string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().DaemonSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasEnvVarPrefixChanged(DaemonSetContainers, prefix, priorValue), timeout)
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

// GetPodTemplateAnnotation returns the value of a pod template annotation.
func (a *DaemonSetAdapter) GetPodTemplateAnnotation(ctx context.Context, namespace, name, annotationKey string) (string, error) {
	ds, err := a.client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return ds.Spec.Template.Annotations[annotationKey], nil
}

// buildDaemonSetOptions converts WorkloadConfig to DaemonSetOption slice.
func buildDaemonSetOptions(cfg WorkloadConfig) []DaemonSetOption {
	return []DaemonSetOption{
		func(ds *appsv1.DaemonSet) {
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
