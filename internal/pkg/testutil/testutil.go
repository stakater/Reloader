package testutil

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// ConfigmapResourceType represents ConfigMap resource type
	ConfigmapResourceType = "configmap"
	// SecretResourceType represents Secret resource type
	SecretResourceType = "secret"
)

// CreateNamespace creates a namespace with the given name.
func CreateNamespace(name string, client kubernetes.Interface) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := client.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	return err
}

// DeleteNamespace deletes the namespace with the given name.
func DeleteNamespace(name string, client kubernetes.Interface) error {
	return client.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
}

// CreateConfigMap creates a ConfigMap with the given name and data.
func CreateConfigMap(client kubernetes.Interface, namespace, name, data string) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"url": data,
		},
	}
	return client.CoreV1().ConfigMaps(namespace).Create(context.Background(), cm, metav1.CreateOptions{})
}

// UpdateConfigMap updates the ConfigMap with new label and/or data.
func UpdateConfigMap(cm *corev1.ConfigMap, namespace, name, label, data string) error {
	if label != "" {
		if cm.Labels == nil {
			cm.Labels = make(map[string]string)
		}
		cm.Labels["test-label"] = label
	}
	if data != "" {
		cm.Data["url"] = data
	}
	// Note: caller must have a client to update
	return nil
}

// UpdateConfigMapWithClient updates the ConfigMap with new label and/or data.
func UpdateConfigMapWithClient(client kubernetes.Interface, namespace, name, label, data string) error {
	ctx := context.Background()
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if label != "" {
		if cm.Labels == nil {
			cm.Labels = make(map[string]string)
		}
		cm.Labels["test-label"] = label
	}
	if data != "" {
		cm.Data["url"] = data
	}
	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// DeleteConfigMap deletes the ConfigMap with the given name.
func DeleteConfigMap(client kubernetes.Interface, namespace, name string) error {
	return client.CoreV1().ConfigMaps(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// CreateSecret creates a Secret with the given name and data.
func CreateSecret(client kubernetes.Interface, namespace, name, data string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"password": []byte(data),
		},
	}
	return client.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

// UpdateSecretWithClient updates the Secret with new label and/or data.
func UpdateSecretWithClient(client kubernetes.Interface, namespace, name, label, data string) error {
	ctx := context.Background()
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if label != "" {
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		secret.Labels["test-label"] = label
	}
	if data != "" {
		secret.Data["password"] = []byte(data)
	}
	_, err = client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// DeleteSecret deletes the Secret with the given name.
func DeleteSecret(client kubernetes.Interface, namespace, name string) error {
	return client.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// CreateDeployment creates a Deployment that references a ConfigMap/Secret.
func CreateDeployment(client kubernetes.Interface, name, namespace string, useConfigMap bool, annotations map[string]string) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment
	if useConfigMap {
		deployment = NewDeploymentWithEnvFrom(name, namespace, name, "")
	} else {
		deployment = NewDeploymentWithEnvFrom(name, namespace, "", name)
	}
	deployment.Annotations = annotations
	// Override image for integration tests
	deployment.Spec.Template.Spec.Containers[0].Image = "busybox:1.36"
	deployment.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", "while true; do sleep 3600; done"}

	return client.AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
}

// DeleteDeployment deletes the Deployment with the given name.
func DeleteDeployment(client kubernetes.Interface, namespace, name string) error {
	return client.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// CreateDaemonSet creates a DaemonSet that references a ConfigMap/Secret.
func CreateDaemonSet(client kubernetes.Interface, name, namespace string, useConfigMap bool, annotations map[string]string) (*appsv1.DaemonSet, error) {
	daemonset := NewDaemonSet(name, namespace, annotations)
	// Override image for integration tests
	daemonset.Spec.Template.Spec.Containers[0].Image = "busybox:1.36"
	daemonset.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", "while true; do sleep 3600; done"}

	if useConfigMap {
		daemonset.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		}
	} else {
		daemonset.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		}
	}

	return client.AppsV1().DaemonSets(namespace).Create(context.Background(), daemonset, metav1.CreateOptions{})
}

// DeleteDaemonSet deletes the DaemonSet with the given name.
func DeleteDaemonSet(client kubernetes.Interface, namespace, name string) error {
	return client.AppsV1().DaemonSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// CreateStatefulSet creates a StatefulSet that references a ConfigMap/Secret.
func CreateStatefulSet(client kubernetes.Interface, name, namespace string, useConfigMap bool, annotations map[string]string) (*appsv1.StatefulSet, error) {
	statefulset := NewStatefulSet(name, namespace, annotations)
	statefulset.Spec.ServiceName = name
	// Override image for integration tests
	statefulset.Spec.Template.Spec.Containers[0].Image = "busybox:1.36"
	statefulset.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", "while true; do sleep 3600; done"}

	if useConfigMap {
		statefulset.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		}
	} else {
		statefulset.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		}
	}

	return client.AppsV1().StatefulSets(namespace).Create(context.Background(), statefulset, metav1.CreateOptions{})
}

// DeleteStatefulSet deletes the StatefulSet with the given name.
func DeleteStatefulSet(client kubernetes.Interface, namespace, name string) error {
	return client.AppsV1().StatefulSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// CreateCronJob creates a CronJob that references a ConfigMap/Secret.
func CreateCronJob(client kubernetes.Interface, name, namespace string, useConfigMap bool, annotations map[string]string) (*batchv1.CronJob, error) {
	cronjob := NewCronJob(name, namespace)
	cronjob.Annotations = annotations
	// Override image for integration tests
	cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = "busybox:1.36"
	cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", "echo hello"}
	cronjob.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure

	if useConfigMap {
		cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		}
	} else {
		cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		}
	}

	return client.BatchV1().CronJobs(namespace).Create(context.Background(), cronjob, metav1.CreateOptions{})
}

// DeleteCronJob deletes the CronJob with the given name.
func DeleteCronJob(client kubernetes.Interface, namespace, name string) error {
	return client.BatchV1().CronJobs(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// ConvertResourceToSHA converts a resource data to SHA256 hash.
func ConvertResourceToSHA(resourceType, namespace, name, data string) string {
	content := fmt.Sprintf("%s/%s/%s:%s", resourceType, namespace, name, data)
	hash := sha256.Sum256([]byte(content))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// WaitForDeploymentAnnotation waits for a deployment to have the specified annotation value.
func WaitForDeploymentAnnotation(client kubernetes.Interface, namespace, name, annotation, expectedValue string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep waiting
		}
		value, ok := deployment.Spec.Template.Annotations[annotation]
		if !ok {
			return false, nil // Keep waiting
		}
		return value == expectedValue, nil
	})
}

// WaitForDeploymentReloadedAnnotation waits for a deployment to have any reloaded annotation.
func WaitForDeploymentReloadedAnnotation(client kubernetes.Interface, namespace, name string, cfg *config.Config, timeout time.Duration) (bool, error) {
	var found bool
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep waiting
		}
		// Check for the last-reloaded-from annotation in pod template
		if deployment.Spec.Template.Annotations != nil {
			if _, ok := deployment.Spec.Template.Annotations[cfg.Annotations.LastReloadedFrom]; ok {
				found = true
				return true, nil
			}
		}
		return false, nil
	})
	if wait.Interrupted(err) {
		return found, nil
	}
	return found, err
}

// WaitForDaemonSetReloadedAnnotation waits for a daemonset to have any reloaded annotation.
func WaitForDaemonSetReloadedAnnotation(client kubernetes.Interface, namespace, name string, cfg *config.Config, timeout time.Duration) (bool, error) {
	var found bool
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		daemonset, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep waiting
		}
		// Check for the last-reloaded-from annotation in pod template
		if daemonset.Spec.Template.Annotations != nil {
			if _, ok := daemonset.Spec.Template.Annotations[cfg.Annotations.LastReloadedFrom]; ok {
				found = true
				return true, nil
			}
		}
		return false, nil
	})
	if wait.Interrupted(err) {
		return found, nil
	}
	return found, err
}

// WaitForStatefulSetReloadedAnnotation waits for a statefulset to have any reloaded annotation.
func WaitForStatefulSetReloadedAnnotation(client kubernetes.Interface, namespace, name string, cfg *config.Config, timeout time.Duration) (bool, error) {
	var found bool
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		statefulset, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep waiting
		}
		// Check for the last-reloaded-from annotation in pod template
		if statefulset.Spec.Template.Annotations != nil {
			if _, ok := statefulset.Spec.Template.Annotations[cfg.Annotations.LastReloadedFrom]; ok {
				found = true
				return true, nil
			}
		}
		return false, nil
	})
	if wait.Interrupted(err) {
		return found, nil
	}
	return found, err
}
