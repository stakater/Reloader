package testutil

import (
	"math/rand"
	"sort"
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
		logrus.Fatalf("Failed to create namespace for testing", err)
	} else {
		logrus.Infof("Creating namespace for testing = %s", namespace)
	}
}

// DeleteNamespace deletes namespace for testing
func DeleteNamespace(namespace string, client kubernetes.Interface) {
	err := client.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Fatalf("Failed to delete namespace that was created for testing", err)
	} else {
		logrus.Infof("Deleting namespace for testing = %s", namespace)
	}
}

// GetDeployment provides deployment for testing
func GetDeployment(namespace string, deploymentName string) *v1beta1.Deployment {
	replicaset := int32(1)
	return &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
			Annotations: map[string]string{
				constants.ConfigmapUpdateOnChangeAnnotation: deploymentName,
				constants.SecretUpdateOnChangeAnnotation:    deploymentName},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicaset,
			Strategy: v1beta1.DeploymentStrategy{
				Type: v1beta1.RollingUpdateDeploymentStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"secondLabel": "temp"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "tutum/hello-world",
							Name:  deploymentName,
							Env: []v1.EnvVar{
								{
									Name:  "BUCKET_NAME",
									Value: "test",
								},
							},
						},
					},
				},
			},
		},
	}
}

// GetDaemonSet provides daemonset for testing
func GetDaemonSet(namespace string, daemonsetName string) *v1beta1.DaemonSet {
	return &v1beta1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonsetName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
			Annotations: map[string]string{
				constants.ConfigmapUpdateOnChangeAnnotation: daemonsetName,
				constants.SecretUpdateOnChangeAnnotation:    daemonsetName},
		},
		Spec: v1beta1.DaemonSetSpec{
			UpdateStrategy: v1beta1.DaemonSetUpdateStrategy{
				Type: v1beta1.RollingUpdateDaemonSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"secondLabel": "temp"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "tutum/hello-world",
							Name:  daemonsetName,
							Env: []v1.EnvVar{
								{
									Name:  "BUCKET_NAME",
									Value: "test",
								},
							},
						},
					},
				},
			},
		},
	}
}

// GetStatefulSet provides statefulset for testing
func GetStatefulSet(namespace string, statefulsetName string) *v1_beta1.StatefulSet {
	return &v1_beta1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulsetName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
			Annotations: map[string]string{
				constants.ConfigmapUpdateOnChangeAnnotation: statefulsetName,
				constants.SecretUpdateOnChangeAnnotation:    statefulsetName},
		},
		Spec: v1_beta1.StatefulSetSpec{
			UpdateStrategy: v1_beta1.StatefulSetUpdateStrategy{
				Type: v1_beta1.RollingUpdateStatefulSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"secondLabel": "temp"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "tutum/hello-world",
							Name:  statefulsetName,
							Env: []v1.EnvVar{
								{
									Name:  "BUCKET_NAME",
									Value: "test",
								},
							},
						},
					},
				},
			},
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
	logrus.Infof("Generating SHA for secret data")
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
	time.Sleep(5 * time.Second)
	return configmapClient, err
}

// CreateSecret creates a secret in given namespace and returns the SecretInterface
func CreateSecret(client kubernetes.Interface, namespace string, secretName string, data string) (core_v1.SecretInterface, error) {
	logrus.Infof("Creating secret")
	secretClient := client.CoreV1().Secrets(namespace)
	_, err := secretClient.Create(GetSecret(namespace, secretName, data))
	time.Sleep(5 * time.Second)
	return secretClient, err
}

// CreateDeployment creates a deployment in given namespace and returns the Deployment
func CreateDeployment(client kubernetes.Interface, deploymentName string, namespace string) (*v1beta1.Deployment, error) {
	logrus.Infof("Creating Deployment")
	deploymentClient := client.ExtensionsV1beta1().Deployments(namespace)
	deployment, err := deploymentClient.Create(GetDeployment(namespace, deploymentName))
	time.Sleep(5 * time.Second)
	return deployment, err
}

// CreateDaemonSet creates a deployment in given namespace and returns the DaemonSet
func CreateDaemonSet(client kubernetes.Interface, daemonsetName string, namespace string) (*v1beta1.DaemonSet, error) {
	logrus.Infof("Creating DaemonSet")
	daemonsetClient := client.ExtensionsV1beta1().DaemonSets(namespace)
	daemonset, err := daemonsetClient.Create(GetDaemonSet(namespace, daemonsetName))
	time.Sleep(5 * time.Second)
	return daemonset, err
}

// CreateStatefulSet creates a deployment in given namespace and returns the StatefulSet
func CreateStatefulSet(client kubernetes.Interface, statefulsetName string, namespace string) (*v1_beta1.StatefulSet, error) {
	logrus.Infof("Creating StatefulSet")
	statefulsetClient := client.AppsV1beta1().StatefulSets(namespace)
	statefulset, err := statefulsetClient.Create(GetStatefulSet(namespace, statefulsetName))
	time.Sleep(5 * time.Second)
	return statefulset, err
}

// DeleteDeployment creates a deployment in given namespace and returns the error if any
func DeleteDeployment(client kubernetes.Interface, namespace string, deploymentName string) error {
	logrus.Infof("Deleting Deployment")
	deploymentError := client.ExtensionsV1beta1().Deployments(namespace).Delete(deploymentName, &metav1.DeleteOptions{})
	time.Sleep(5 * time.Second)
	return deploymentError
}

// DeleteDaemonSet creates a daemonset in given namespace and returns the error if any
func DeleteDaemonSet(client kubernetes.Interface, namespace string, daemonsetName string) error {
	logrus.Infof("Deleting DaemonSet %s", daemonsetName)
	daemonsetError := client.ExtensionsV1beta1().DaemonSets(namespace).Delete(daemonsetName, &metav1.DeleteOptions{})
	time.Sleep(5 * time.Second)
	return daemonsetError
}

// DeleteStatefulSet creates a statefulset in given namespace and returns the error if any
func DeleteStatefulSet(client kubernetes.Interface, namespace string, statefulsetName string) error {
	logrus.Infof("Deleting StatefulSet %s", statefulsetName)
	statefulsetError := client.AppsV1beta1().StatefulSets(namespace).Delete(statefulsetName, &metav1.DeleteOptions{})
	time.Sleep(5 * time.Second)
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
	time.Sleep(5 * time.Second)
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
	time.Sleep(5 * time.Second)
	return updateErr
}

// DeleteConfigMap deletes a configmap in given namespace and returns the error if any
func DeleteConfigMap(client kubernetes.Interface, namespace string, configmapName string) error {
	logrus.Infof("Deleting configmap %q.\n", configmapName)
	err := client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	time.Sleep(5 * time.Second)
	return err
}

// DeleteSecret deletes a secret in given namespace and returns the error if any
func DeleteSecret(client kubernetes.Interface, namespace string, secretName string) error {
	logrus.Infof("Deleting secret %q.\n", secretName)
	err := client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	time.Sleep(5 * time.Second)
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

func VerifyResourceUpdate(client kubernetes.Interface, config util.Config, envVarPostfix string, upgradeFuncs callbacks.RollingUpgradeFuncs) bool {
	items := upgradeFuncs.ItemsFunc(client, config.Namespace)
	for _, i := range items {
		containers := upgradeFuncs.ContainersFunc(i)
		// match statefulsets with the correct annotation
		annotationValue := util.ToObjectMeta(i).Annotations[config.Annotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == config.ResourceName {
					matches = true
					break
				}
			}
			if matches {
				envName := constants.EnvVarPrefix + util.ConvertToEnvVarName(annotationValue) + envVarPostfix
				updated := GetResourceSHA(containers, envName)

				if updated == config.SHAValue {
					return true
				}
			}
		}
	}
	return false
}
