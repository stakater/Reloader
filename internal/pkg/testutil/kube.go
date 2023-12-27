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

	openshiftv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	letters = []rune("abcdefghijklmnopqrstuvwxyz")
	// ConfigmapResourceType is a resource type which controller watches for changes
	ConfigmapResourceType = "configMaps"
	// SecretResourceType is a resource type which controller watches for changes
	SecretResourceType = "secrets"
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

func getObjectMeta(namespace string, name string, autoReload bool) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      map[string]string{"firstLabel": "temp"},
		Annotations: getAnnotations(name, autoReload),
	}
}

func getAnnotations(name string, autoReload bool) map[string]string {
	if autoReload {
		return map[string]string{
			options.ReloaderAutoAnnotation: "true"}
	}

	return map[string]string{
		options.ConfigmapUpdateOnChangeAnnotation: name,
		options.SecretUpdateOnChangeAnnotation:    name}
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
	}
}

func getVolumeMounts(name string) []v1.VolumeMount {
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
					VolumeMounts: getVolumeMounts(name),
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
					VolumeMounts: getVolumeMounts(name),
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
		ObjectMeta: getObjectMeta(namespace, deploymentName, false),
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
		ObjectMeta: getObjectMeta(namespace, deploymentConfigName, false),
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
		ObjectMeta: getObjectMeta(namespace, deploymentName, false),
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
		ObjectMeta: getObjectMeta(namespace, deploymentName, true),
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
		ObjectMeta: getObjectMeta(namespace, deploymentName, true),
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
		ObjectMeta: getObjectMeta(namespace, deploymentConfigName, false),
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
		ObjectMeta: getObjectMeta(namespace, deploymentName, true),
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
		ObjectMeta: getObjectMeta(namespace, deploymentName, false),
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
		deployment.ObjectMeta.Annotations = nil
	}
	deployment.Spec.Template.ObjectMeta.Annotations = getAnnotations(deploymentName, true)
	return deployment
}

// GetDaemonSet provides daemonset for testing
func GetDaemonSet(namespace string, daemonsetName string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: getObjectMeta(namespace, daemonsetName, false),
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
		ObjectMeta: getObjectMeta(namespace, daemonSetName, true),
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
		ObjectMeta: getObjectMeta(namespace, statefulsetName, false),
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
		ObjectMeta: getObjectMeta(namespace, statefulsetName, true),
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

	var last util.ReloadSource
	bytes := []byte(annotationJson)
	err := json.Unmarshal(bytes, &last)
	if err != nil {
		return ""
	}

	return last.Hash
}

// ConvertResourceToSHA generates SHA from secret or configmap data
func ConvertResourceToSHA(resourceType string, namespace string, resourceName string, data string) string {
	values := []string{}
	if resourceType == SecretResourceType {
		secret := GetSecret(namespace, resourceName, data)
		for k, v := range secret.Data {
			values = append(values, k+"="+string(v[:]))
		}
	} else if resourceType == ConfigmapResourceType {
		configmap := GetConfigmap(namespace, resourceName, data)
		for k, v := range configmap.Data {
			values = append(values, k+"="+v)
		}
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

// RandSeq generates a random sequence
func RandSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// VerifyResourceEnvVarUpdate verifies whether the rolling upgrade happened or not
func VerifyResourceEnvVarUpdate(clients kube.Clients, config util.Config, envVarPostfix string, upgradeFuncs callbacks.RollingUpgradeFuncs) bool {
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
		reloaderEnabled, err := strconv.ParseBool(reloaderEnabledValue)
		matches := false
		if err == nil && reloaderEnabled {
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

// VerifyResourceAnnotationUpdate verifies whether the rolling upgrade happened or not
func VerifyResourceAnnotationUpdate(clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs) bool {
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
		reloaderEnabled, _ := strconv.ParseBool(reloaderEnabledValue)
		matches := false
		if reloaderEnabled || reloaderEnabledValue == "" && options.AutoReloadAll {
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
