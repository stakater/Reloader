package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
)

// DeploymentConfigAdapter implements WorkloadAdapter for OpenShift DeploymentConfigs.
type DeploymentConfigAdapter struct {
	dynamicClient dynamic.Interface
}

// NewDeploymentConfigAdapter creates a new DeploymentConfigAdapter.
func NewDeploymentConfigAdapter(dynamicClient dynamic.Interface) *DeploymentConfigAdapter {
	return &DeploymentConfigAdapter{dynamicClient: dynamicClient}
}

// Type returns the workload type.
func (a *DeploymentConfigAdapter) Type() WorkloadType {
	return WorkloadDeploymentConfig
}

// Create creates a DeploymentConfig with the given config.
func (a *DeploymentConfigAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildDCOptions(cfg)
	return CreateDeploymentConfig(ctx, a.dynamicClient, namespace, name, opts...)
}

// Delete removes the DeploymentConfig.
func (a *DeploymentConfigAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteDeploymentConfig(ctx, a.dynamicClient, namespace, name)
}

// WaitReady waits for the DeploymentConfig to be ready.
func (a *DeploymentConfigAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForDeploymentConfigReady(ctx, a.dynamicClient, namespace, name, timeout)
}

// WaitReloaded waits for the DeploymentConfig to have the reload annotation.
func (a *DeploymentConfigAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForDeploymentConfigReloaded(ctx, a.dynamicClient, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the DeploymentConfig to have a STAKATER_ env var.
func (a *DeploymentConfigAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForDeploymentConfigEnvVar(ctx, a.dynamicClient, namespace, name, prefix, timeout)
}

// SupportsEnvVarStrategy returns true as DeploymentConfigs support env var reload strategy.
func (a *DeploymentConfigAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as DeploymentConfigs use standard rolling restart.
func (a *DeploymentConfigAdapter) RequiresSpecialHandling() bool {
	return false
}

// buildDCOptions converts WorkloadConfig to DCOption slice.
func buildDCOptions(cfg WorkloadConfig) []DCOption {
	var opts []DCOption

	// Add annotations (to pod template)
	if len(cfg.Annotations) > 0 {
		opts = append(opts, WithDCAnnotations(cfg.Annotations))
	}

	// Add envFrom references
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		opts = append(opts, WithDCConfigMapEnvFrom(cfg.ConfigMapName))
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		opts = append(opts, WithDCSecretEnvFrom(cfg.SecretName))
	}

	// Add volume mounts
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		opts = append(opts, WithDCConfigMapVolume(cfg.ConfigMapName))
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		opts = append(opts, WithDCSecretVolume(cfg.SecretName))
	}

	// Add projected volume
	if cfg.UseProjectedVolume {
		opts = append(opts, WithDCProjectedVolume(cfg.ConfigMapName, cfg.SecretName))
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
		opts = append(opts, WithDCConfigMapKeyRef(cfg.ConfigMapName, key, envVar))
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
		opts = append(opts, WithDCSecretKeyRef(cfg.SecretName, key, envVar))
	}

	// Add init container with envFrom
	if cfg.UseInitContainer {
		opts = append(opts, WithDCInitContainer(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add init container with volume mount
	if cfg.UseInitContainerVolume {
		opts = append(opts, WithDCInitContainerVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	return opts
}

// WithDCProjectedVolume adds a projected volume with ConfigMap and/or Secret sources to a DeploymentConfig.
func WithDCProjectedVolume(cmName, secretName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		volumeName := "projected-config"
		sources := []interface{}{}

		if cmName != "" {
			sources = append(sources, map[string]interface{}{
				"configMap": map[string]interface{}{
					"name": cmName,
				},
			})
		}
		if secretName != "" {
			sources = append(sources, map[string]interface{}{
				"secret": map[string]interface{}{
					"name": secretName,
				},
			})
		}

		// Add volume
		volumes, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "volumes")
		volumes = append(volumes, map[string]interface{}{
			"name": volumeName,
			"projected": map[string]interface{}{
				"sources": sources,
			},
		})
		_ = unstructured.SetNestedSlice(dc.Object, volumes, "spec", "template", "spec", "volumes")

		// Add volumeMount
		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			volumeMounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      volumeName,
				"mountPath": "/etc/projected",
			})
			container["volumeMounts"] = volumeMounts
			containers[0] = container
			_ = unstructured.SetNestedSlice(dc.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithDCConfigMapKeyRef adds an env var with valueFrom.configMapKeyRef to a DeploymentConfig.
func WithDCConfigMapKeyRef(cmName, key, envVarName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			env, _, _ := unstructured.NestedSlice(container, "env")
			env = append(env, map[string]interface{}{
				"name": envVarName,
				"valueFrom": map[string]interface{}{
					"configMapKeyRef": map[string]interface{}{
						"name": cmName,
						"key":  key,
					},
				},
			})
			container["env"] = env
			containers[0] = container
			_ = unstructured.SetNestedSlice(dc.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithDCSecretKeyRef adds an env var with valueFrom.secretKeyRef to a DeploymentConfig.
func WithDCSecretKeyRef(secretName, key, envVarName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			env, _, _ := unstructured.NestedSlice(container, "env")
			env = append(env, map[string]interface{}{
				"name": envVarName,
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": secretName,
						"key":  key,
					},
				},
			})
			container["env"] = env
			containers[0] = container
			_ = unstructured.SetNestedSlice(dc.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithDCInitContainer adds an init container that references ConfigMap and/or Secret via envFrom.
func WithDCInitContainer(cmName, secretName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		initContainer := map[string]interface{}{
			"name":    "init",
			"image":   DefaultImage,
			"command": []interface{}{"sh", "-c", "echo init done"},
		}

		envFrom := []interface{}{}
		if cmName != "" {
			envFrom = append(envFrom, map[string]interface{}{
				"configMapRef": map[string]interface{}{
					"name": cmName,
				},
			})
		}
		if secretName != "" {
			envFrom = append(envFrom, map[string]interface{}{
				"secretRef": map[string]interface{}{
					"name": secretName,
				},
			})
		}
		if len(envFrom) > 0 {
			initContainer["envFrom"] = envFrom
		}

		initContainers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "initContainers")
		initContainers = append(initContainers, initContainer)
		_ = unstructured.SetNestedSlice(dc.Object, initContainers, "spec", "template", "spec", "initContainers")
	}
}

// WithDCInitContainerVolume adds an init container with ConfigMap/Secret volume mounts to a DeploymentConfig.
func WithDCInitContainerVolume(cmName, secretName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		initContainer := map[string]interface{}{
			"name":    "init",
			"image":   DefaultImage,
			"command": []interface{}{"sh", "-c", "echo init done"},
		}

		volumeMounts := []interface{}{}
		volumes, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "volumes")

		if cmName != "" {
			volumeName := fmt.Sprintf("init-cm-%s", cmName)
			volumes = append(volumes, map[string]interface{}{
				"name": volumeName,
				"configMap": map[string]interface{}{
					"name": cmName,
				},
			})
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      volumeName,
				"mountPath": fmt.Sprintf("/etc/init-config/%s", cmName),
			})
		}
		if secretName != "" {
			volumeName := fmt.Sprintf("init-secret-%s", secretName)
			volumes = append(volumes, map[string]interface{}{
				"name": volumeName,
				"secret": map[string]interface{}{
					"secretName": secretName,
				},
			})
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      volumeName,
				"mountPath": fmt.Sprintf("/etc/init-secrets/%s", secretName),
			})
		}

		if len(volumeMounts) > 0 {
			initContainer["volumeMounts"] = volumeMounts
		}

		_ = unstructured.SetNestedSlice(dc.Object, volumes, "spec", "template", "spec", "volumes")

		initContainers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "initContainers")
		initContainers = append(initContainers, initContainer)
		_ = unstructured.SetNestedSlice(dc.Object, initContainers, "spec", "template", "spec", "initContainers")
	}
}

// WaitForDeploymentConfigEnvVar waits for a DeploymentConfig's container to have an env var with the given prefix.
func WaitForDeploymentConfigEnvVar(ctx context.Context, dynamicClient dynamic.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		dc, err := dynamicClient.Resource(DeploymentConfigGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
		for _, c := range containers {
			container := c.(map[string]interface{})
			env, _, _ := unstructured.NestedSlice(container, "env")
			for _, e := range env {
				envVar := e.(map[string]interface{})
				if envName, ok := envVar["name"].(string); ok && strings.HasPrefix(envName, prefix) {
					found = true
					return true, nil
				}
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}
