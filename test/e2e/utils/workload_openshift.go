package utils

import (
	"context"
	"time"

	openshiftappsv1 "github.com/openshift/api/apps/v1"
	openshiftclient "github.com/openshift/client-go/apps/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

// WaitReady waits for the DeploymentConfig to be ready.
func (a *DeploymentConfigAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForDeploymentConfigReady(ctx, a.openshiftClient, namespace, name, timeout)
}

// WaitReloaded waits for the DeploymentConfig to have the reload annotation.
func (a *DeploymentConfigAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForDeploymentConfigReloaded(ctx, a.openshiftClient, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the DeploymentConfig to have a STAKATER_ env var.
func (a *DeploymentConfigAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForDeploymentConfigEnvVar(ctx, a.openshiftClient, namespace, name, prefix, timeout)
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

// WaitForDeploymentConfigReady waits for a DeploymentConfig to be ready using typed client.
func WaitForDeploymentConfigReady(ctx context.Context, client openshiftclient.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		dc, err := client.AppsV1().DeploymentConfigs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if dc.Spec.Replicas > 0 && dc.Status.ReadyReplicas == dc.Spec.Replicas {
			return true, nil
		}

		return false, nil
	})
}

// WaitForDeploymentConfigReloaded waits for a DeploymentConfig's pod template to have the reloader annotation.
func WaitForDeploymentConfigReloaded(ctx context.Context, client openshiftclient.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForAnnotation(ctx, func(ctx context.Context) (map[string]string, error) {
		dc, err := client.AppsV1().DeploymentConfigs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if dc.Spec.Template != nil {
			return dc.Spec.Template.Annotations, nil
		}
		return nil, nil
	}, annotationKey, timeout)
}

// WaitForDeploymentConfigEnvVar waits for a DeploymentConfig's container to have an env var with the given prefix.
func WaitForDeploymentConfigEnvVar(ctx context.Context, client openshiftclient.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForEnvVarPrefix(ctx, func(ctx context.Context) ([]corev1.Container, error) {
		dc, err := client.AppsV1().DeploymentConfigs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if dc.Spec.Template != nil {
			return dc.Spec.Template.Spec.Containers, nil
		}
		return nil, nil
	}, prefix, timeout)
}
