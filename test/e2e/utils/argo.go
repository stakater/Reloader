package utils

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
)

// ArgoRolloutGVR returns the GroupVersionResource for Argo Rollouts.
var ArgoRolloutGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "rollouts",
}

// RolloutOption is a functional option for configuring an Argo Rollout.
type RolloutOption func(*unstructured.Unstructured)

// IsArgoRolloutsInstalled checks if Argo Rollouts CRD is installed in the cluster.
func IsArgoRolloutsInstalled(ctx context.Context, dynamicClient dynamic.Interface) bool {
	// Try to list rollouts - if CRD exists, this will succeed (possibly with empty list)
	_, err := dynamicClient.Resource(ArgoRolloutGVR).Namespace("default").List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// CreateArgoRollout creates an Argo Rollout with the given options.
func CreateArgoRollout(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string, opts ...RolloutOption) error {
	rollout := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Rollout",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": name,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": name,
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":    "app",
								"image":   "busybox:1.36",
								"command": []interface{}{"sh", "-c", "sleep 3600"},
							},
						},
					},
				},
				"strategy": map[string]interface{}{
					"canary": map[string]interface{}{
						"steps": []interface{}{
							map[string]interface{}{
								"setWeight": int64(100),
							},
						},
					},
				},
			},
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(rollout)
	}

	_, err := dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Create(ctx, rollout, metav1.CreateOptions{})
	return err
}

// DeleteArgoRollout deletes an Argo Rollout.
func DeleteArgoRollout(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) error {
	err := dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	return err
}

// GetArgoRollout retrieves an Argo Rollout.
func GetArgoRollout(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	return dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// WithRolloutConfigMapEnvFrom adds a ConfigMap envFrom to the Rollout.
func WithRolloutConfigMapEnvFrom(configMapName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			envFrom, _, _ := unstructured.NestedSlice(container, "envFrom")
			envFrom = append(envFrom, map[string]interface{}{
				"configMapRef": map[string]interface{}{
					"name": configMapName,
				},
			})
			container["envFrom"] = envFrom
			containers[0] = container
			_ = unstructured.SetNestedSlice(rollout.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithRolloutSecretEnvFrom adds a Secret envFrom to the Rollout.
func WithRolloutSecretEnvFrom(secretName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			envFrom, _, _ := unstructured.NestedSlice(container, "envFrom")
			envFrom = append(envFrom, map[string]interface{}{
				"secretRef": map[string]interface{}{
					"name": secretName,
				},
			})
			container["envFrom"] = envFrom
			containers[0] = container
			_ = unstructured.SetNestedSlice(rollout.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithRolloutConfigMapVolume adds a ConfigMap volume to the Rollout.
func WithRolloutConfigMapVolume(configMapName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		// Add volume
		volumes, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "volumes")
		volumes = append(volumes, map[string]interface{}{
			"name": configMapName + "-volume",
			"configMap": map[string]interface{}{
				"name": configMapName,
			},
		})
		_ = unstructured.SetNestedSlice(rollout.Object, volumes, "spec", "template", "spec", "volumes")

		// Add volumeMount
		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			volumeMounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      configMapName + "-volume",
				"mountPath": "/etc/config/" + configMapName,
			})
			container["volumeMounts"] = volumeMounts
			containers[0] = container
			_ = unstructured.SetNestedSlice(rollout.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithRolloutSecretVolume adds a Secret volume to the Rollout.
func WithRolloutSecretVolume(secretName string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		// Add volume
		volumes, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "volumes")
		volumes = append(volumes, map[string]interface{}{
			"name": secretName + "-volume",
			"secret": map[string]interface{}{
				"secretName": secretName,
			},
		})
		_ = unstructured.SetNestedSlice(rollout.Object, volumes, "spec", "template", "spec", "volumes")

		// Add volumeMount
		containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			volumeMounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      secretName + "-volume",
				"mountPath": "/etc/secrets/" + secretName,
			})
			container["volumeMounts"] = volumeMounts
			containers[0] = container
			_ = unstructured.SetNestedSlice(rollout.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithRolloutAnnotations adds annotations to the Rollout's pod template.
func WithRolloutAnnotations(annotations map[string]string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		annotationsMap := make(map[string]interface{})
		for k, v := range annotations {
			annotationsMap[k] = v
		}
		_ = unstructured.SetNestedMap(rollout.Object, annotationsMap, "spec", "template", "metadata", "annotations")
	}
}

// WithRolloutObjectAnnotations adds annotations to the Rollout's top-level metadata.
// Use this for annotations that are read from the Rollout object itself (like rollout-strategy).
func WithRolloutObjectAnnotations(annotations map[string]string) RolloutOption {
	return func(rollout *unstructured.Unstructured) {
		annotationsMap := make(map[string]interface{})
		for k, v := range annotations {
			annotationsMap[k] = v
		}
		_ = unstructured.SetNestedMap(rollout.Object, annotationsMap, "metadata", "annotations")
	}
}

// WaitForRolloutReady waits for an Argo Rollout to be ready.
func WaitForRolloutReady(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		rollout, err := dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep polling
		}

		// Check status.phase == "Healthy" or replicas == availableReplicas
		status, found, _ := unstructured.NestedMap(rollout.Object, "status")
		if !found {
			return false, nil
		}

		phase, _, _ := unstructured.NestedString(status, "phase")
		if phase == "Healthy" {
			return true, nil
		}

		// Alternative: check replicas
		replicas, _, _ := unstructured.NestedInt64(rollout.Object, "spec", "replicas")
		availableReplicas, _, _ := unstructured.NestedInt64(status, "availableReplicas")
		if replicas > 0 && replicas == availableReplicas {
			return true, nil
		}

		return false, nil
	})
}

// WaitForRolloutReloaded waits for an Argo Rollout's pod template to have the reloader annotation.
func WaitForRolloutReloaded(ctx context.Context, dynamicClient dynamic.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		rollout, err := dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check pod template annotations
		annotations, _, _ := unstructured.NestedStringMap(rollout.Object, "spec", "template", "metadata", "annotations")
		if annotations != nil {
			if _, ok := annotations[annotationKey]; ok {
				found = true
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// GetRolloutPodTemplateAnnotations retrieves the pod template annotations from an Argo Rollout.
func GetRolloutPodTemplateAnnotations(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) (map[string]string, error) {
	rollout, err := dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	annotations, _, _ := unstructured.NestedStringMap(rollout.Object, "spec", "template", "metadata", "annotations")
	return annotations, nil
}

// WaitForRolloutRestartAt waits for an Argo Rollout's spec.restartAt field to be set.
// This is used when the restart strategy is specified.
func WaitForRolloutRestartAt(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		rollout, err := dynamicClient.Resource(ArgoRolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check if spec.restartAt is set
		restartAt, exists, _ := unstructured.NestedString(rollout.Object, "spec", "restartAt")
		if exists && restartAt != "" {
			found = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}
