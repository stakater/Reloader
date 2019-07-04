package callbacks

import (
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openshiftv1 "github.com/openshift/api/apps/v1"
)

//ItemsFunc is a generic function to return a specific resource array in given namespace
type ItemsFunc func(kube.Clients, string) []interface{}

//ContainersFunc is a generic func to return containers
type ContainersFunc func(interface{}) []v1.Container

//InitContainersFunc is a generic func to return containers
type InitContainersFunc func(interface{}) []v1.Container

//VolumesFunc is a generic func to return volumes
type VolumesFunc func(interface{}) []v1.Volume

//UpdateFunc performs the resource update
type UpdateFunc func(kube.Clients, string, interface{}) error

//RollingUpgradeFuncs contains generic functions to perform rolling upgrade
type RollingUpgradeFuncs struct {
	ItemsFunc          ItemsFunc
	ContainersFunc     ContainersFunc
	InitContainersFunc InitContainersFunc
	UpdateFunc         UpdateFunc
	VolumesFunc        VolumesFunc
	ResourceType       string
}

// GetDeploymentItems returns the deployments in given namespace
func GetDeploymentItems(clients kube.Clients, namespace string) []interface{} {
	deployments, err := clients.KubernetesClient.ExtensionsV1beta1().Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deployments %v", err)
	}
	return util.InterfaceSlice(deployments.Items)
}

// GetDaemonSetItems returns the daemonSets in given namespace
func GetDaemonSetItems(clients kube.Clients, namespace string) []interface{} {
	daemonSets, err := clients.KubernetesClient.ExtensionsV1beta1().DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list daemonSets %v", err)
	}
	return util.InterfaceSlice(daemonSets.Items)
}

// GetStatefulSetItems returns the statefulSets in given namespace
func GetStatefulSetItems(clients kube.Clients, namespace string) []interface{} {
	statefulSets, err := clients.KubernetesClient.AppsV1beta1().StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list statefulSets %v", err)
	}
	return util.InterfaceSlice(statefulSets.Items)
}

// GetDeploymentConfigItems returns the deploymentConfigs in given namespace
func GetDeploymentConfigItems(clients kube.Clients, namespace string) []interface{} {
	deploymentConfigs, err := clients.OpenshiftAppsClient.Apps().DeploymentConfigs(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deploymentConfigs %v", err)
	}
	return util.InterfaceSlice(deploymentConfigs.Items)
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

// GetDeploymentConfigContainers returns the containers of given deploymentConfig
func GetDeploymentConfigContainers(item interface{}) []v1.Container {
	return item.(openshiftv1.DeploymentConfig).Spec.Template.Spec.Containers
}

// GetDeploymentInitContainers returns the containers of given deployment
func GetDeploymentInitContainers(item interface{}) []v1.Container {
	return item.(v1beta1.Deployment).Spec.Template.Spec.InitContainers
}

// GetDaemonSetInitContainers returns the containers of given daemonset
func GetDaemonSetInitContainers(item interface{}) []v1.Container {
	return item.(v1beta1.DaemonSet).Spec.Template.Spec.InitContainers
}

// GetStatefulsetInitContainers returns the containers of given statefulSet
func GetStatefulsetInitContainers(item interface{}) []v1.Container {
	return item.(apps_v1beta1.StatefulSet).Spec.Template.Spec.InitContainers
}

// GetDeploymentConfigInitContainers returns the containers of given deploymentConfig
func GetDeploymentConfigInitContainers(item interface{}) []v1.Container {
	return item.(openshiftv1.DeploymentConfig).Spec.Template.Spec.InitContainers
}

// UpdateDeployment performs rolling upgrade on deployment
func UpdateDeployment(clients kube.Clients, namespace string, resource interface{}) error {
	deployment := resource.(v1beta1.Deployment)
	_, err := clients.KubernetesClient.ExtensionsV1beta1().Deployments(namespace).Update(&deployment)
	return err
}

// UpdateDaemonSet performs rolling upgrade on daemonSet
func UpdateDaemonSet(clients kube.Clients, namespace string, resource interface{}) error {
	daemonSet := resource.(v1beta1.DaemonSet)
	_, err := clients.KubernetesClient.ExtensionsV1beta1().DaemonSets(namespace).Update(&daemonSet)
	return err
}

// UpdateStatefulset performs rolling upgrade on statefulSet
func UpdateStatefulset(clients kube.Clients, namespace string, resource interface{}) error {
	statefulSet := resource.(apps_v1beta1.StatefulSet)
	_, err := clients.KubernetesClient.AppsV1beta1().StatefulSets(namespace).Update(&statefulSet)
	return err
}

// UpdateDeploymentConfig performs rolling upgrade on deploymentConfig
func UpdateDeploymentConfig(clients kube.Clients, namespace string, resource interface{}) error {
	deploymentConfig := resource.(openshiftv1.DeploymentConfig)
	_, err := clients.OpenshiftAppsClient.AppsV1().DeploymentConfigs(namespace).Update(&deploymentConfig)
	return err
}

// GetDeploymentVolumes returns the Volumes of given deployment
func GetDeploymentVolumes(item interface{}) []v1.Volume {
	return item.(v1beta1.Deployment).Spec.Template.Spec.Volumes
}

// GetDaemonSetVolumes returns the Volumes of given daemonset
func GetDaemonSetVolumes(item interface{}) []v1.Volume {
	return item.(v1beta1.DaemonSet).Spec.Template.Spec.Volumes
}

// GetStatefulsetVolumes returns the Volumes of given statefulSet
func GetStatefulsetVolumes(item interface{}) []v1.Volume {
	return item.(apps_v1beta1.StatefulSet).Spec.Template.Spec.Volumes
}

// GetDeploymentConfigVolumes returns the Volumes of given deploymentConfig
func GetDeploymentConfigVolumes(item interface{}) []v1.Volume {
	return item.(openshiftv1.DeploymentConfig).Spec.Template.Spec.Volumes
}
