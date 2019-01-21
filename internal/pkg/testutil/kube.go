package testutil

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	v1_beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
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

func GetClient() *kubernetes.Clientset {
	newClient, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	return newClient
}

// CreateNamespace creates namespace for testing
func CreateNamespace(namespace string, client kubernetes.Interface) {
	_, err := client.CoreV1().Namespaces().Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil {
		logrus.Fatalf("Failed to create namespace for testing %v", err)
	} else {
		logrus.Infof("Creating namespace for testing = %s", namespace)
	}
}

// DeleteNamespace deletes namespace for testing
func DeleteNamespace(namespace string, client kubernetes.Interface) {
	err := client.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
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
			constants.ReloaderAutoAnnotation: "true"}
	}

	return map[string]string{
		constants.ConfigmapUpdateOnChangeAnnotation: name,
		constants.SecretUpdateOnChangeAnnotation:    name}
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
					VolumeMounts: []v1.VolumeMount{
						{
							MountPath: "etc/config",
							Name:      "configmap",
						},
						{
							MountPath: "etc/sec",
							Name:      "secret",
						},
					},
				},
			},
			Volumes: []v1.Volume{
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
			},
		},
	}
}

// GetDeployment provides deployment for testing
func GetDeployment(namespace string, deploymentName string) *v1beta1.Deployment {
	replicaset := int32(1)
	return &v1beta1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, false),
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicaset,
			Strategy: v1beta1.DeploymentStrategy{
				Type: v1beta1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(deploymentName),
		},
	}
}

func GetDeploymentWithEnvVars(namespace string, deploymentName string) *v1beta1.Deployment {
	replicaset := int32(1)
	return &v1beta1.Deployment{
		ObjectMeta: getObjectMeta(namespace, deploymentName, true),
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicaset,
			Strategy: v1beta1.DeploymentStrategy{
				Type: v1beta1.RollingUpdateDeploymentStrategyType,
			},
			Template: getPodTemplateSpecWithEnvVars(deploymentName),
		},
	}
}

// GetDaemonSet provides daemonset for testing
func GetDaemonSet(namespace string, daemonsetName string) *v1beta1.DaemonSet {
	return &v1beta1.DaemonSet{
		ObjectMeta: getObjectMeta(namespace, daemonsetName, false),
		Spec: v1beta1.DaemonSetSpec{
			UpdateStrategy: v1beta1.DaemonSetUpdateStrategy{
				Type: v1beta1.RollingUpdateDaemonSetStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(daemonsetName),
		},
	}
}

func GetDaemonSetWithEnvVars(namespace string, daemonSetName string) *v1beta1.DaemonSet {
	return &v1beta1.DaemonSet{
		ObjectMeta: getObjectMeta(namespace, daemonSetName, true),
		Spec: v1beta1.DaemonSetSpec{
			UpdateStrategy: v1beta1.DaemonSetUpdateStrategy{
				Type: v1beta1.RollingUpdateDaemonSetStrategyType,
			},
			Template: getPodTemplateSpecWithEnvVars(daemonSetName),
		},
	}
}

// GetStatefulSet provides statefulset for testing
func GetStatefulSet(namespace string, statefulsetName string) *v1_beta1.StatefulSet {
	return &v1_beta1.StatefulSet{
		ObjectMeta: getObjectMeta(namespace, statefulsetName, false),
		Spec: v1_beta1.StatefulSetSpec{
			UpdateStrategy: v1_beta1.StatefulSetUpdateStrategy{
				Type: v1_beta1.RollingUpdateStatefulSetStrategyType,
			},
			Template: getPodTemplateSpecWithVolumes(statefulsetName),
		},
	}
}

// GetStatefulSet provides statefulset for testing
func GetStatefulSetWithEnvVar(namespace string, statefulsetName string) *v1_beta1.StatefulSet {
	return &v1_beta1.StatefulSet{
		ObjectMeta: getObjectMeta(namespace, statefulsetName, true),
		Spec: v1_beta1.StatefulSetSpec{
			UpdateStrategy: v1_beta1.StatefulSetUpdateStrategy{
				Type: v1_beta1.RollingUpdateStatefulSetStrategyType,
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

// GetResourceSHA returns the SHA value of given environment variable
func GetResourceSHA(containers []v1.Container, envar string) string {
	for i := range containers {
		envs := containers[i].Env
		for j := range envs {
			if envs[j].Name == envar {
				return envs[j].Value
			}
		}
	}
	return ""
}

//ConvertResourceToSHA generates SHA from secret or configmap data
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
	_, err := configmapClient.Create(GetConfigmap(namespace, configmapName, data))
	time.Sleep(10 * time.Second)
	return configmapClient, err
}

// CreateSecret creates a secret in given namespace and returns the SecretInterface
func CreateSecret(client kubernetes.Interface, namespace string, secretName string, data string) (core_v1.SecretInterface, error) {
	logrus.Infof("Creating secret")
	secretClient := client.CoreV1().Secrets(namespace)
	_, err := secretClient.Create(GetSecret(namespace, secretName, data))
	time.Sleep(10 * time.Second)
	return secretClient, err
}

// CreateDeployment creates a deployment in given namespace and returns the Deployment
func CreateDeployment(client kubernetes.Interface, deploymentName string, namespace string, volumeMount bool) (*v1beta1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.ExtensionsV1beta1().Deployments(namespace)
	var deploymentObj *v1beta1.Deployment
	if volumeMount {
		deploymentObj = GetDeployment(namespace, deploymentName)
	} else {
		deploymentObj = GetDeploymentWithEnvVars(namespace, deploymentName)
	}
	deployment, err := deploymentClient.Create(deploymentObj)
	time.Sleep(10 * time.Second)
	return deployment, err
}

// CreateDaemonSet creates a deployment in given namespace and returns the DaemonSet
func CreateDaemonSet(client kubernetes.Interface, daemonsetName string, namespace string, volumeMount bool) (*v1beta1.DaemonSet, error) {
	logrus.Infof("Creating DaemonSet")
	daemonsetClient := client.ExtensionsV1beta1().DaemonSets(namespace)
	var daemonsetObj *v1beta1.DaemonSet
	if volumeMount {
		daemonsetObj = GetDaemonSet(namespace, daemonsetName)
	} else {
		daemonsetObj = GetDaemonSetWithEnvVars(namespace, daemonsetName)
	}
	daemonset, err := daemonsetClient.Create(daemonsetObj)
	time.Sleep(10 * time.Second)
	return daemonset, err
}

// CreateStatefulSet creates a deployment in given namespace and returns the StatefulSet
func CreateStatefulSet(client kubernetes.Interface, statefulsetName string, namespace string, volumeMount bool) (*v1_beta1.StatefulSet, error) {
	logrus.Infof("Creating StatefulSet")
	statefulsetClient := client.AppsV1beta1().StatefulSets(namespace)
	var statefulsetObj *v1_beta1.StatefulSet
	if volumeMount {
		statefulsetObj = GetStatefulSet(namespace, statefulsetName)
	} else {
		statefulsetObj = GetStatefulSetWithEnvVar(namespace, statefulsetName)
	}
	statefulset, err := statefulsetClient.Create(statefulsetObj)
	time.Sleep(10 * time.Second)
	return statefulset, err
}

// DeleteDeployment creates a deployment in given namespace and returns the error if any
func DeleteDeployment(client kubernetes.Interface, namespace string, deploymentName string) error {
	logrus.Infof("Deleting Deployment")
	deploymentError := client.ExtensionsV1beta1().Deployments(namespace).Delete(deploymentName, &metav1.DeleteOptions{})
	time.Sleep(10 * time.Second)
	return deploymentError
}

// DeleteDaemonSet creates a daemonset in given namespace and returns the error if any
func DeleteDaemonSet(client kubernetes.Interface, namespace string, daemonsetName string) error {
	logrus.Infof("Deleting DaemonSet %s", daemonsetName)
	daemonsetError := client.ExtensionsV1beta1().DaemonSets(namespace).Delete(daemonsetName, &metav1.DeleteOptions{})
	time.Sleep(10 * time.Second)
	return daemonsetError
}

// DeleteStatefulSet creates a statefulset in given namespace and returns the error if any
func DeleteStatefulSet(client kubernetes.Interface, namespace string, statefulsetName string) error {
	logrus.Infof("Deleting StatefulSet %s", statefulsetName)
	statefulsetError := client.AppsV1beta1().StatefulSets(namespace).Delete(statefulsetName, &metav1.DeleteOptions{})
	time.Sleep(10 * time.Second)
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
	_, updateErr := configmapClient.Update(configmap)
	time.Sleep(10 * time.Second)
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
	_, updateErr := secretClient.Update(secret)
	time.Sleep(10 * time.Second)
	return updateErr
}

// DeleteConfigMap deletes a configmap in given namespace and returns the error if any
func DeleteConfigMap(client kubernetes.Interface, namespace string, configmapName string) error {
	logrus.Infof("Deleting configmap %q.\n", configmapName)
	err := client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	time.Sleep(10 * time.Second)
	return err
}

// DeleteSecret deletes a secret in given namespace and returns the error if any
func DeleteSecret(client kubernetes.Interface, namespace string, secretName string) error {
	logrus.Infof("Deleting secret %q.\n", secretName)
	err := client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	time.Sleep(10 * time.Second)
	return err
}

// RandSeq generates a random sequence
func RandSeq(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// VerifyResourceUpdate verifies whether the rolling upgrade happened or not
func VerifyResourceUpdate(client kubernetes.Interface, config util.Config, envVarPostfix string, upgradeFuncs callbacks.RollingUpgradeFuncs) bool {
	items := upgradeFuncs.ItemsFunc(client, config.Namespace)
	for _, i := range items {
		containers := upgradeFuncs.ContainersFunc(i)
		// match statefulsets with the correct annotation
		annotationValue := util.ToObjectMeta(i).Annotations[config.Annotation]
		reloaderEnabledValue := util.ToObjectMeta(i).Annotations[constants.ReloaderAutoAnnotation]
		reloaderEnabled, err := strconv.ParseBool(reloaderEnabledValue)
		matches := false
		if err == nil && reloaderEnabled {
			matches = true
		} else if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				if value == config.ResourceName {
					matches = true
					break
				}
			}
		}

		if matches {
			envName := constants.EnvVarPrefix + util.ConvertToEnvVarName(config.ResourceName) + "_" + envVarPostfix
			updated := GetResourceSHA(containers, envName)
			if updated == config.SHAValue {
				return true
			}
		}
	}
	return false
}
