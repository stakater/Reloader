package handler

import (
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/common"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResourceUpdatedHandler contains updated objects
type ResourceUpdatedHandler struct {
	Resource    interface{}
	OldResource interface{}
}

//Config contains rolling upgrade configuration parameters
type Config struct {
	namespace    string
	resourceName string
	annotation   string
	shaValue     string
}

//ItemsFunc is a generic function to return a specific resource array in given namespace
type ItemsFunc func(kubernetes.Interface, string) []interface{}

//ContainersFunc is a generic func to return containers
type ContainersFunc func(interface{}) []v1.Container

//UpdateFunc performs the resource update
type UpdateFunc func(kubernetes.Interface, string, interface{}) error

//RollingUpgradeFuncs contains generic functions to perform rolling upgrade
type RollingUpgradeFuncs struct {
	ItemsFunc      ItemsFunc
	ContainersFunc ContainersFunc
	UpdateFunc     UpdateFunc
}

// Handle processes the updated resource
func (r ResourceUpdatedHandler) Handle() error {
	if r.Resource == nil || r.OldResource == nil {
		logrus.Errorf("Error in Handler")
	} else {
		logrus.Infof("Detected changes in object %s", r.Resource)
		// process resource based on its type
		rollingUpgrade(r, RollingUpgradeFuncs{
			ItemsFunc:      GetDeploymentItems,
			ContainersFunc: GetDeploymentContainers,
			UpdateFunc:     UpdateDeployment,
		})
		rollingUpgrade(r, RollingUpgradeFuncs{
			ItemsFunc:      GetDaemonSetItems,
			ContainersFunc: GetDaemonSetContainers,
			UpdateFunc:     UpdateDaemonSet,
		})
		rollingUpgrade(r, RollingUpgradeFuncs{
			ItemsFunc:      GetStatefulSetItems,
			ContainersFunc: GetStatefulsetContainers,
			UpdateFunc:     UpdateStatefulset,
		})
	}
	return nil
}

// GetDeploymentItems returns the deployments in given namespace
func GetDeploymentItems(client kubernetes.Interface, namespace string) []interface{} {
	deployments, err := client.ExtensionsV1beta1().Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deployments %v", err)
	}
	return util.InterfaceSlice(deployments.Items)
}

// GetDaemonSetItems returns the daemonSet in given namespace
func GetDaemonSetItems(client kubernetes.Interface, namespace string) []interface{} {
	daemonSets, err := client.ExtensionsV1beta1().DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list daemonSets %v", err)
	}
	return util.InterfaceSlice(daemonSets.Items)
}

// GetStatefulSetItems returns the statefulSet in given namespace
func GetStatefulSetItems(client kubernetes.Interface, namespace string) []interface{} {
	statefulSets, err := client.AppsV1beta1().StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list statefulSets %v", err)
	}
	return util.InterfaceSlice(statefulSets.Items)
}

// GetDeploymentContainers returns the containers of given deployment
func GetDeploymentContainers(item interface{}) []v1.Container {
	return item.(v1beta1.Deployment).Spec.Template.Spec.Containers
}

// GetDaemonSetContainers returns the containers of given daemonset
func GetDaemonSetContainers(item interface{}) []v1.Container {
	return item.(v1beta1.DaemonSet).Spec.Template.Spec.Containers
}

// GetStatefulsetContainers returns the containers of given statefulSet
func GetStatefulsetContainers(item interface{}) []v1.Container {
	return item.(apps_v1beta1.StatefulSet).Spec.Template.Spec.Containers
}

// UpdateDeployment performs rolling upgrade on deployment
func UpdateDeployment(client kubernetes.Interface, namespace string, resource interface{}) error {
	deployment := resource.(v1beta1.Deployment)
	_, err := client.ExtensionsV1beta1().Deployments(namespace).Update(&deployment)
	return err
}

// UpdateDaemonSet performs rolling upgrade on daemonSet
func UpdateDaemonSet(client kubernetes.Interface, namespace string, resource interface{}) error {
	daemonSet := resource.(v1beta1.DaemonSet)
	_, err := client.ExtensionsV1beta1().DaemonSets(namespace).Update(&daemonSet)
	return err
}

// UpdateStatefulset performs rolling upgrade on statefulSet
func UpdateStatefulset(client kubernetes.Interface, namespace string, resource interface{}) error {
	statefulSet := resource.(apps_v1beta1.StatefulSet)
	_, err := client.AppsV1beta1().StatefulSets(namespace).Update(&statefulSet)
	return err
}

func rollingUpgrade(r ResourceUpdatedHandler, upgradeFuncs RollingUpgradeFuncs) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}

	config, envVarPostfix, oldSHAData := getConfig(r)

	if config.shaValue != oldSHAData {
		err = PerformRollingUpgrade(client, config, envVarPostfix, upgradeFuncs)
		if err != nil {
			logrus.Fatalf("Rolling upgrade failed with error = %v", err)
		}
	} else {
		logrus.Infof("Rolling upgrade will not happend because no actual change in data has been detected")
	}
}

func getConfig(r ResourceUpdatedHandler) (Config, string, string) {
	var shaData, oldSHAData, envVarPostfix string
	var config Config
	if _, ok := r.Resource.(*v1.ConfigMap); ok {
		logrus.Infof("Performing 'Updated' action for resource of type 'configmap'")
		configmap := r.Resource.(*v1.ConfigMap)
		shaData = getSHAfromConfigmap(configmap.Data)
		oldSHAData = getSHAfromConfigmap(r.OldResource.(*v1.ConfigMap).Data)
		config = Config{
			namespace:    configmap.Namespace,
			resourceName: configmap.Name,
			annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
			shaValue:     shaData,
		}
		envVarPostfix = constants.ConfigmapEnvarPostfix
	} else if _, ok := r.Resource.(*v1.Secret); ok {
		logrus.Infof("Performing 'Updated' action for resource of type 'secret'")
		secret := r.Resource.(*v1.Secret)
		shaData = getSHAfromSecret(secret.Data)
		oldSHAData = getSHAfromSecret(r.OldResource.(*v1.Secret).Data)
		config = Config{
			namespace:    secret.Namespace,
			resourceName: secret.Name,
			annotation:   constants.SecretUpdateOnChangeAnnotation,
			shaValue:     shaData,
		}
		envVarPostfix = constants.SecretEnvarPostfix
	} else {
		logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
	}
	return config, envVarPostfix, oldSHAData
}

// PerformRollingUpgrade upgrades the deployment if there is any change in configmap or secret data
func PerformRollingUpgrade(client kubernetes.Interface, config Config, envarPostfix string, upgradeFuncs RollingUpgradeFuncs) error {
	items := upgradeFuncs.ItemsFunc(client, config.namespace)
	var err error
	for _, i := range items {
		containers := upgradeFuncs.ContainersFunc(i)
		// find correct annotation and update the resource
		annotationValue := util.ToObjectMeta(i).Annotations[config.annotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				if value == config.resourceName {
					updated := updateContainers(containers, value, config.shaValue, envarPostfix)
					if !updated {
						logrus.Warnf("Rolling upgrade did not happen")
					} else {
						err = upgradeFuncs.UpdateFunc(client, config.namespace, i)
						if err != nil {
							logrus.Errorf("Update deployment failed %v", err)
						} else {
							logrus.Infof("Updated Deployment %s", config.resourceName)
						}
						break
					}
				}
			}
		}
	}
	return err
}

func updateContainers(containers []v1.Container, annotationValue string, shaData string, envarPostfix string) bool {
	updated := false
	envar := constants.EnvVarPrefix + common.ConvertToEnvVarName(annotationValue) + envarPostfix
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

func getSHAfromConfigmap(data map[string]string) string {
	values := []string{}
	for k, v := range data {
		values = append(values, k+"="+v)
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

func getSHAfromSecret(data map[string][]byte) string {
	values := []string{}
	logrus.Infof("Generating SHA for secret data")
	for k, v := range data {
		values = append(values, k+"="+string(v[:]))
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}
