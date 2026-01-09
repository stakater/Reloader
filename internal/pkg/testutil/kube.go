package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	argorollout "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	openshiftv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stakater/Reloader/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	csiclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	csiclient_v1 "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/typed/apis/v1"
)

var (
	letters = []rune("abcdefghijklmnopqrstuvwxyz")
	// ConfigmapResourceType is a resource type which controller watches for changes
	ConfigmapResourceType = "configMaps"
	// SecretResourceType is a resource type which controller watches for changes
	SecretResourceType = "secrets"
	// SecretproviderclasspodstatusResourceType is a resource type which controller watches for changes
	SecretProviderClassPodStatusResourceType = "secretproviderclasspodstatuses"
)

var (
	Clients             = kube.GetClients()
	Pod                 = "test-reloader-" + RandSeq(5)
	Namespace           = "test-reloader-" + RandSeq(5)
	ConfigmapNamePrefix = "testconfigmap-reloader"
	SecretNamePrefix    = "testsecret-reloader"
	Data                = "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	NewData             = "dGVzdE5ld1NlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	UpdatedData         = "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy"
	Collectors          = metrics.NewCollectors()
	SleepDuration       = 3 * time.Second
)

// CreateNamespace creates namespace for testing
func CreateNamespace(namespace string, client kubernetes.Interface) {
	_, err := client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
	if err != nil {
		logrus.Fatalf("Failed to create namespace for testing %v", err)
	} else {
		logrus.Infof("Creating namespace for testing = %s", namespace)
	}
}

// DeleteNamespace deletes namespace for testing
func DeleteNamespace(namespace string, client kubernetes.Interface) {
	err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if err != nil {
		logrus.Fatalf("Failed to delete namespace that was created for testing %v", err)
	} else {
		logrus.Infof("Deleting namespace for testing = %s", namespace)
	}
}

func getObjectMeta(namespace string, name string, autoReload bool, secretAutoReload bool, configmapAutoReload bool, secretproviderclass bool, extraAnnotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      map[string]string{"firstLabel": "temp"},
		Annotations: getAnnotations(name, autoReload, secretAutoReload, configmapAutoReload, secretproviderclass, extraAnnotations),
	}
}

func getAnnotations(name string, autoReload bool, secretAutoReload bool, configmapAutoReload bool, secretproviderclass bool, extraAnnotations map[string]string) map[string]string {
	annotations := make(map[string]string)
	if autoReload {
		annotations[options.ReloaderAutoAnnotation] = "true"
	}
	if secretAutoReload {
		annotations[options.SecretReloaderAutoAnnotation] = "true"
	}
	if configmapAutoReload {
		annotations[options.ConfigmapReloaderAutoAnnotation] = "true"
	}
	if secretproviderclass {
		annotations[options.SecretProviderClassReloaderAutoAnnotation] = "true"
	}

	if len(annotations) == 0 {
		annotations = map[string]string{
			options.ConfigmapUpdateOnChangeAnnotation:           name,
			options.SecretUpdateOnChangeAnnotation:              name,
			options.SecretProviderClassUpdateOnChangeAnnotation: name,
		}
	}
	for k, v := range extraAnnotations {
		annotations[k] = v
	}
	return annotations
}

func getEnvVarSources(name string) []v1.EnvFromSource {
	return []v1.EnvFromSource{
		{
			ConfigMapRef: &v1.ConfigMapEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: name,
				},
			},
		},
		{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: name,
				},
			},
		},
	}
}

func getVolumes(name string) []v1.Volume {
	return []v1.Volume{
		{
			Name: "projectedconfigmap",
			VolumeSource: v1.VolumeSource{
				Projected: &v1.ProjectedVolumeSource{
					Sources: []v1.VolumeProjection{
						{
							ConfigMap: &v1.ConfigMapProjection{
								LocalObjectReference: v1.LocalObjectReference{
									Name: name,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "projectedsecret",
			VolumeSource: v1.VolumeSource{
				Projected: &v1.ProjectedVolumeSource{
					Sources: []v1.VolumeProjection{
						{
							Secret: &v1.SecretProjection{
								LocalObjectReference: v1.LocalObjectReference{
									Name: name,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "configmap",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: name,
					},
				},
			},
		},
		{
			Name: "secret",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: name,
				},
			},
		},
		{
			Name: "secretproviderclass",
			VolumeSource: v1.VolumeSource{
				CSI: &v1.CSIVolumeSource{
					Driver:           "secrets-store.csi.k8s.io",
					VolumeAttributes: map[string]string{"secretProviderClass": name},
				},
			},
		},
	}
}

func getVolumeMounts() []v1.VolumeMount {
	return []v1.VolumeMount{
		{
			MountPath: "etc/config",
			Name:      "configmap",
		},
		{
			MountPath: "etc/sec",
			Name:      "secret",
		},
		{
			MountPath: "etc/spc",
			Name:      "secretproviderclass",
		},
		{
			MountPath: "etc/projectedconfig",
			Name:      "projectedconfigmap",
		},
		{
			MountPath: "etc/projectedsec",
			Name:      "projectedsecret",
		},
	}
}

func getPodTemplateSpecWithEnvVars(name string) v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"secondLabel": "temp"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Image: "tutum/hello-world",
					Name:  name,
					Env: []v1.EnvVar{
						{
							Name:  "BUCKET_NAME",
							Value: "test",
						},
						{
							Name: "CONFIGMAP_" + util.ConvertToEnvVarName(name),
							ValueFrom: &v1.EnvVarSource{
								ConfigMapKeyRef: &v1.ConfigMapKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: name,
									},
									Key: "test.url",
								},
							},
						},
						{
							Name: "SECRET_" + util.ConvertToEnvVarName(name),
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: name,
									},
									Key: "test.url",
								},
							},
						},
					},
				},
			},
		},
	}
}

func getPodTemplateSpecWithEnvVarSources(name string) v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"secondLabel": "temp"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Image:   "tutum/hello-world",
					Name:    name,
					EnvFrom: getEnvVarSources(name),
				},
			},
		},
	}
}

func getPodTemplateSpecWithVolumes(name string) v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"secondLabel": "temp"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Image: "tutum/hello-world",
					Name:  name,
					Env: []v1.EnvVar{
						{
							Name:  "BUCKET_NAME",
							Value: "test",
						},
					},
					VolumeMounts: getVolumeMounts(),
				},
			},
			Volumes: getVolumes(name),
		},
	}
}

func getPodTemplateSpecWithInitContainer(name string) v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"secondLabel": "temp"},
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{
					Image:        "busybox",
					Name:         "busyBox",
					VolumeMounts: getVolumeMounts(),
				},
			},
			Containers: []v1.Container{
				{
					Image: "tutum/hello-world",
					Name:  name,
					Env: []v1.EnvVar{
						{
							Name:  "BUCKET_NAME",
							Value: "test",
						},
					},
				},
			},
			Volumes: getVolumes(name),
		},
	}
}

func getPodTemplateSpecWithInitContainerAndEnv(name string) v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"secondLabel": "temp"},
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{
					Image:   "busybox",
					Name:    "busyBox",
					EnvFrom: getEnvVarSources(name),
				},
			},
			Containers: []v1.Container{
				{
					Image: "tutum/hello-world",
					Name:  name,
					Env: []v1.EnvVar{
						{
							Name:  "BUCKET_NAME",
							Value: "test",
						},
					},
				},
			},
		},
	}
}

// GetDeployment provides deployment for testing
func GetDeployment(namespace string, deploymentName string) *appsv1.Deployment {
	replicaset := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, false, false, false, false, map[string]string{}),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(deploymentName),
		},
	}
}

// GetDeploymentConfig provides deployment for testing
func GetDeploymentConfig(namespace string, deploymentConfigName string) *openshiftv1.DeploymentConfig {
	replicaset := int32(1)
	podTemplateSpecWithVolume := getPodTemplateSpecWithVolumes(deploymentConfigName)
	return &openshiftv1.DeploymentConfig{
		ObjectMeta: getObjectMeta(namespace, deploymentConfigName, false, false, false, false, map[string]string{}),
		Spec: openshiftv1.DeploymentConfigSpec{
			Replicas: replicaset,
			Strategy: openshiftv1.DeploymentStrategy{
				Type: openshiftv1.DeploymentStrategyTypeRolling,
			},
			Template: &podTemplateSpecWithVolume,
		},
	}
}

// GetDeploymentWithInitContainer provides deployment with init container and volumeMounts
func GetDeploymentWithInitContainer(namespace string, deploymentName string) *appsv1.Deployment {
	replicaset := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, false, false, false, false, map[string]string{}),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithInitContainer(deploymentName),
		},
	}
}

// GetDeploymentWithInitContainerAndEnv provides deployment with init container and EnvSource
func GetDeploymentWithInitContainerAndEnv(namespace string, deploymentName string) *appsv1.Deployment {
	replicaset := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, true, false, false, false, map[string]string{}),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithInitContainerAndEnv(deploymentName),
		},
	}
}

func GetDeploymentWithEnvVars(namespace string, deploymentName string) *appsv1.Deployment {
	replicaset := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, true, false, false, false, map[string]string{}),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithEnvVars(deploymentName),
		},
	}
}

func GetDeploymentConfigWithEnvVars(namespace string, deploymentConfigName string) *openshiftv1.DeploymentConfig {
	replicaset := int32(1)
	podTemplateSpecWithEnvVars := getPodTemplateSpecWithEnvVars(deploymentConfigName)
	return &openshiftv1.DeploymentConfig{
		ObjectMeta: getObjectMeta(namespace, deploymentConfigName, false, false, false, false, map[string]string{}),
		Spec: openshiftv1.DeploymentConfigSpec{
			Replicas: replicaset,
			Strategy: openshiftv1.DeploymentStrategy{
				Type: openshiftv1.DeploymentStrategyTypeRolling,
			},
			Template: &podTemplateSpecWithEnvVars,
		},
	}
}

func GetDeploymentWithEnvVarSources(namespace string, deploymentName string) *appsv1.Deployment {
	replicaset := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, true, false, false, false, map[string]string{}),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithEnvVarSources(deploymentName),
		},
	}
}

func GetDeploymentWithPodAnnotations(namespace string, deploymentName string, both bool) *appsv1.Deployment {
	replicaset := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, false, false, false, false, map[string]string{}),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithEnvVarSources(deploymentName),
		},
	}
	if !both {
		deployment.Annotations = nil
	}
	deployment.Spec.Template.Annotations = getAnnotations(deploymentName, true, false, false, false, map[string]string{})
	return deployment
}

func GetDeploymentWithTypedAutoAnnotation(namespace string, deploymentName string, resourceType string) *appsv1.Deployment {
	replicaset := int32(1)
	var objectMeta metav1.ObjectMeta
	switch resourceType {
	case SecretResourceType:
		objectMeta = getObjectMeta(namespace, deploymentName, false, true, false, false, map[string]string{})
	case ConfigmapResourceType:
		objectMeta = getObjectMeta(namespace, deploymentName, false, false, true, false, map[string]string{})
	case SecretProviderClassPodStatusResourceType:
		objectMeta = getObjectMeta(namespace, deploymentName, false, false, false, true, map[string]string{})
	}

	return &appsv1.Deployment{
		ObjectMeta: objectMeta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(deploymentName),
		},
	}
}

func GetDeploymentWithExcludeAnnotation(namespace string, deploymentName string, resourceType string) *appsv1.Deployment {
	replicaset := int32(1)

	annotation := map[string]string{}

	switch resourceType {
	case SecretResourceType:
		annotation[options.SecretExcludeReloaderAnnotation] = deploymentName
	case ConfigmapResourceType:
		annotation[options.ConfigmapExcludeReloaderAnnotation] = deploymentName
	case SecretProviderClassPodStatusResourceType:
		annotation[options.SecretProviderClassExcludeReloaderAnnotation] = deploymentName
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        deploymentName,
			Namespace:   namespace,
			Labels:      map[string]string{"firstLabel": "temp"},
			Annotations: annotation,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(deploymentName),
		},
	}
}

// GetDaemonSet provides daemonset for testing
func GetDaemonSet(namespace string, daemonsetName string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: getObjectMeta(namespace, daemonsetName, false, false, false, false, map[string]string{}),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(daemonsetName),
		},
	}
}

func GetDaemonSetWithEnvVars(namespace string, daemonSetName string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: getObjectMeta(namespace, daemonSetName, true, false, false, false, map[string]string{}),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: getPodTemplateSpecWithEnvVars(daemonSetName),
		},
	}
}

// GetStatefulSet provides statefulset for testing
func GetStatefulSet(namespace string, statefulsetName string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: getObjectMeta(namespace, statefulsetName, false, false, false, false, map[string]string{}),
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(statefulsetName),
		},
	}
}

// GetStatefulSet provides statefulset for testing
func GetStatefulSetWithEnvVar(namespace string, statefulsetName string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: getObjectMeta(namespace, statefulsetName, true, false, false, false, map[string]string{}),
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: getPodTemplateSpecWithEnvVars(statefulsetName),
		},
	}
}

// GetConfigmap provides configmap for testing
func GetConfigmap(namespace string, configmapName string, testData string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
		},
		Data: map[string]string{"test.url": testData},
	}
}

func GetSecretProviderClass(namespace string, secretProviderClassName string, data string) *csiv1.SecretProviderClass {
	return &csiv1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretProviderClassName,
			Namespace: namespace,
		},
		Spec: csiv1.SecretProviderClassSpec{
			Provider: "Test",
			Parameters: map[string]string{
				"parameter1": data,
			},
		},
	}
}

func GetSecretProviderClassPodStatus(namespace string, secretProviderClassPodStatusName string, data string) *csiv1.SecretProviderClassPodStatus {
	return &csiv1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretProviderClassPodStatusName,
			Namespace: namespace,
		},
		Status: csiv1.SecretProviderClassPodStatusStatus{
			PodName:                 "test123",
			SecretProviderClassName: secretProviderClassPodStatusName,
			TargetPath:              "/var/lib/kubelet/d8771ddf-935a-4199-a20b-f35f71c1d9e7/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			Mounted:                 true,
			Objects: []csiv1.SecretProviderClassObject{
				{
					ID:      "parameter1",
					Version: data,
				},
			},
		},
	}
}

// GetConfigmapWithUpdatedLabel provides configmap for testing
func GetConfigmapWithUpdatedLabel(namespace string, configmapName string, testLabel string, testData string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": testLabel},
		},
		Data: map[string]string{"test.url": testData},
	}
}

// GetSecret provides secret for testing
func GetSecret(namespace string, secretName string, data string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
		},
		Data: map[string][]byte{"test.url": []byte(data)},
	}
}

func GetCronJob(namespace string, cronJobName string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: getObjectMeta(namespace, cronJobName, false, false, false, false, map[string]string{}),
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *", // Run every 5 minutes
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"secondLabel": "temp"},
					},
					Template: getPodTemplateSpecWithVolumes(cronJobName),
				},
			},
		},
	}
}

func GetJob(namespace string, jobName string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: getObjectMeta(namespace, jobName, false, false, false, false, map[string]string{}),
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Template: getPodTemplateSpecWithVolumes(jobName),
		},
	}
}

func GetCronJobWithEnvVar(namespace string, cronJobName string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: getObjectMeta(namespace, cronJobName, true, false, false, false, map[string]string{}),
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *", // Run every 5 minutes
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"secondLabel": "temp"},
					},
					Template: getPodTemplateSpecWithEnvVars(cronJobName),
				},
			},
		},
	}
}

func GetJobWithEnvVar(namespace string, jobName string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: getObjectMeta(namespace, jobName, true, false, false, false, map[string]string{}),
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Template: getPodTemplateSpecWithEnvVars(jobName),
		},
	}
}

// GetSecretWithUpdatedLabel provides secret for testing
func GetSecretWithUpdatedLabel(namespace string, secretName string, label string, data string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": label},
		},
		Data: map[string][]byte{"test.url": []byte(data)},
	}
}

// GetResourceSHAFromEnvVar returns the SHA value of given environment variable
func GetResourceSHAFromEnvVar(containers []v1.Container, envVar string) string {
	for i := range containers {
		envs := containers[i].Env
		for j := range envs {
			if envs[j].Name == envVar {
				return envs[j].Value
			}
		}
	}
	return ""
}

// GetResourceSHAFromAnnotation returns the SHA value of given environment variable
func GetResourceSHAFromAnnotation(podAnnotations map[string]string) string {
	lastReloadedResourceName := fmt.Sprintf("%s/%s",
		constants.ReloaderAnnotationPrefix,
		constants.LastReloadedFromAnnotation,
	)

	annotationJson, ok := podAnnotations[lastReloadedResourceName]
	if !ok {
		return ""
	}

	var last common.ReloadSource
	bytes := []byte(annotationJson)
	err := json.Unmarshal(bytes, &last)
	if err != nil {
		return ""
	}

	return last.Hash
}

// ConvertResourceToSHA generates SHA from secret, configmap or secretproviderclasspodstatus data
func ConvertResourceToSHA(resourceType string, namespace string, resourceName string, data string) string {
	values := []string{}
	switch resourceType {
	case SecretResourceType:
		secret := GetSecret(namespace, resourceName, data)
		for k, v := range secret.Data {
			values = append(values, k+"="+string(v[:]))
		}
	case ConfigmapResourceType:
		configmap := GetConfigmap(namespace, resourceName, data)
		for k, v := range configmap.Data {
			values = append(values, k+"="+v)
		}
	case SecretProviderClassPodStatusResourceType:
		secretproviderclasspodstatus := GetSecretProviderClassPodStatus(namespace, resourceName, data)
		for _, v := range secretproviderclasspodstatus.Status.Objects {
			values = append(values, v.ID+"="+v.Version)
		}
		values = append(values, "SecretProviderClassName="+secretproviderclasspodstatus.Status.SecretProviderClassName)
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

// CreateConfigMap creates a configmap in given namespace and returns the ConfigMapInterface
func CreateConfigMap(client kubernetes.Interface, namespace string, configmapName string, data string) (core_v1.ConfigMapInterface, error) {
	logrus.Infof("Creating configmap")
	configmapClient := client.CoreV1().ConfigMaps(namespace)
	_, err := configmapClient.Create(context.TODO(), GetConfigmap(namespace, configmapName, data), metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return configmapClient, err
}

// CreateSecretProviderClass creates a SecretProviderClass in given namespace and returns the SecretProviderClassInterface
func CreateSecretProviderClass(client csiclient.Interface, namespace string, secretProviderClassName string, data string) (csiclient_v1.SecretProviderClassInterface, error) {
	logrus.Infof("Creating SecretProviderClass")
	secretProviderClassClient := client.SecretsstoreV1().SecretProviderClasses(namespace)
	_, err := secretProviderClassClient.Create(context.TODO(), GetSecretProviderClass(namespace, secretProviderClassName, data), metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return secretProviderClassClient, err
}

// CreateSecretProviderClassPodStatus creates a SecretProviderClassPodStatus in given namespace and returns the SecretProviderClassPodStatusInterface
func CreateSecretProviderClassPodStatus(client csiclient.Interface, namespace string, secretProviderClassPodStatusName string, data string) (csiclient_v1.SecretProviderClassPodStatusInterface, error) {
	logrus.Infof("Creating SecretProviderClassPodStatus")
	secretProviderClassPodStatusClient := client.SecretsstoreV1().SecretProviderClassPodStatuses(namespace)
	secretProviderClassPodStatus := GetSecretProviderClassPodStatus(namespace, secretProviderClassPodStatusName, data)
	_, err := secretProviderClassPodStatusClient.Create(context.TODO(), secretProviderClassPodStatus, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return secretProviderClassPodStatusClient, err
}

// CreateSecret creates a secret in given namespace and returns the SecretInterface
func CreateSecret(client kubernetes.Interface, namespace string, secretName string, data string) (core_v1.SecretInterface, error) {
	logrus.Infof("Creating secret")
	secretClient := client.CoreV1().Secrets(namespace)
	_, err := secretClient.Create(context.TODO(), GetSecret(namespace, secretName, data), metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return secretClient, err
}

// CreateDeployment creates a deployment in given namespace and returns the Deployment
func CreateDeployment(client kubernetes.Interface, deploymentName string, namespace string, volumeMount bool) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	var deploymentObj *appsv1.Deployment
	if volumeMount {
		deploymentObj = GetDeployment(namespace, deploymentName)
	} else {
		deploymentObj = GetDeploymentWithEnvVars(namespace, deploymentName)
	}
	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return deployment, err
}

// CreateDeployment creates a deployment in given namespace and returns the Deployment
func CreateDeploymentWithAnnotations(client kubernetes.Interface, deploymentName string, namespace string, additionalAnnotations map[string]string, volumeMount bool) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	var deploymentObj *appsv1.Deployment
	if volumeMount {
		deploymentObj = GetDeployment(namespace, deploymentName)
	} else {
		deploymentObj = GetDeploymentWithEnvVars(namespace, deploymentName)
	}

	for annotationKey, annotationValue := range additionalAnnotations {
		deploymentObj.Annotations[annotationKey] = annotationValue
	}

	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return deployment, err
}

// CreateDeploymentConfig creates a deploymentConfig in given namespace and returns the DeploymentConfig
func CreateDeploymentConfig(client appsclient.Interface, deploymentName string, namespace string, volumeMount bool) (*openshiftv1.DeploymentConfig, error) {
	logrus.Infof("Creating DeploymentConfig")
	deploymentConfigsClient := client.AppsV1().DeploymentConfigs(namespace)
	var deploymentConfigObj *openshiftv1.DeploymentConfig
	if volumeMount {
		deploymentConfigObj = GetDeploymentConfig(namespace, deploymentName)
	} else {
		deploymentConfigObj = GetDeploymentConfigWithEnvVars(namespace, deploymentName)
	}
	deploymentConfig, err := deploymentConfigsClient.Create(context.TODO(), deploymentConfigObj, metav1.CreateOptions{})
	time.Sleep(5 * time.Second)
	return deploymentConfig, err
}

// CreateDeploymentWithInitContainer creates a deployment in given namespace with init container and returns the Deployment
func CreateDeploymentWithInitContainer(client kubernetes.Interface, deploymentName string, namespace string, volumeMount bool) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	var deploymentObj *appsv1.Deployment
	if volumeMount {
		deploymentObj = GetDeploymentWithInitContainer(namespace, deploymentName)
	} else {
		deploymentObj = GetDeploymentWithInitContainerAndEnv(namespace, deploymentName)
	}
	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return deployment, err
}

// CreateDeploymentWithEnvVarSource creates a deployment in given namespace and returns the Deployment
func CreateDeploymentWithEnvVarSource(client kubernetes.Interface, deploymentName string, namespace string) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	deploymentObj := GetDeploymentWithEnvVarSources(namespace, deploymentName)
	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return deployment, err

}

// CreateDeploymentWithPodAnnotations creates a deployment in given namespace and returns the Deployment
func CreateDeploymentWithPodAnnotations(client kubernetes.Interface, deploymentName string, namespace string, both bool) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	deploymentObj := GetDeploymentWithPodAnnotations(namespace, deploymentName, both)
	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return deployment, err
}

// CreateDeploymentWithEnvVarSourceAndAnnotations returns a deployment in given
// namespace with given annotations.
func CreateDeploymentWithEnvVarSourceAndAnnotations(client kubernetes.Interface, deploymentName string, namespace string, annotations map[string]string) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	deploymentObj := GetDeploymentWithEnvVarSources(namespace, deploymentName)
	deploymentObj.Annotations = annotations
	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return deployment, err
}

// CreateDeploymentWithTypedAutoAnnotation creates a deployment in given namespace and returns the Deployment with typed auto annotation
func CreateDeploymentWithTypedAutoAnnotation(client kubernetes.Interface, deploymentName string, namespace string, resourceType string) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	deploymentObj := GetDeploymentWithTypedAutoAnnotation(namespace, deploymentName, resourceType)
	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return deployment, err
}

// CreateDeploymentWithExcludeAnnotation creates a deployment in given namespace and returns the Deployment with typed auto annotation
func CreateDeploymentWithExcludeAnnotation(client kubernetes.Interface, deploymentName string, namespace string, resourceType string) (*appsv1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.AppsV1().Deployments(namespace)
	deploymentObj := GetDeploymentWithExcludeAnnotation(namespace, deploymentName, resourceType)
	deployment, err := deploymentClient.Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	return deployment, err
}

// CreateDaemonSet creates a deployment in given namespace and returns the DaemonSet
func CreateDaemonSet(client kubernetes.Interface, daemonsetName string, namespace string, volumeMount bool) (*appsv1.DaemonSet, error) {
	logrus.Infof("Creating DaemonSet")
	daemonsetClient := client.AppsV1().DaemonSets(namespace)
	var daemonsetObj *appsv1.DaemonSet
	if volumeMount {
		daemonsetObj = GetDaemonSet(namespace, daemonsetName)
	} else {
		daemonsetObj = GetDaemonSetWithEnvVars(namespace, daemonsetName)
	}
	daemonset, err := daemonsetClient.Create(context.TODO(), daemonsetObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return daemonset, err
}

// CreateStatefulSet creates a deployment in given namespace and returns the StatefulSet
func CreateStatefulSet(client kubernetes.Interface, statefulsetName string, namespace string, volumeMount bool) (*appsv1.StatefulSet, error) {
	logrus.Infof("Creating StatefulSet")
	statefulsetClient := client.AppsV1().StatefulSets(namespace)
	var statefulsetObj *appsv1.StatefulSet
	if volumeMount {
		statefulsetObj = GetStatefulSet(namespace, statefulsetName)
	} else {
		statefulsetObj = GetStatefulSetWithEnvVar(namespace, statefulsetName)
	}
	statefulset, err := statefulsetClient.Create(context.TODO(), statefulsetObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return statefulset, err
}

// CreateCronJob creates a cronjob in given namespace and returns the CronJob
func CreateCronJob(client kubernetes.Interface, cronJobName string, namespace string, volumeMount bool) (*batchv1.CronJob, error) {
	logrus.Infof("Creating CronJob")
	cronJobClient := client.BatchV1().CronJobs(namespace)
	var cronJobObj *batchv1.CronJob
	if volumeMount {
		cronJobObj = GetCronJob(namespace, cronJobName)
	} else {
		cronJobObj = GetCronJobWithEnvVar(namespace, cronJobName)
	}
	cronJob, err := cronJobClient.Create(context.TODO(), cronJobObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return cronJob, err
}

// CreateJob creates a job in given namespace and returns the Job
func CreateJob(client kubernetes.Interface, jobName string, namespace string, volumeMount bool) (*batchv1.Job, error) {
	logrus.Infof("Creating Job")
	jobClient := client.BatchV1().Jobs(namespace)
	var jobObj *batchv1.Job
	if volumeMount {
		jobObj = GetJob(namespace, jobName)
	} else {
		jobObj = GetJobWithEnvVar(namespace, jobName)
	}
	job, err := jobClient.Create(context.TODO(), jobObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return job, err
}

// DeleteDeployment creates a deployment in given namespace and returns the error if any
func DeleteDeployment(client kubernetes.Interface, namespace string, deploymentName string) error {
	logrus.Infof("Deleting Deployment")
	deploymentError := client.AppsV1().Deployments(namespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return deploymentError
}

// DeleteDeploymentConfig deletes a deploymentConfig in given namespace and returns the error if any
func DeleteDeploymentConfig(client appsclient.Interface, namespace string, deploymentConfigName string) error {
	logrus.Infof("Deleting DeploymentConfig")
	deploymentConfigError := client.AppsV1().DeploymentConfigs(namespace).Delete(context.TODO(), deploymentConfigName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return deploymentConfigError
}

// DeleteDaemonSet creates a daemonset in given namespace and returns the error if any
func DeleteDaemonSet(client kubernetes.Interface, namespace string, daemonsetName string) error {
	logrus.Infof("Deleting DaemonSet %s", daemonsetName)
	daemonsetError := client.AppsV1().DaemonSets(namespace).Delete(context.TODO(), daemonsetName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return daemonsetError
}

// DeleteStatefulSet creates a statefulset in given namespace and returns the error if any
func DeleteStatefulSet(client kubernetes.Interface, namespace string, statefulsetName string) error {
	logrus.Infof("Deleting StatefulSet %s", statefulsetName)
	statefulsetError := client.AppsV1().StatefulSets(namespace).Delete(context.TODO(), statefulsetName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return statefulsetError
}

// DeleteCronJob deletes a cronJob in given namespace and returns the error if any
func DeleteCronJob(client kubernetes.Interface, namespace string, cronJobName string) error {
	logrus.Infof("Deleting CronJob %s", cronJobName)
	cronJobError := client.BatchV1().CronJobs(namespace).Delete(context.TODO(), cronJobName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return cronJobError
}

// Deleteob deletes a job in given namespace and returns the error if any
func DeleteJob(client kubernetes.Interface, namespace string, jobName string) error {
	logrus.Infof("Deleting Job %s", jobName)
	jobError := client.BatchV1().Jobs(namespace).Delete(context.TODO(), jobName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return jobError
}

// UpdateConfigMap updates a configmap in given namespace and returns the error if any
func UpdateConfigMap(configmapClient core_v1.ConfigMapInterface, namespace string, configmapName string, label string, data string) error {
	logrus.Infof("Updating configmap %q.\n", configmapName)
	var configmap *v1.ConfigMap
	if label != "" {
		configmap = GetConfigmapWithUpdatedLabel(namespace, configmapName, label, data)
	} else {
		configmap = GetConfigmap(namespace, configmapName, data)
	}
	_, updateErr := configmapClient.Update(context.TODO(), configmap, metav1.UpdateOptions{})
	time.Sleep(3 * time.Second)
	return updateErr
}

// UpdateSecret updates a secret in given namespace and returns the error if any
func UpdateSecret(secretClient core_v1.SecretInterface, namespace string, secretName string, label string, data string) error {
	logrus.Infof("Updating secret %q.\n", secretName)
	var secret *v1.Secret
	if label != "" {
		secret = GetSecretWithUpdatedLabel(namespace, secretName, label, data)
	} else {
		secret = GetSecret(namespace, secretName, data)
	}
	_, updateErr := secretClient.Update(context.TODO(), secret, metav1.UpdateOptions{})
	time.Sleep(3 * time.Second)
	return updateErr
}

// UpdateSecretProviderClassPodStatus updates a secretproviderclasspodstatus in given namespace and returns the error if any
func UpdateSecretProviderClassPodStatus(spcpsClient csiclient_v1.SecretProviderClassPodStatusInterface, namespace string, spcpsName string, label string, data string) error {
	logrus.Infof("Updating secretproviderclasspodstatus %q.\n", spcpsName)
	updatedStatus := GetSecretProviderClassPodStatus(namespace, spcpsName, data).Status
	secretproviderclasspodstatus, err := spcpsClient.Get(context.TODO(), spcpsName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	secretproviderclasspodstatus.Status = updatedStatus
	if label != "" {
		labels := secretproviderclasspodstatus.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["firstLabel"] = label
	}
	_, updateErr := spcpsClient.Update(context.TODO(), secretproviderclasspodstatus, metav1.UpdateOptions{})
	time.Sleep(3 * time.Second)
	return updateErr
}

// DeleteConfigMap deletes a configmap in given namespace and returns the error if any
func DeleteConfigMap(client kubernetes.Interface, namespace string, configmapName string) error {
	logrus.Infof("Deleting configmap %q.\n", configmapName)
	err := client.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), configmapName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return err
}

// DeleteSecret deletes a secret in given namespace and returns the error if any
func DeleteSecret(client kubernetes.Interface, namespace string, secretName string) error {
	logrus.Infof("Deleting secret %q.\n", secretName)
	err := client.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return err
}

// DeleteSecretProviderClass deletes a secretproviderclass in given namespace and returns the error if any
func DeleteSecretProviderClass(client csiclient.Interface, namespace string, secretProviderClassName string) error {
	logrus.Infof("Deleting secretproviderclass %q.\n", secretProviderClassName)
	err := client.SecretsstoreV1().SecretProviderClasses(namespace).Delete(context.TODO(), secretProviderClassName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return err
}

// DeleteSecretProviderClassPodStatus deletes a secretproviderclasspodstatus in given namespace and returns the error if any
func DeleteSecretProviderClassPodStatus(client csiclient.Interface, namespace string, secretProviderClassPodStatusName string) error {
	logrus.Infof("Deleting secretproviderclasspodstatus %q.\n", secretProviderClassPodStatusName)
	err := client.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Delete(context.TODO(), secretProviderClassPodStatusName, metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	return err
}

// RandSeq generates a random sequence
func RandSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// VerifyResourceEnvVarUpdate verifies whether the rolling upgrade happened or not
func VerifyResourceEnvVarUpdate(clients kube.Clients, config common.Config, envVarPostfix string, upgradeFuncs callbacks.RollingUpgradeFuncs) bool {
	items := upgradeFuncs.ItemsFunc(clients, config.Namespace)
	for _, i := range items {
		containers := upgradeFuncs.ContainersFunc(i)
		accessor, err := meta.Accessor(i)
		if err != nil {
			return false
		}
		annotations := accessor.GetAnnotations()
		// match statefulsets with the correct annotation
		annotationValue := annotations[config.Annotation]
		searchAnnotationValue := annotations[options.AutoSearchAnnotation]
		reloaderEnabledValue := annotations[options.ReloaderAutoAnnotation]
		typedAutoAnnotationEnabledValue := annotations[config.TypedAutoAnnotation]
		reloaderEnabled, err := strconv.ParseBool(reloaderEnabledValue)
		typedAutoAnnotationEnabled, errTyped := strconv.ParseBool(typedAutoAnnotationEnabledValue)
		matches := false
		if err == nil && reloaderEnabled || errTyped == nil && typedAutoAnnotationEnabled {
			matches = true
		} else if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				value = strings.Trim(value, " ")
				if value == config.ResourceName {
					matches = true
					break
				}
			}
		} else if searchAnnotationValue == "true" {
			if config.ResourceAnnotations[options.SearchMatchAnnotation] == "true" {
				matches = true
			}
		}

		if matches {
			envName := constants.EnvVarPrefix + util.ConvertToEnvVarName(config.ResourceName) + "_" + envVarPostfix
			updated := GetResourceSHAFromEnvVar(containers, envName)
			if updated == config.SHAValue {
				return true
			}
		}
	}
	return false
}

// VerifyResourceEnvVarRemoved verifies whether the rolling upgrade happened or not and all Envvars SKAKATER_name_CONFIGMAP/SECRET are removed
func VerifyResourceEnvVarRemoved(clients kube.Clients, config common.Config, envVarPostfix string, upgradeFuncs callbacks.RollingUpgradeFuncs) bool {
	items := upgradeFuncs.ItemsFunc(clients, config.Namespace)
	for _, i := range items {
		containers := upgradeFuncs.ContainersFunc(i)
		accessor, err := meta.Accessor(i)
		if err != nil {
			return false
		}

		annotations := accessor.GetAnnotations()
		// match statefulsets with the correct annotation

		annotationValue := annotations[config.Annotation]
		searchAnnotationValue := annotations[options.AutoSearchAnnotation]
		reloaderEnabledValue := annotations[options.ReloaderAutoAnnotation]
		typedAutoAnnotationEnabledValue := annotations[config.TypedAutoAnnotation]
		reloaderEnabled, err := strconv.ParseBool(reloaderEnabledValue)
		typedAutoAnnotationEnabled, errTyped := strconv.ParseBool(typedAutoAnnotationEnabledValue)

		matches := false
		if err == nil && reloaderEnabled || errTyped == nil && typedAutoAnnotationEnabled {
			matches = true
		} else if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				value = strings.Trim(value, " ")
				if value == config.ResourceName {
					matches = true
					break
				}
			}
		} else if searchAnnotationValue == "true" {
			if config.ResourceAnnotations[options.SearchMatchAnnotation] == "true" {
				matches = true
			}
		}

		if matches {
			envName := constants.EnvVarPrefix + util.ConvertToEnvVarName(config.ResourceName) + "_" + envVarPostfix
			value := GetResourceSHAFromEnvVar(containers, envName)
			if value == "" {
				return true
			}
		}
	}
	return false
}

// VerifyResourceAnnotationUpdate verifies whether the rolling upgrade happened or not
func VerifyResourceAnnotationUpdate(clients kube.Clients, config common.Config, upgradeFuncs callbacks.RollingUpgradeFuncs) bool {
	items := upgradeFuncs.ItemsFunc(clients, config.Namespace)
	for _, i := range items {
		podAnnotations := upgradeFuncs.PodAnnotationsFunc(i)
		accessor, err := meta.Accessor(i)
		if err != nil {
			return false
		}
		annotations := accessor.GetAnnotations()
		// match statefulsets with the correct annotation
		annotationValue := annotations[config.Annotation]
		searchAnnotationValue := annotations[options.AutoSearchAnnotation]
		reloaderEnabledValue := annotations[options.ReloaderAutoAnnotation]
		typedAutoAnnotationEnabledValue := annotations[config.TypedAutoAnnotation]
		reloaderEnabled, _ := strconv.ParseBool(reloaderEnabledValue)
		typedAutoAnnotationEnabled, _ := strconv.ParseBool(typedAutoAnnotationEnabledValue)
		matches := false
		if reloaderEnabled || typedAutoAnnotationEnabled || reloaderEnabledValue == "" && typedAutoAnnotationEnabledValue == "" && options.AutoReloadAll {
			matches = true
		} else if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				value = strings.Trim(value, " ")
				if value == config.ResourceName {
					matches = true
					break
				}
			}
		} else if searchAnnotationValue == "true" {
			if config.ResourceAnnotations[options.SearchMatchAnnotation] == "true" {
				matches = true
			}
		}

		if matches {
			updated := GetResourceSHAFromAnnotation(podAnnotations)
			if updated == config.SHAValue {
				return true
			}
		}
	}
	return false
}

func GetSHAfromEmptyData() string {
	// Use a special marker that represents "deleted" or "empty" state
	// This ensures we have a distinct, deterministic hash for the delete strategy
	// Note: We could use GenerateSHA("") which now returns a hash, but using a marker
	// makes the intent clearer and avoids potential confusion with actual empty data
	return crypto.GenerateSHA("__RELOADER_EMPTY_DELETE_MARKER__")
}

// GetRollout provides rollout for testing
func GetRollout(namespace string, rolloutName string, annotations map[string]string) *argorolloutv1alpha1.Rollout {
	replicaset := int32(1)
	return &argorolloutv1alpha1.Rollout{
		ObjectMeta: getObjectMeta(namespace, rolloutName, false, false, false, false, annotations),
		Spec: argorolloutv1alpha1.RolloutSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secondLabel": "temp"},
			},
			Replicas: &replicaset,
			Template: getPodTemplateSpecWithVolumes(rolloutName),
		},
	}
}

// CreateRollout creates a rolout in given namespace and returns the Rollout
func CreateRollout(client argorollout.Interface, rolloutName string, namespace string, annotations map[string]string) (*argorolloutv1alpha1.Rollout, error) {
	logrus.Infof("Creating Rollout")
	rolloutClient := client.ArgoprojV1alpha1().Rollouts(namespace)
	rolloutObj := GetRollout(namespace, rolloutName, annotations)
	rollout, err := rolloutClient.Create(context.TODO(), rolloutObj, metav1.CreateOptions{})
	time.Sleep(3 * time.Second)
	return rollout, err
}
