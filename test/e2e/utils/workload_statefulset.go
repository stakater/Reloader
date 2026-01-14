package utils

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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

// WaitReady waits for the StatefulSet to be ready using watches.
func (a *StatefulSetAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().StatefulSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, IsReady(StatefulSetIsReady), timeout)
	return err
}

// WaitReloaded waits for the StatefulSet to have the reload annotation using watches.
func (a *StatefulSetAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().StatefulSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasPodTemplateAnnotation(StatefulSetPodTemplate, annotationKey), timeout)
	return HandleWatchResult(err)
}

// WaitEnvVar waits for the StatefulSet to have a STAKATER_ env var using watches.
func (a *StatefulSetAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().StatefulSets(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasEnvVarPrefix(StatefulSetContainers, prefix), timeout)
	return HandleWatchResult(err)
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
			ApplyWorkloadConfig(&sts.Spec.Template, cfg)
		},
	}
}
