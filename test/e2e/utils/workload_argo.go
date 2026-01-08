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

// ArgoRolloutAdapter implements WorkloadAdapter for Argo Rollouts.
type ArgoRolloutAdapter struct {
	dynamicClient dynamic.Interface
}

// NewArgoRolloutAdapter creates a new ArgoRolloutAdapter.
func NewArgoRolloutAdapter(dynamicClient dynamic.Interface) *ArgoRolloutAdapter {
	return &ArgoRolloutAdapter{dynamicClient: dynamicClient}
}

// Type returns the workload type.
func (a *ArgoRolloutAdapter) Type() WorkloadType {
	return WorkloadArgoRollout
}

// Create creates an Argo Rollout with the given config.
func (a *ArgoRolloutAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildRolloutOptions(cfg)
	return CreateArgoRollout(ctx, a.dynamicClient, namespace, name, opts...)
}

// Delete removes the Argo Rollout.
func (a *ArgoRolloutAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteArgoRollout(ctx, a.dynamicClient, namespace, name)
}

// WaitReady waits for the Argo Rollout to be ready.
func (a *ArgoRolloutAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForRolloutReady(ctx, a.dynamicClient, namespace, name, timeout)
}

// WaitReloaded waits for the Argo Rollout to have the reload annotation.
func (a *ArgoRolloutAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForRolloutReloaded(ctx, a.dynamicClient, namespace, name, annotationKey, timeout)
}

// WaitEnvVar waits for the Argo Rollout to have a STAKATER_ env var.
func (a *ArgoRolloutAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return WaitForRolloutEnvVar(ctx, a.dynamicClient, namespace, name, prefix, timeout)
}

// SupportsEnvVarStrategy returns true as Argo Rollouts support env var reload strategy.
func (a *ArgoRolloutAdapter) SupportsEnvVarStrategy() bool {
	return true
}

// RequiresSpecialHandling returns false as Argo Rollouts use standard rolling restart.
func (a *ArgoRolloutAdapter) RequiresSpecialHandling() bool {
	return false
}

// buildRolloutOptions converts WorkloadConfig to RolloutOption slice.
func buildRolloutOptions(cfg WorkloadConfig) []RolloutOption {
	var opts []RolloutOption

	// Add annotations (to pod template)
	if len(cfg.Annotations) > 0 {
		opts = append(opts, WithRolloutAnnotations(cfg.Annotations))
	}

	// Add envFrom references
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		opts = append(opts, WithRolloutConfigMapEnvFrom(cfg.ConfigMapName))
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		opts = append(opts, WithRolloutSecretEnvFrom(cfg.SecretName))
	}

	// Add volume mounts
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		opts = append(opts, WithRolloutConfigMapVolume(cfg.ConfigMapName))
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		opts = append(opts, WithRolloutSecretVolume(cfg.SecretName))
	}

	// Add projected volume
	if cfg.UseProjectedVolume {
		opts = append(opts, WithRolloutProjectedVolume(cfg.ConfigMapName, cfg.SecretName))
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
		opts = append(opts, WithRolloutConfigMapKeyRef(cfg.ConfigMapName, key, envVar))
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
		opts = append(opts, WithRolloutSecretKeyRef(cfg.SecretName, key, envVar))
	}

	// Add init container with envFrom
	if cfg.UseInitContainer {
		opts = append(opts, WithRolloutInitContainer(cfg.ConfigMapName, cfg.SecretName))
	}

	// Add init container with volume mount
	if cfg.UseInitContainerVolume {
		opts = append(opts, WithRolloutInitContainerVolume(cfg.ConfigMapName, cfg.SecretName))
	}

	return opts
}

// WithRolloutProjectedVolume adds a projected volume with ConfigMap and/or Secret sources to a Rollout.
func WithRolloutProjectedVolume(cmName, secretName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
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
		volumes, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "volumes")
		volumes = append(volumes, map[string]interface{}{
			"name": volumeName,
			"projected": map[string]interface{}{
				"sources": sources,
			},
		})
		_ = unstructured.SetNestedSlice(rollout.Object, volumes, "spec", "template", "spec", "volumes")

		// Add volumeMount
		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			volumeMounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      volumeName,
				"mountPath": "/etc/projected",
			})
			container["volumeMounts"] = volumeMounts
			containers[0] = container
			_ = unstructured.SetNestedSlice(rollout.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithRolloutConfigMapKeyRef adds an env var with valueFrom.configMapKeyRef to a Rollout.
func WithRolloutConfigMapKeyRef(cmName, key, envVarName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
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
			_ = unstructured.SetNestedSlice(rollout.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithRolloutSecretKeyRef adds an env var with valueFrom.secretKeyRef to a Rollout.
func WithRolloutSecretKeyRef(secretName, key, envVarName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
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
			_ = unstructured.SetNestedSlice(rollout.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithRolloutInitContainer adds an init container that references ConfigMap and/or Secret.
func WithRolloutInitContainer(cmName, secretName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
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

		initContainers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "initContainers")
		initContainers = append(initContainers, initContainer)
		_ = unstructured.SetNestedSlice(rollout.Object, initContainers, "spec", "template", "spec", "initContainers")
	}
}

// WithRolloutInitContainerVolume adds an init container with ConfigMap/Secret volume mounts.
func WithRolloutInitContainerVolume(cmName, secretName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		initContainer := map[string]interface{}{
			"name":    "init",
			"image":   DefaultImage,
			"command": []interface{}{"sh", "-c", "echo init done"},
		}

		volumeMounts := []interface{}{}
		volumes, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "volumes")

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

		_ = unstructured.SetNestedSlice(rollout.Object, volumes, "spec", "template", "spec", "volumes")

		initContainers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "initContainers")
		initContainers = append(initContainers, initContainer)
		_ = unstructured.SetNestedSlice(rollout.Object, initContainers, "spec", "template", "spec", "initContainers")
	}
}

// WaitForRolloutEnvVar waits for an Argo Rollout's container to have an env var with the given prefix.
func WaitForRolloutEnvVar(ctx context.Context, dynamicClient dynamic.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		rollout, err := dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
		for _, c := range containers {
			container := c.(map[string]interface{})
			env, _, _ := unstructured.NestedSlice(container, "env")
			for _, e := range env {
				envVar := e.(map[string]interface{})
				if name, ok := envVar["name"].(string); ok && strings.HasPrefix(name, prefix) {
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
