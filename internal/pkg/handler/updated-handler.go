package handler

import (
	"strings"

	"github.com/sirupsen/logrus"
	helper "github.com/stakater/Reloader/internal/pkg/helper"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	configmapUpdateOnChangeAnnotation = "reloader.stakater.com/configmap.update-on-change"
	// Adding separate annotation to differentiate between configmap and secret
	secretUpdateOnChangeAnnotation = "reloader.stakater.com/secret.update-on-change"
)

// ResourceUpdatedHandler contains updated objects
type ResourceUpdatedHandler struct {
	Resource    interface{}
	OldResource interface{}
}

// Handle processes the updated resource
func (r ResourceUpdatedHandler) Handle() error {
	if r.Resource == nil || r.OldResource == nil {
		logrus.Errorf("Error in Handler")
	} else {
		logrus.Infof("Detected changes in object %s", r.Resource)
		// process resource based on its type
		if _, ok := r.Resource.(*v1.ConfigMap); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'configmap'")
			rollingUpgrade(r, "configmaps", "deployments")
			rollingUpgrade(r, "configmaps", "daemonsets")
			rollingUpgrade(r, "configmaps", "statefulSets")
		} else if _, ok := r.Resource.(*v1.Secret); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'secret'")
			rollingUpgrade(r, "secrets", "deployments")
			rollingUpgrade(r, "secrets", "daemonsets")
			rollingUpgrade(r, "secrets", "statefulSets")
		} else {
			logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
		}
	}
	return nil
}

func rollingUpgrade(r ResourceUpdatedHandler, resourceType string, rollingUpgradeType string) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	var namespace, name, shaData, envName string
	if resourceType == "configmaps" {
		namespace = r.Resource.(*v1.ConfigMap).Namespace
		name = r.Resource.(*v1.ConfigMap).Name
		shaData = helper.ConvertConfigmapToSHA(r.Resource.(*v1.ConfigMap))
		envName = "_CONFIGMAP"
	} else if resourceType == "secrets" {
		namespace = r.Resource.(*v1.Secret).Namespace
		name = r.Resource.(*v1.Secret).Name
		shaData = helper.ConvertSecretToSHA(r.Resource.(*v1.Secret))
		envName = "_SECRET"
	}

	if rollingUpgradeType == "deployments" {
		RollingUpgradeForDeployment(client, namespace, name, shaData, envName)
	} else if rollingUpgradeType == "daemonsets" {
		RollingUpgradeForDaemonSets(client, namespace, name, shaData, envName)
	} else if rollingUpgradeType == "statefulSets" {
		RollingUpgradeForStatefulSets(client, namespace, name, shaData, envName)
	}
}

// RollingUpgradeForDeployment upgrades the deployment if there is any change in configmap or secret data
func RollingUpgradeForDeployment(client kubernetes.Interface, namespace string, name string, shaData string, envName string) error {
	deployments, err := client.ExtensionsV1beta1().Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deployments %v", err)
	}
	var updateOnChangeAnnotation string
	if envName == "_CONFIGMAP" {
		updateOnChangeAnnotation = configmapUpdateOnChangeAnnotation
	} else if envName == "_SECRET" {
		updateOnChangeAnnotation = secretUpdateOnChangeAnnotation
	}
	for _, d := range deployments.Items {
		containers := d.Spec.Template.Spec.Containers
		// match deployments with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[updateOnChangeAnnotation]

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
				updated := updateContainers(containers, name, shaData, envName)

				if !updated {
					logrus.Warnf("Rolling upgrade did not happen")
				} else {
					// update the deployment
					_, err := client.ExtensionsV1beta1().Deployments(namespace).Update(&d)
					if err != nil {
						logrus.Errorf("Update deployment failed %v", err)
					} else {
						logrus.Infof("Updated Deployment %s", d.Name)
					}
				}
			}
		}
	}
	return nil
}

// RollingUpgradeForDaemonSets upgrades the daemonset if there is any change in configmap or secret data
func RollingUpgradeForDaemonSets(client kubernetes.Interface, namespace string, name string, shaData string, envName string) error {
	daemonSets, err := client.ExtensionsV1beta1().DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list daemonSets %v", err)
	}

	var updateOnChangeAnnotation string
	if envName == "_CONFIGMAP" {
		updateOnChangeAnnotation = configmapUpdateOnChangeAnnotation
	} else if envName == "_SECRET" {
		updateOnChangeAnnotation = secretUpdateOnChangeAnnotation
	}
	for _, d := range daemonSets.Items {
		containers := d.Spec.Template.Spec.Containers
		// match daemonSets with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[updateOnChangeAnnotation]

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
				updated := updateContainers(containers, name, shaData, envName)

				if !updated {
					logrus.Warnf("Rolling upgrade did not happen")
				} else {
					// update the daemonSet
					_, err := client.ExtensionsV1beta1().DaemonSets(namespace).Update(&d)
					if err != nil {
						logrus.Errorf("Update daemonSet failed %v", err)
					} else {
						logrus.Infof("Updated daemonSet %s", d.Name)
					}
				}
			}
		}
	}
	return nil
}

// RollingUpgradeForStatefulSets upgrades the statefulset if there is any change in configmap or secret data
func RollingUpgradeForStatefulSets(client kubernetes.Interface, namespace string, name string, shaData string, envName string) error {
	statefulSets, err := client.AppsV1beta1().StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list statefulSets %v", err)
	}
	var updateOnChangeAnnotation string
	if envName == "_CONFIGMAP" {
		updateOnChangeAnnotation = configmapUpdateOnChangeAnnotation
	} else if envName == "_SECRET" {
		updateOnChangeAnnotation = secretUpdateOnChangeAnnotation
	}
	for _, d := range statefulSets.Items {
		containers := d.Spec.Template.Spec.Containers
		// match statefulSets with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[updateOnChangeAnnotation]

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
				updated := updateContainers(containers, name, shaData, envName)

				if !updated {
					logrus.Warnf("Rolling upgrade did not happen")
				} else {
					// update the statefulSet
					_, err := client.AppsV1beta1().StatefulSets(namespace).Update(&d)
					if err != nil {
						logrus.Errorf("Update statefulSet failed %v", err)
					} else {
						logrus.Infof("Updated statefulSet %s", d.Name)
					}
				}
			}
		}
	}
	return nil
}

func updateContainers(containers []v1.Container, annotationValue string, shaData string, resourceType string) bool {
	updated := false
	envar := "STAKATER_" + helper.ConvertToEnvVarName(annotationValue) + resourceType
	logrus.Infof("Generated environment variable: %s", envar)

	for i := range containers {
		envs := containers[i].Env
		matched := false
		for j := range envs {
			if envs[j].Name == envar {
				matched = true
				logrus.Infof("%s environment variable found", envar)
				if envs[j].Value != shaData {
					logrus.Infof("Updating %s to %s", envar, shaData)
					envs[j].Value = shaData
					updated = true
				}
			}
		}
		// if no existing env var exists lets create one
		if !matched {
			e := v1.EnvVar{
				Name:  envar,
				Value: shaData,
			}
			containers[i].Env = append(containers[i].Env, e)
			updated = true
			logrus.Infof("%s environment variable does not found, creating a new env with value %s", envar, shaData)
		}
	}
	return updated
}
