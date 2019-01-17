package callbacks

import (
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/util"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//ItemsFunc is a generic function to return a specific resource array in given namespace
type ItemsFunc func(kubernetes.Interface, string) []interface{}

//ContainersFunc is a generic func to return containers
type ContainersFunc func(interface{}) []v1.Container

//VolumesFunc is a generic func to return volumes
type VolumesFunc func(interface{}) []v1.Volume

//UpdateFunc performs the resource update
type UpdateFunc func(kubernetes.Interface, string, interface{}) error

//RollingUpgradeFuncs contains generic functions to perform rolling upgrade
type RollingUpgradeFuncs struct {
	ItemsFunc      ItemsFunc
	ContainersFunc ContainersFunc
	UpdateFunc     UpdateFunc
	VolumesFunc    VolumesFunc
	ResourceType   string
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
