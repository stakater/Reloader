package handler

import (
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/common"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
		rollingUpgrade(r, "deployments")
		rollingUpgrade(r, "daemonsets")
		rollingUpgrade(r, "statefulSets")
	}
	return nil
}

func rollingUpgrade(r ResourceUpdatedHandler, rollingUpgradeType string) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}

	var shaData, oldSHAdata string
	shaData = getSHAfromData(r.Resource)
	oldSHAdata = getSHAfromData(r.OldResource)
	if shaData != oldSHAdata {
		var namespace, resourceName, envarPostfix, annotation string
		if _, ok := r.Resource.(*v1.ConfigMap); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'configmap'")
			namespace = r.Resource.(*v1.ConfigMap).Namespace
			resourceName = r.Resource.(*v1.ConfigMap).Name
			envarPostfix = common.ConfigmapEnvarPostfix
			annotation = common.ConfigmapUpdateOnChangeAnnotation
		} else if _, ok := r.Resource.(*v1.Secret); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'secret'")
			namespace = r.Resource.(*v1.Secret).Namespace
			resourceName = r.Resource.(*v1.Secret).Name
			envarPostfix = common.SecretEnvarPostfix
			annotation = common.SecretUpdateOnChangeAnnotation
		} else {
			logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
		}

		if rollingUpgradeType == "deployments" {
			err = RollingUpgradeDeployment(client, namespace, resourceName, shaData, envarPostfix, annotation)
		} else if rollingUpgradeType == "daemonsets" {
			err = RollingUpgradeDaemonSets(client, namespace, resourceName, shaData, envarPostfix, annotation)
		} else if rollingUpgradeType == "statefulSets" {
			err = RollingUpgradeStatefulSets(client, namespace, resourceName, shaData, envarPostfix, annotation)
		}

		if err != nil {
			logrus.Errorf("Rolling upgrade failed for %s of resource type %s", resourceName, rollingUpgradeType)
		}
	} else {
		logrus.Infof("Resource update will not happen because no data change detected")
	}
}

// RollingUpgradeDeployment upgrades the deployment if there is any change in configmap or secret data
func RollingUpgradeDeployment(client kubernetes.Interface, namespace string, resourceName string, shaData string, envarPostfix string, annotation string) error {
	deployments, err := client.ExtensionsV1beta1().Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deployments %v", err)
	}

	for _, d := range deployments.Items {
		containers := d.Spec.Template.Spec.Containers
		// match deployments with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[annotation]
		updated := performRollingUpgrade(containers, resourceName, annotationValue, shaData, envarPostfix)
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
	return nil
}

// RollingUpgradeDaemonSets upgrades the daemonset if there is any change in configmap or secret data
func RollingUpgradeDaemonSets(client kubernetes.Interface, namespace string, resourceName string, shaData string, envarPostfix string, annotation string) error {
	daemonSets, err := client.ExtensionsV1beta1().DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list daemonSets %v", err)
	}

	for _, d := range daemonSets.Items {
		containers := d.Spec.Template.Spec.Containers
		// match daemonSets with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[annotation]
		updated := performRollingUpgrade(containers, resourceName, annotationValue, shaData, envarPostfix)
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
	return err
}

// RollingUpgradeStatefulSets upgrades the statefulset if there is any change in configmap or secret data
func RollingUpgradeStatefulSets(client kubernetes.Interface, namespace string, resourceName string, shaData string, envarPostfix string, annotation string) error {
	statefulSets, err := client.AppsV1beta1().StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list statefulSets %v", err)
	}

	for _, s := range statefulSets.Items {
		containers := s.Spec.Template.Spec.Containers
		// match statefulSets with the correct annotation
		annotationValue := s.ObjectMeta.Annotations[annotation]
		updated := performRollingUpgrade(containers, resourceName, annotationValue, shaData, envarPostfix)
		if !updated {
			logrus.Warnf("Rolling upgrade did not happen")
		} else {
			// update the statefulSet
			_, err := client.AppsV1beta1().StatefulSets(namespace).Update(&s)
			if err != nil {
				logrus.Errorf("Update statefulSet failed %v", err)
			} else {
				logrus.Infof("Updated statefulSet %s", s.Name)
			}
		}
	}
	return err
}

func performRollingUpgrade(containers []v1.Container, resourceName string, annotationValue string, shaData string, envarPostfix string) bool {
	updated := false
	if annotationValue != "" {
		values := strings.Split(annotationValue, ",")
		for _, value := range values {
			if value == resourceName {
				updated = updateContainers(containers, value, shaData, envarPostfix)
				break
			}
		}
	}
	return updated
}

func updateContainers(containers []v1.Container, annotationValue string, shaData string, envarPostfix string) bool {
	updated := false
	envar := common.EnvVarPrefix + common.ConvertToEnvVarName(annotationValue) + envarPostfix
	logrus.Infof("Generated environment variable: %s", envar)
	for i := range containers {
		envs := containers[i].Env

		//update if env var exists
		updated = updateEnvVar(envs, envar, shaData)

		// if no existing env var exists lets create one
		if !updated {
			e := v1.EnvVar{
				Name:  envar,
				Value: shaData,
			}
			containers[i].Env = append(containers[i].Env, e)
			updated = true
			logrus.Infof("%s environment variable does not exist, creating a new envVar", envar)
		}
	}
	return updated
}

func updateEnvVar(envs []v1.EnvVar, envar string, shaData string) bool {
	for j := range envs {
		if envs[j].Name == envar {
			logrus.Infof("%s environment variable found", envar)
			if envs[j].Value != shaData {
				logrus.Infof("Updating %s to %s", envar, shaData)
				envs[j].Value = shaData
				return true
			}
		}
	}
	return false
}

func getSHAfromData(resource interface{}) string {
	values := []string{}
	if _, ok := resource.(*v1.ConfigMap); ok {
		logrus.Infof("Generating SHA for configmap data")
		for k, v := range resource.(*v1.ConfigMap).Data {
			values = append(values, k+"="+v)
		}
	} else if _, ok := resource.(*v1.Secret); ok {
		logrus.Infof("Generating SHA for secret data")
		for k, v := range resource.(*v1.Secret).Data {
			values = append(values, k+"="+string(v[:]))
		}
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}
