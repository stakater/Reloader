package helper

import (
	"math/rand"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	v1_beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	letters                           = []rune("abcdefghijklmnopqrstuvwxyz")
	configmapUpdateOnChangeAnnotation = "reloader.stakater.com/configmap.update-on-change"
	secretUpdateOnChangeAnnotation    = "reloader.stakater.com/secret.update-on-change"
)

// RandSeq generates a random sequence
func RandSeq(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
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
				"reloader.stakater.com/configmap.update-on-change": deploymentName,
				"reloader.stakater.com/secret.update-on-change":    deploymentName},
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

// GetDaemonset provides daemonset for testing
func GetDaemonset(namespace string, daemonsetName string) *v1beta1.DaemonSet {
	return &v1beta1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonsetName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
			Annotations: map[string]string{
				"reloader.stakater.com/configmap.update-on-change": daemonsetName,
				"reloader.stakater.com/secret.update-on-change":    daemonsetName},
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

// GetStatefulset provides statefulset for testing
func GetStatefulset(namespace string, statefulsetName string) *v1_beta1.StatefulSet {
	return &v1_beta1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulsetName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
			Annotations: map[string]string{
				"reloader.stakater.com/configmap.update-on-change": statefulsetName,
				"reloader.stakater.com/secret.update-on-change":    statefulsetName},
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

// VerifyDeploymentUpdate verifies whether deployment has been updated with environment variable or not
func VerifyDeploymentUpdate(client kubernetes.Interface, namespace string, name string, resourceType string, shaData string) bool {
	deployments, err := client.ExtensionsV1beta1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deployments %v", err)
	}
	for _, d := range deployments.Items {
		containers := d.Spec.Template.Spec.Containers
		// match deployments with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[configmapUpdateOnChangeAnnotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == name {
					matches = true
					break
				}
			}
			if matches {
				envName := "STAKATER_" + ConvertToEnvVarName(annotationValue) + resourceType
				updated := getResourceSHA(containers, envName)
				logrus.Infof("shaData %s", shaData)
				logrus.Infof("updated %s", updated)

				if updated != shaData {
					return false
				} else {
					return true
				}
			}
		}
	}
	return false
}

// VerifyDaemonsetUpdate verifies whether daemonset has been updated with environment variable or not
func VerifyDaemonsetUpdate(client kubernetes.Interface, namespace string, name string, resourceType string, shaData string) bool {
	daemonsets, err := client.ExtensionsV1beta1().DaemonSets(namespace).List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list daemonsets %v", err)
	}
	for _, d := range daemonsets.Items {
		containers := d.Spec.Template.Spec.Containers
		// match daemonsets with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[configmapUpdateOnChangeAnnotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == name {
					matches = true
					break
				}
			}
			if matches {
				envName := "STAKATER_" + ConvertToEnvVarName(annotationValue) + resourceType
				updated := getResourceSHA(containers, envName)
				logrus.Infof("shaData %s", shaData)
				logrus.Infof("updated %s", updated)

				if updated != shaData {
					return false
				} else {
					return true
				}
			}
		}
	}
	return false
}

// VerifyStatefulsetUpdate verifies whether statefulset has been updated with environment variable or not
func VerifyStatefulsetUpdate(client kubernetes.Interface, namespace string, name string, resourceType string, shaData string) bool {
	statefulsets, err := client.AppsV1beta1().StatefulSets(namespace).List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list statefulsets %v", err)
	}
	for _, d := range statefulsets.Items {
		containers := d.Spec.Template.Spec.Containers
		// match statefulsets with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[configmapUpdateOnChangeAnnotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == name {
					matches = true
					break
				}
			}
			if matches {
				envName := "STAKATER_" + ConvertToEnvVarName(annotationValue) + resourceType
				updated := getResourceSHA(containers, envName)
				logrus.Infof("shaData %s", shaData)
				logrus.Infof("updated %s", updated)

				if updated != shaData {
					return false
				} else {
					return true
				}
			}
		}
	}
	return false
}

func getResourceSHA(containers []v1.Container, envar string) string {
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
