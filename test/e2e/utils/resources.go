package utils

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

const (
	// DefaultImage is the default container image used for test workloads.
	DefaultImage = "busybox:1.36"
	// DefaultCommand is the default command for test containers.
	DefaultCommand = "sleep 3600"
)

// CreateNamespace creates a namespace with the given name.
func CreateNamespace(ctx context.Context, client kubernetes.Interface, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

// CreateNamespaceWithLabels creates a namespace with the given name and labels.
func CreateNamespaceWithLabels(ctx context.Context, client kubernetes.Interface, name string, labels map[string]string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	_, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

// DeleteNamespace deletes the namespace with the given name.
func DeleteNamespace(ctx context.Context, client kubernetes.Interface, name string) error {
	return client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}

// CreateConfigMap creates a ConfigMap with the given name, data, and optional annotations.
func CreateConfigMap(ctx context.Context, client kubernetes.Interface, namespace, name string, data map[string]string, annotations map[string]string) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Data: data,
	}
	return client.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
}

// CreateConfigMapWithLabels creates a ConfigMap with the given name, data, labels, and optional annotations.
func CreateConfigMapWithLabels(ctx context.Context, client kubernetes.Interface, namespace, name string, data map[string]string, labels, annotations map[string]string) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: data,
	}
	return client.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
}

// CreateSecret creates a Secret with the given name, data, and optional annotations.
func CreateSecret(ctx context.Context, client kubernetes.Interface, namespace, name string, data map[string][]byte, annotations map[string]string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Data: data,
	}
	return client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
}

// UpdateConfigMap updates a ConfigMap's data.
func UpdateConfigMap(ctx context.Context, client kubernetes.Interface, namespace, name string, data map[string]string) error {
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cm.Data = data
	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// UpdateConfigMapLabels updates a ConfigMap's labels.
func UpdateConfigMapLabels(ctx context.Context, client kubernetes.Interface, namespace, name string, labels map[string]string) error {
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	for k, v := range labels {
		cm.Labels[k] = v
	}
	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// UpdateSecret updates a Secret's data.
func UpdateSecret(ctx context.Context, client kubernetes.Interface, namespace, name string, data map[string][]byte) error {
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	secret.Data = data
	_, err = client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// UpdateSecretLabels updates a Secret's labels.
func UpdateSecretLabels(ctx context.Context, client kubernetes.Interface, namespace, name string, labels map[string]string) error {
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	for k, v := range labels {
		secret.Labels[k] = v
	}
	_, err = client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// stringToByteMap converts a string map to a byte map for Secret data.
func stringToByteMap(data map[string]string) map[string][]byte {
	result := make(map[string][]byte)
	for k, v := range data {
		result[k] = []byte(v)
	}
	return result
}

// CreateSecretFromStrings creates a Secret with string data (convenience wrapper).
func CreateSecretFromStrings(ctx context.Context, client kubernetes.Interface, namespace, name string, data map[string]string, annotations map[string]string) (*corev1.Secret, error) {
	return CreateSecret(ctx, client, namespace, name, stringToByteMap(data), annotations)
}

// UpdateSecretFromStrings updates a Secret's data using string values.
func UpdateSecretFromStrings(ctx context.Context, client kubernetes.Interface, namespace, name string, data map[string]string) error {
	return UpdateSecret(ctx, client, namespace, name, stringToByteMap(data))
}

// DeleteConfigMap deletes a ConfigMap.
func DeleteConfigMap(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeleteSecret deletes a Secret.
func DeleteSecret(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeploymentOption is a functional option for configuring a Deployment.
type DeploymentOption func(*appsv1.Deployment)

// CreateDeployment creates a Deployment with the given options.
func CreateDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string, opts ...DeploymentOption) (*appsv1.Deployment, error) {
	deploy := baseDeploymentResource(namespace, name)
	for _, opt := range opts {
		opt(deploy)
	}
	return client.AppsV1().Deployments(namespace).Create(ctx, deploy, metav1.CreateOptions{})
}

// WithAnnotations adds annotations to the Deployment metadata.
func WithAnnotations(annotations map[string]string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		if d.Annotations == nil {
			d.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			d.Annotations[k] = v
		}
	}
}

// WithConfigMapEnvFrom adds an envFrom reference to a ConfigMap.
func WithConfigMapEnvFrom(name string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].EnvFrom = append(
			d.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		)
	}
}

// WithSecretEnvFrom adds an envFrom reference to a Secret.
func WithSecretEnvFrom(name string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].EnvFrom = append(
			d.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		)
	}
}

// WithConfigMapVolume adds a volume mount for a ConfigMap.
func WithConfigMapVolume(name string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		volumeName := fmt.Sprintf("cm-%s", name)
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		})
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/config/%s", name),
			},
		)
	}
}

// WithSecretVolume adds a volume mount for a Secret.
func WithSecretVolume(name string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		volumeName := fmt.Sprintf("secret-%s", name)
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: name,
				},
			},
		})
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/secrets/%s", name),
			},
		)
	}
}

// WithProjectedVolume adds a projected volume with ConfigMap and/or Secret sources.
func WithProjectedVolume(cmName, secretName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		volumeName := "projected-config"
		sources := []corev1.VolumeProjection{}

		if cmName != "" {
			sources = append(sources, corev1.VolumeProjection{
				ConfigMap: &corev1.ConfigMapProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			})
		}
		if secretName != "" {
			sources = append(sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			})
		}

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: sources,
				},
			},
		})
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/etc/projected",
			},
		)
	}
}

// WithInitContainer adds an init container that references ConfigMap and/or Secret.
func WithInitContainer(cmName, secretName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		initContainer := corev1.Container{
			Name:    "init",
			Image:   DefaultImage,
			Command: []string{"sh", "-c", "echo init done"},
		}

		if cmName != "" {
			initContainer.EnvFrom = append(initContainer.EnvFrom, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			})
		}
		if secretName != "" {
			initContainer.EnvFrom = append(initContainer.EnvFrom, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			})
		}

		d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, initContainer)
	}
}

// WithMultipleContainers adds additional containers to the pod.
func WithMultipleContainers(count int) DeploymentOption {
	return func(d *appsv1.Deployment) {
		for i := 1; i < count; i++ {
			d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, corev1.Container{
				Name:    fmt.Sprintf("container-%d", i),
				Image:   DefaultImage,
				Command: []string{"sh", "-c", DefaultCommand},
			})
		}
	}
}

// WithMultipleContainersAndEnv creates two containers, each with a different ConfigMap envFrom.
func WithMultipleContainersAndEnv(cm1Name, cm2Name string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].EnvFrom = append(d.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cm1Name},
				},
			})
		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, corev1.Container{
			Name:    "container-1",
			Image:   DefaultImage,
			Command: []string{"sh", "-c", DefaultCommand},
			EnvFrom: []corev1.EnvFromSource{
				{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cm2Name},
					},
				},
			},
		})
	}
}

// WithReplicas sets the number of replicas.
func WithReplicas(replicas int32) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Replicas = ptr.To(replicas)
	}
}

// WithConfigMapKeyRef adds a valueFrom.configMapKeyRef env var to the container.
func WithConfigMapKeyRef(cmName, key, envVarName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Env = append(
			d.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name: envVarName,
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
						Key:                  key,
					},
				},
			},
		)
	}
}

// WithSecretKeyRef adds a valueFrom.secretKeyRef env var to the container.
func WithSecretKeyRef(secretName, key, envVarName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Env = append(
			d.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name: envVarName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  key,
					},
				},
			},
		)
	}
}

// WithPodTemplateAnnotations adds annotations to the pod template metadata (not deployment metadata).
func WithPodTemplateAnnotations(annotations map[string]string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Annotations == nil {
			d.Spec.Template.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			d.Spec.Template.Annotations[k] = v
		}
	}
}

// WithInitContainerVolume adds an init container with ConfigMap/Secret volume mounts.
func WithInitContainerVolume(cmName, secretName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		initContainer := corev1.Container{
			Name:    "init",
			Image:   DefaultImage,
			Command: []string{"sh", "-c", "echo init done"},
		}

		if cmName != "" {
			volumeName := fmt.Sprintf("init-cm-%s", cmName)
			d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
					},
				},
			})
			initContainer.VolumeMounts = append(initContainer.VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/init-config/%s", cmName),
			})
		}
		if secretName != "" {
			volumeName := fmt.Sprintf("init-secret-%s", secretName)
			d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
					},
				},
			})
			initContainer.VolumeMounts = append(initContainer.VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/etc/init-secrets/%s", secretName),
			})
		}

		d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, initContainer)
	}
}

// WithInitContainerProjectedVolume adds an init container with projected volume.
func WithInitContainerProjectedVolume(cmName, secretName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		volumeName := "init-projected-config"
		sources := []corev1.VolumeProjection{}

		if cmName != "" {
			sources = append(sources, corev1.VolumeProjection{
				ConfigMap: &corev1.ConfigMapProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			})
		}
		if secretName != "" {
			sources = append(sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			})
		}

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: sources,
				},
			},
		})

		initContainer := corev1.Container{
			Name:    "init",
			Image:   DefaultImage,
			Command: []string{"sh", "-c", "echo init done"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeName,
					MountPath: "/etc/init-projected",
				},
			},
		}

		d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, initContainer)
	}
}

// WithCSIVolume adds a CSI volume referencing a SecretProviderClass to a Deployment.
func WithCSIVolume(spcName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		volumeName := csiVolumeName(spcName)
		mountPath := csiMountPath(spcName)

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   CSIDriverName,
					ReadOnly: ptr.To(true),
					VolumeAttributes: map[string]string{
						"secretProviderClass": spcName,
					},
				},
			},
		})
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: mountPath,
				ReadOnly:  true,
			},
		)
	}
}

// WithInitContainerCSIVolume adds an init container with a CSI volume mount.
func WithInitContainerCSIVolume(spcName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		volumeName := csiVolumeName(spcName)
		mountPath := csiMountPath(spcName)

		hasCSIVolume := false
		for _, v := range d.Spec.Template.Spec.Volumes {
			if v.Name == volumeName {
				hasCSIVolume = true
				break
			}
		}
		if !hasCSIVolume {
			d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:   CSIDriverName,
						ReadOnly: ptr.To(true),
						VolumeAttributes: map[string]string{
							"secretProviderClass": spcName,
						},
					},
				},
			})
		}

		initContainer := corev1.Container{
			Name:    fmt.Sprintf("init-csi-%s", spcName),
			Image:   DefaultImage,
			Command: []string{"sh", "-c", "echo init done"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeName,
					MountPath: mountPath,
					ReadOnly:  true,
				},
			},
		}
		d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, initContainer)
	}
}

func baseDeploymentResource(namespace, name string) *appsv1.Deployment {
	labels := map[string]string{"app": name}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   DefaultImage,
							Command: []string{"sh", "-c", DefaultCommand},
						},
					},
				},
			},
		},
	}
}

// DeleteDeployment deletes a Deployment.
func DeleteDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DaemonSetOption is a functional option for configuring a DaemonSet.
type DaemonSetOption func(*appsv1.DaemonSet)

// CreateDaemonSet creates a DaemonSet with the given options.
func CreateDaemonSet(ctx context.Context, client kubernetes.Interface, namespace, name string, opts ...DaemonSetOption) (*appsv1.DaemonSet, error) {
	ds := baseDaemonSetResource(namespace, name)
	for _, opt := range opts {
		opt(ds)
	}
	return client.AppsV1().DaemonSets(namespace).Create(ctx, ds, metav1.CreateOptions{})
}

// baseDaemonSetResource creates a base DaemonSet template.
func baseDaemonSetResource(namespace, name string) *appsv1.DaemonSet {
	labels := map[string]string{"app": name}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   DefaultImage,
							Command: []string{"sh", "-c", DefaultCommand},
						},
					},
				},
			},
		},
	}
}

// DeleteDaemonSet deletes a DaemonSet.
func DeleteDaemonSet(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.AppsV1().DaemonSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// StatefulSetOption is a functional option for configuring a StatefulSet.
type StatefulSetOption func(*appsv1.StatefulSet)

// CreateStatefulSet creates a StatefulSet with the given options.
func CreateStatefulSet(ctx context.Context, client kubernetes.Interface, namespace, name string, opts ...StatefulSetOption) (*appsv1.StatefulSet, error) {
	ss := baseStatefulSetResource(namespace, name)
	for _, opt := range opts {
		opt(ss)
	}
	return client.AppsV1().StatefulSets(namespace).Create(ctx, ss, metav1.CreateOptions{})
}

// baseStatefulSetResource creates a base StatefulSet template.
func baseStatefulSetResource(namespace, name string) *appsv1.StatefulSet {
	labels := map[string]string{"app": name}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   DefaultImage,
							Command: []string{"sh", "-c", DefaultCommand},
						},
					},
				},
			},
		},
	}
}

// DeleteStatefulSet deletes a StatefulSet.
func DeleteStatefulSet(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// CronJobOption is a functional option for configuring a CronJob.
type CronJobOption func(*batchv1.CronJob)

// CreateCronJob creates a CronJob with the given options.
func CreateCronJob(ctx context.Context, client kubernetes.Interface, namespace, name string, opts ...CronJobOption) (*batchv1.CronJob, error) {
	cj := baseCronJobResource(namespace, name)
	for _, opt := range opts {
		opt(cj)
	}
	return client.BatchV1().CronJobs(namespace).Create(ctx, cj, metav1.CreateOptions{})
}

// WithCronJobAnnotations adds annotations to the CronJob metadata.
func WithCronJobAnnotations(annotations map[string]string) CronJobOption {
	return func(cj *batchv1.CronJob) {
		if cj.Annotations == nil {
			cj.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			cj.Annotations[k] = v
		}
	}
}

// WithCronJobConfigMapEnvFrom adds an envFrom reference to a ConfigMap.
func WithCronJobConfigMapEnvFrom(name string) CronJobOption {
	return func(cj *batchv1.CronJob) {
		cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].EnvFrom = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		)
	}
}

// WithCronJobSecretEnvFrom adds an envFrom reference to a Secret.
func WithCronJobSecretEnvFrom(name string) CronJobOption {
	return func(cj *batchv1.CronJob) {
		cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].EnvFrom = append(
			cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		)
	}
}

// baseCronJobResource creates a base CronJob template.
func baseCronJobResource(namespace, name string) *batchv1.CronJob {
	labels := map[string]string{"app": name}
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "* * * * *", // Every minute
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:    "job",
									Image:   DefaultImage,
									Command: []string{"sh", "-c", "echo done"},
								},
							},
						},
					},
				},
			},
		},
	}
}

// DeleteCronJob deletes a CronJob.
func DeleteCronJob(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.BatchV1().CronJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// JobOption is a functional option for configuring a Job.
type JobOption func(*batchv1.Job)

// CreateJob creates a Job with the given options.
func CreateJob(ctx context.Context, client kubernetes.Interface, namespace, name string, opts ...JobOption) (*batchv1.Job, error) {
	job := baseJobResource(namespace, name)
	for _, opt := range opts {
		opt(job)
	}
	return client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
}

// WithJobAnnotations adds annotations to the Job metadata.
func WithJobAnnotations(annotations map[string]string) JobOption {
	return func(j *batchv1.Job) {
		if j.Annotations == nil {
			j.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			j.Annotations[k] = v
		}
	}
}

// WithJobConfigMapEnvFrom adds an envFrom reference to a ConfigMap.
func WithJobConfigMapEnvFrom(name string) JobOption {
	return func(j *batchv1.Job) {
		j.Spec.Template.Spec.Containers[0].EnvFrom = append(
			j.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		)
	}
}

// WithJobSecretEnvFrom adds an envFrom reference to a Secret.
func WithJobSecretEnvFrom(name string) JobOption {
	return func(j *batchv1.Job) {
		j.Spec.Template.Spec.Containers[0].EnvFrom = append(
			j.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		)
	}
}

// WithJobConfigMapKeyRef adds a valueFrom.configMapKeyRef env var to a Job.
func WithJobConfigMapKeyRef(cmName, key, envVarName string) JobOption {
	return func(j *batchv1.Job) {
		j.Spec.Template.Spec.Containers[0].Env = append(
			j.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name: envVarName,
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
						Key:                  key,
					},
				},
			},
		)
	}
}

// WithJobSecretKeyRef adds a valueFrom.secretKeyRef env var to a Job.
func WithJobSecretKeyRef(secretName, key, envVarName string) JobOption {
	return func(j *batchv1.Job) {
		j.Spec.Template.Spec.Containers[0].Env = append(
			j.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name: envVarName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  key,
					},
				},
			},
		)
	}
}

// WithJobCSIVolume adds a CSI volume referencing a SecretProviderClass to a Job.
func WithJobCSIVolume(spcName string) JobOption {
	return func(j *batchv1.Job) {
		volumeName := csiVolumeName(spcName)
		mountPath := csiMountPath(spcName)

		j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   CSIDriverName,
					ReadOnly: ptr.To(true),
					VolumeAttributes: map[string]string{
						"secretProviderClass": spcName,
					},
				},
			},
		})
		j.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			j.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: mountPath,
				ReadOnly:  true,
			},
		)
	}
}

// baseJobResource creates a base Job template.
func baseJobResource(namespace, name string) *batchv1.Job {
	labels := map[string]string{"app": name}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "job",
							Image:   DefaultImage,
							Command: []string{"sh", "-c", "echo done"},
						},
					},
				},
			},
		},
	}
}

// DeleteJob deletes a Job.
func DeleteJob(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	propagation := metav1.DeletePropagationBackground
	return client.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
}

func csiVolumeName(spcName string) string {
	return fmt.Sprintf("csi-%s", spcName)
}

func csiMountPath(spcName string) string {
	return fmt.Sprintf("/mnt/secrets-store/%s", spcName)
}

// GetDeployment retrieves a deployment by name.
func GetDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string) (*appsv1.Deployment, error) {
	return client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetPodLogs retrieves logs from pods matching the given label selector.
func GetPodLogs(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) (string, error) {
	pods, err := client.CoreV1().Pods(namespace).List(
		ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	var allLogs strings.Builder
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			logs, err := client.CoreV1().Pods(namespace).GetLogs(
				pod.Name, &corev1.PodLogOptions{
					Container: container.Name,
				},
			).Do(ctx).Raw()
			if err != nil {
				allLogs.WriteString(fmt.Sprintf("Error getting logs for %s/%s: %v\n", pod.Name, container.Name, err))
				continue
			}
			allLogs.WriteString(fmt.Sprintf("=== %s/%s ===\n%s\n", pod.Name, container.Name, string(logs)))
		}
	}

	return allLogs.String(), nil
}
