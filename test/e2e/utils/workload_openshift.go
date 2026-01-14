package utils

import (
	"context"
	"errors"
	"time"

	openshiftappsv1 "github.com/openshift/api/apps/v1"
	openshiftclient "github.com/openshift/client-go/apps/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// DCOption is a function that modifies a DeploymentConfig.
type DCOption func(*openshiftappsv1.DeploymentConfig)

// DeploymentConfigAdapter implements WorkloadAdapter for OpenShift DeploymentConfigs.
type DeploymentConfigAdapter struct {
	openshiftClient openshiftclient.Interface
}

// NewDeploymentConfigAdapter creates a new DeploymentConfigAdapter.
func NewDeploymentConfigAdapter(openshiftClient openshiftclient.Interface) *DeploymentConfigAdapter {
	return &DeploymentConfigAdapter{
		openshiftClient: openshiftClient,
	}
}

// Type returns the workload type.
func (a *DeploymentConfigAdapter) Type() WorkloadType {
	return WorkloadDeploymentConfig
}

// Create creates a DeploymentConfig with the given config.
func (a *DeploymentConfigAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	dc := baseDeploymentConfig(name)
	opts := buildDeploymentConfigOptions(cfg)
	for _, opt := range opts {
		opt(dc)
	}
	_, err := a.openshiftClient.AppsV1().DeploymentConfigs(namespace).Create(ctx, dc, metav1.CreateOptions{})
	return err
}

// Delete removes the DeploymentConfig.
func (a *DeploymentConfigAdapter) Delete(ctx context.Context, namespace, name string) error {
	return a.openshiftClient.AppsV1().DeploymentConfigs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// WaitReady waits for the DeploymentConfig to be ready using watches.
func (a *DeploymentConfigAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.openshiftClient.AppsV1().DeploymentConfigs(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, IsReady(DeploymentConfigIsReady), timeout)
	return err
}

// WaitReloaded waits for the DeploymentConfig to have the reload annotation using watches.
func (a *DeploymentConfigAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.openshiftClient.AppsV1().DeploymentConfigs(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasPodTemplateAnnotation(DeploymentConfigPodTemplate, annotationKey), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return false, nil
	}
	return err == nil, err
}

// WaitEnvVar waits for the DeploymentConfig to have a STAKATER_ env var using watches.
func (a *DeploymentConfigAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.openshiftClient.AppsV1().DeploymentConfigs(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasEnvVarPrefix(DeploymentConfigContainers, prefix), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return false, nil
	}
	return err == nil, err
}

// SupportsEnvVarStrategy returns true as DeploymentConfigs support env var reload strategy.
func (a *DeploymentConfigAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as DeploymentConfigs use standard rolling restart.
func (a *DeploymentConfigAdapter) RequiresSpecialHandling() bool {
	return false
}

// baseDeploymentConfig returns a minimal DeploymentConfig template.
func baseDeploymentConfig(name string) *openshiftappsv1.DeploymentConfig {
	return &openshiftappsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: openshiftappsv1.DeploymentConfigSpec{
			Replicas: 1,
			Selector: map[string]string{"app": name},
			Template: &corev1.PodTemplateSpec{
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
			Triggers: openshiftappsv1.DeploymentTriggerPolicies{
				{Type: openshiftappsv1.DeploymentTriggerOnConfigChange},
			},
		},
	}
}

// buildDeploymentConfigOptions converts WorkloadConfig to DCOption slice.
func buildDeploymentConfigOptions(cfg WorkloadConfig) []DCOption {
	return []DCOption{
		func(dc *openshiftappsv1.DeploymentConfig) {
			// Set annotations on DeploymentConfig level (where Reloader checks them)
			if len(cfg.Annotations) > 0 {
				if dc.Annotations == nil {
					dc.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					dc.Annotations[k] = v
				}
			}
			if dc.Spec.Template != nil {
				ApplyWorkloadConfig(dc.Spec.Template, cfg)
			}
		},
	}
}
