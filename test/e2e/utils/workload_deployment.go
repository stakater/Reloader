package utils

import (
	"context"
	"errors"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// DeploymentAdapter implements WorkloadAdapter for Kubernetes Deployments.
type DeploymentAdapter struct {
	client kubernetes.Interface
}

// NewDeploymentAdapter creates a new DeploymentAdapter.
func NewDeploymentAdapter(client kubernetes.Interface) *DeploymentAdapter {
	return &DeploymentAdapter{client: client}
}

// Type returns the workload type.
func (a *DeploymentAdapter) Type() WorkloadType {
	return WorkloadDeployment
}

// Create creates a Deployment with the given config.
func (a *DeploymentAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildDeploymentOptions(cfg)
	_, err := CreateDeployment(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the Deployment.
func (a *DeploymentAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteDeployment(ctx, a.client, namespace, name)
}

// WaitReady waits for the Deployment to be ready using watches.
func (a *DeploymentAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().Deployments(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, IsReady(DeploymentIsReady), timeout)
	return err
}

// WaitReloaded waits for the Deployment to have the reload annotation using watches.
func (a *DeploymentAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().Deployments(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasPodTemplateAnnotation(DeploymentPodTemplate, annotationKey), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return false, nil
	}
	return err == nil, err
}

// WaitEnvVar waits for the Deployment to have a STAKATER_ env var using watches.
func (a *DeploymentAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().Deployments(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasEnvVarPrefix(DeploymentContainers, prefix), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return false, nil
	}
	return err == nil, err
}

// WaitPaused waits for the Deployment to have the paused annotation using watches.
func (a *DeploymentAdapter) WaitPaused(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().Deployments(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasAnnotation(DeploymentAnnotations, annotationKey), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return false, nil
	}
	return err == nil, err
}

// WaitUnpaused waits for the Deployment to NOT have the paused annotation using watches.
func (a *DeploymentAdapter) WaitUnpaused(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.AppsV1().Deployments(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, NoAnnotation(DeploymentAnnotations, annotationKey), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return false, nil
	}
	return err == nil, err
}

// SupportsEnvVarStrategy returns true as Deployments support env var reload strategy.
func (a *DeploymentAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as Deployments use standard rolling restart.
func (a *DeploymentAdapter) RequiresSpecialHandling() bool {
	return false
}

// buildDeploymentOptions converts WorkloadConfig to DeploymentOption slice.
func buildDeploymentOptions(cfg WorkloadConfig) []DeploymentOption {
	return []DeploymentOption{
		func(d *appsv1.Deployment) {
			// Set annotations on deployment level (where Reloader checks them)
			if len(cfg.Annotations) > 0 {
				if d.Annotations == nil {
					d.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					d.Annotations[k] = v
				}
			}
			ApplyWorkloadConfig(&d.Spec.Template, cfg)
		},
	}
}
