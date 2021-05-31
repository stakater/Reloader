package callbacks

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
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

//AnnotationsFunc is a generic func to return annotations
type AnnotationsFunc func(interface{}) map[string]string

//PodAnnotationsFunc is a generic func to return annotations
type PodAnnotationsFunc func(interface{}) map[string]string

//RollingUpgradeFuncs contains generic functions to perform rolling upgrade
type RollingUpgradeFuncs struct {
	ItemsFunc          ItemsFunc
	AnnotationsFunc    AnnotationsFunc
	PodAnnotationsFunc PodAnnotationsFunc
	ContainersFunc     ContainersFunc
	InitContainersFunc InitContainersFunc
	UpdateFunc         UpdateFunc
	VolumesFunc        VolumesFunc
	ResourceType       string
}

// GetDeploymentItems returns the deployments in given namespace
func GetDeploymentItems(clients kube.Clients, namespace string) []interface{} {
	deployments, err := clients.KubernetesClient.AppsV1().Deployments(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deployments %v", err)
	}
	return util.InterfaceSlice(deployments.Items)
}

// GetDaemonSetItems returns the daemonSets in given namespace
func GetDaemonSetItems(clients kube.Clients, namespace string) []interface{} {
	daemonSets, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list daemonSets %v", err)
	}
	return util.InterfaceSlice(daemonSets.Items)
}

// GetStatefulSetItems returns the statefulSets in given namespace
func GetStatefulSetItems(clients kube.Clients, namespace string) []interface{} {
	statefulSets, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list statefulSets %v", err)
	}
	return util.InterfaceSlice(statefulSets.Items)
}

// GetDeploymentConfigItems returns the deploymentConfigs in given namespace
func GetDeploymentConfigItems(clients kube.Clients, namespace string) []interface{} {
	deploymentConfigs, err := clients.OpenshiftAppsClient.AppsV1().DeploymentConfigs(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deploymentConfigs %v", err)
	}
	return util.InterfaceSlice(deploymentConfigs.Items)
}

// GetRolloutItems returns the rollouts in given namespace
func GetRolloutItems(clients kube.Clients, namespace string) []interface{} {
	rollouts, err := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list Rollouts %v", err)
	}
	return util.InterfaceSlice(rollouts.Items)
}

// GetDeploymentAnnotations returns the annotations of given deployment
func GetDeploymentAnnotations(item interface{}) map[string]string {
	return item.(appsv1.Deployment).ObjectMeta.Annotations
}

// GetDaemonSetAnnotations returns the annotations of given daemonSet
func GetDaemonSetAnnotations(item interface{}) map[string]string {
	return item.(appsv1.DaemonSet).ObjectMeta.Annotations
}

// GetStatefulSetAnnotations returns the annotations of given statefulSet
func GetStatefulSetAnnotations(item interface{}) map[string]string {
	return item.(appsv1.StatefulSet).ObjectMeta.Annotations
}

// GetDeploymentConfigAnnotations returns the annotations of given deploymentConfig
func GetDeploymentConfigAnnotations(item interface{}) map[string]string {
	return item.(openshiftv1.DeploymentConfig).ObjectMeta.Annotations
}

// GetRolloutAnnotations returns the annotations of given rollout
func GetRolloutAnnotations(item interface{}) map[string]string {
	return item.(argorolloutv1alpha1.Rollout).ObjectMeta.Annotations
}

// GetDeploymentPodAnnotations returns the pod's annotations of given deployment
func GetDeploymentPodAnnotations(item interface{}) map[string]string {
	return item.(appsv1.Deployment).Spec.Template.ObjectMeta.Annotations
}

// GetDaemonSetPodAnnotations returns the pod's annotations of given daemonSet
func GetDaemonSetPodAnnotations(item interface{}) map[string]string {
	return item.(appsv1.DaemonSet).Spec.Template.ObjectMeta.Annotations
}

// GetStatefulSetPodAnnotations returns the pod's annotations of given statefulSet
func GetStatefulSetPodAnnotations(item interface{}) map[string]string {
	return item.(appsv1.StatefulSet).Spec.Template.ObjectMeta.Annotations
}

// GetDeploymentConfigPodAnnotations returns the pod's annotations of given deploymentConfig
func GetDeploymentConfigPodAnnotations(item interface{}) map[string]string {
	return item.(openshiftv1.DeploymentConfig).Spec.Template.ObjectMeta.Annotations
}

// GetRolloutPodAnnotations returns the pod's annotations of given rollout
func GetRolloutPodAnnotations(item interface{}) map[string]string {
	return item.(argorolloutv1alpha1.Rollout).Spec.Template.ObjectMeta.Annotations
}

// GetDeploymentContainers returns the containers of given deployment
func GetDeploymentContainers(item interface{}) []v1.Container {
	return item.(appsv1.Deployment).Spec.Template.Spec.Containers
}

// GetDaemonSetContainers returns the containers of given daemonSet
func GetDaemonSetContainers(item interface{}) []v1.Container {
	return item.(appsv1.DaemonSet).Spec.Template.Spec.Containers
}

// GetStatefulSetContainers returns the containers of given statefulSet
func GetStatefulSetContainers(item interface{}) []v1.Container {
	return item.(appsv1.StatefulSet).Spec.Template.Spec.Containers
}

// GetDeploymentConfigContainers returns the containers of given deploymentConfig
func GetDeploymentConfigContainers(item interface{}) []v1.Container {
	return item.(openshiftv1.DeploymentConfig).Spec.Template.Spec.Containers
}

// GetRolloutContainers returns the containers of given rollout
func GetRolloutContainers(item interface{}) []v1.Container {
	return item.(argorolloutv1alpha1.Rollout).Spec.Template.Spec.Containers
}

// GetDeploymentInitContainers returns the containers of given deployment
func GetDeploymentInitContainers(item interface{}) []v1.Container {
	return item.(appsv1.Deployment).Spec.Template.Spec.InitContainers
}

// GetDaemonSetInitContainers returns the containers of given daemonSet
func GetDaemonSetInitContainers(item interface{}) []v1.Container {
	return item.(appsv1.DaemonSet).Spec.Template.Spec.InitContainers
}

// GetStatefulSetInitContainers returns the containers of given statefulSet
func GetStatefulSetInitContainers(item interface{}) []v1.Container {
	return item.(appsv1.StatefulSet).Spec.Template.Spec.InitContainers
}

// GetDeploymentConfigInitContainers returns the containers of given deploymentConfig
func GetDeploymentConfigInitContainers(item interface{}) []v1.Container {
	return item.(openshiftv1.DeploymentConfig).Spec.Template.Spec.InitContainers
}

// GetRolloutInitContainers returns the containers of given rollout
func GetRolloutInitContainers(item interface{}) []v1.Container {
	return item.(argorolloutv1alpha1.Rollout).Spec.Template.Spec.InitContainers
}

// UpdateDeployment performs rolling upgrade on deployment
func UpdateDeployment(clients kube.Clients, namespace string, resource interface{}) error {
	deployment := resource.(appsv1.Deployment)
	_, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Update(context.TODO(), &deployment, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateDaemonSet performs rolling upgrade on daemonSet
func UpdateDaemonSet(clients kube.Clients, namespace string, resource interface{}) error {
	daemonSet := resource.(appsv1.DaemonSet)
	_, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).Update(context.TODO(), &daemonSet, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateStatefulSet performs rolling upgrade on statefulSet
func UpdateStatefulSet(clients kube.Clients, namespace string, resource interface{}) error {
	statefulSet := resource.(appsv1.StatefulSet)
	_, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).Update(context.TODO(), &statefulSet, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateDeploymentConfig performs rolling upgrade on deploymentConfig
func UpdateDeploymentConfig(clients kube.Clients, namespace string, resource interface{}) error {
	deploymentConfig := resource.(openshiftv1.DeploymentConfig)
	_, err := clients.OpenshiftAppsClient.AppsV1().DeploymentConfigs(namespace).Update(context.TODO(), &deploymentConfig, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateRollout performs rolling upgrade on rollout
func UpdateRollout(clients kube.Clients, namespace string, resource interface{}) error {
	rollout := resource.(argorolloutv1alpha1.Rollout)
	rolloutBefore, _ := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(context.TODO(), rollout.Name, meta_v1.GetOptions{})
	logrus.Warnf("Before: %+v", rolloutBefore.Spec.Template.Spec.Containers[0].Env)
	logrus.Warnf("After: %+v", rollout.Spec.Template.Spec.Containers[0].Env)
	_, err := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Update(context.TODO(), &rollout, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// GetDeploymentVolumes returns the Volumes of given deployment
func GetDeploymentVolumes(item interface{}) []v1.Volume {
	return item.(appsv1.Deployment).Spec.Template.Spec.Volumes
}

// GetDaemonSetVolumes returns the Volumes of given daemonSet
func GetDaemonSetVolumes(item interface{}) []v1.Volume {
	return item.(appsv1.DaemonSet).Spec.Template.Spec.Volumes
}

// GetStatefulSetVolumes returns the Volumes of given statefulSet
func GetStatefulSetVolumes(item interface{}) []v1.Volume {
	return item.(appsv1.StatefulSet).Spec.Template.Spec.Volumes
}

// GetDeploymentConfigVolumes returns the Volumes of given deploymentConfig
func GetDeploymentConfigVolumes(item interface{}) []v1.Volume {
	return item.(openshiftv1.DeploymentConfig).Spec.Template.Spec.Volumes
}

// GetRolloutVolumes returns the Volumes of given rollout
func GetRolloutVolumes(item interface{}) []v1.Volume {
	return item.(argorolloutv1alpha1.Rollout).Spec.Template.Spec.Volumes
}
