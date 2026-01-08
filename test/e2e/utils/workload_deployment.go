package utils

import (
	"context"
	"time"

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

// WaitReady waits for the Deployment to be ready.
func (a *DeploymentAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForDeploymentReady(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the Deployment to have the reload annotation.
func (a *DeploymentAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForDeploymentReloaded(ctx, a.client, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the Deployment to have a STAKATER_ env var.
func (a *DeploymentAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForDeploymentEnvVar(ctx, a.client, namespace, name, prefix, timeout)
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
	var opts []DeploymentOption

	// Add annotations
	if len(cfg.Annotations) > 0 {
		opts = append(opts, WithAnnotations(cfg.Annotations))
	}

	// Add envFrom references
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		opts = append(opts, WithConfigMapEnvFrom(cfg.ConfigMapName))
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		opts = append(opts, WithSecretEnvFrom(cfg.SecretName))
	}

	// Add volume mounts
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		opts = append(opts, WithConfigMapVolume(cfg.ConfigMapName))
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		opts = append(opts, WithSecretVolume(cfg.SecretName))
	}

	// Add projected volume
	if cfg.UseProjectedVolume {
		opts = append(opts, WithProjectedVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add valueFrom references
	if cfg.UseConfigMapKeyRef && cfg.ConfigMapName != "" {
		key := cfg.ConfigMapKey
		if key == "" {
			key = "key"
		}
		envVar := cfg.EnvVarName
		if envVar == "" {
			envVar = "CONFIG_VAR"
		}
		opts = append(opts, WithConfigMapKeyRef(cfg.ConfigMapName, key, envVar))
	}
	if cfg.UseSecretKeyRef && cfg.SecretName != "" {
		key := cfg.SecretKey
		if key == "" {
			key = "key"
		}
		envVar := cfg.EnvVarName
		if envVar == "" {
			envVar = "SECRET_VAR"
		}
		opts = append(opts, WithSecretKeyRef(cfg.SecretName, key, envVar))
	}

	// Add init container with envFrom
	if cfg.UseInitContainer {
		opts = append(opts, WithInitContainer(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add init container with volume mount
	if cfg.UseInitContainerVolume {
		opts = append(opts, WithInitContainerVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add multiple containers
	if cfg.MultipleContainers > 1 {
		opts = append(opts, WithMultipleContainers(cfg.MultipleContainers))
	}

	return opts
}
