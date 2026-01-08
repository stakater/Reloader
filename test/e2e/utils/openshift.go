package utils

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// DeploymentConfigGVR returns the GroupVersionResource for OpenShift DeploymentConfigs.
var DeploymentConfigGVR = schema.GroupVersionResource{
	Group:    "apps.openshift.io",
	Version:  "v1",
	Resource: "deploymentconfigs",
}

// DCOption is a functional option for configuring a DeploymentConfig.
type DCOption func(*unstructured.Unstructured)

// HasDeploymentConfigSupport checks if the cluster has OpenShift DeploymentConfig API available.
func HasDeploymentConfigSupport(discoveryClient discovery.DiscoveryInterface) bool {
	_, apiLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false
	}

	for _, apiList := range apiLists {
		for _, resource := range apiList.APIResources {
			if resource.Kind == "DeploymentConfig" {
				return true
			}
		}
	}

	return false
}

// CreateDeploymentConfig creates an OpenShift DeploymentConfig with the given options.
func CreateDeploymentConfig(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string, opts ...DCOption) error {
	dc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.openshift.io/v1",
			"kind":       "DeploymentConfig",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{
					"app": name,
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
				"triggers": []interface{}{
					map[string]interface{}{
						"type": "ConfigChange",
					},
				},
			},
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(dc)
	}

	_, err := dynamicClient.Resource(DeploymentConfigGVR).Namespace(namespace).Create(ctx, dc, metav1.CreateOptions{})
	return err
}

// DeleteDeploymentConfig deletes a DeploymentConfig.
func DeleteDeploymentConfig(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) error {
	return dynamicClient.Resource(DeploymentConfigGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// GetDeploymentConfig retrieves a DeploymentConfig.
func GetDeploymentConfig(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	return dynamicClient.Resource(DeploymentConfigGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// WithDCConfigMapEnvFrom adds a ConfigMap envFrom to the DeploymentConfig.
func WithDCConfigMapEnvFrom(configMapName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
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
			_ = unstructured.SetNestedSlice(dc.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithDCSecretEnvFrom adds a Secret envFrom to the DeploymentConfig.
func WithDCSecretEnvFrom(secretName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
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
			_ = unstructured.SetNestedSlice(dc.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithDCConfigMapVolume adds a ConfigMap volume to the DeploymentConfig.
func WithDCConfigMapVolume(configMapName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		// Add volume
		volumes, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "volumes")
		volumes = append(volumes, map[string]interface{}{
			"name": configMapName + "-volume",
			"configMap": map[string]interface{}{
				"name": configMapName,
			},
		})
		_ = unstructured.SetNestedSlice(dc.Object, volumes, "spec", "template", "spec", "volumes")

		// Add volumeMount
		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			volumeMounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      configMapName + "-volume",
				"mountPath": "/etc/config/" + configMapName,
			})
			container["volumeMounts"] = volumeMounts
			containers[0] = container
			_ = unstructured.SetNestedSlice(dc.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithDCSecretVolume adds a Secret volume to the DeploymentConfig.
func WithDCSecretVolume(secretName string) DCOption {
	return func(dc *unstructured.Unstructured) {
		// Add volume
		volumes, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "volumes")
		volumes = append(volumes, map[string]interface{}{
			"name": secretName + "-volume",
			"secret": map[string]interface{}{
				"secretName": secretName,
			},
		})
		_ = unstructured.SetNestedSlice(dc.Object, volumes, "spec", "template", "spec", "volumes")

		// Add volumeMount
		containers, _, _ := unstructured.NestedSlice(dc.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			volumeMounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
			volumeMounts = append(volumeMounts, map[string]interface{}{
				"name":      secretName + "-volume",
				"mountPath": "/etc/secrets/" + secretName,
			})
			container["volumeMounts"] = volumeMounts
			containers[0] = container
			_ = unstructured.SetNestedSlice(dc.Object, containers, "spec", "template", "spec", "containers")
		}
	}
}

// WithDCAnnotations adds annotations to the DeploymentConfig's pod template.
func WithDCAnnotations(annotations map[string]string) DCOption {
	return func(dc *unstructured.Unstructured) {
		annotationsMap := make(map[string]interface{})
		for k, v := range annotations {
			annotationsMap[k] = v
		}
		_ = unstructured.SetNestedMap(dc.Object, annotationsMap, "spec", "template", "metadata", "annotations")
	}
}

// WaitForDeploymentConfigReady waits for a DeploymentConfig to be ready.
func WaitForDeploymentConfigReady(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		dc, err := dynamicClient.Resource(DeploymentConfigGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep polling
		}

		// Check replicas == readyReplicas
		replicas, _, _ := unstructured.NestedInt64(dc.Object, "spec", "replicas")
		readyReplicas, _, _ := unstructured.NestedInt64(dc.Object, "status", "readyReplicas")

		if replicas > 0 && replicas == readyReplicas {
			return true, nil
		}

		return false, nil
	})
}

// WaitForDeploymentConfigReloaded waits for a DeploymentConfig's pod template to have the reloader annotation.
func WaitForDeploymentConfigReloaded(ctx context.Context, dynamicClient dynamic.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		dc, err := dynamicClient.Resource(DeploymentConfigGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check pod template annotations
		annotations, _, _ := unstructured.NestedStringMap(dc.Object, "spec", "template", "metadata", "annotations")
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

// GetDeploymentConfigPodTemplateAnnotations retrieves the pod template annotations from a DeploymentConfig.
func GetDeploymentConfigPodTemplateAnnotations(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) (map[string]string, error) {
	dc, err := dynamicClient.Resource(DeploymentConfigGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	annotations, _, _ := unstructured.NestedStringMap(dc.Object, "spec", "template", "metadata", "annotations")
	return annotations, nil
}
