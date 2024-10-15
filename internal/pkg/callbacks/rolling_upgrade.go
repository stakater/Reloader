package callbacks

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	patchtypes "k8s.io/apimachinery/pkg/types"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	openshiftv1 "github.com/openshift/api/apps/v1"
)

// ItemFunc is a generic function to return a specific resource in given namespace
type ItemFunc func(kube.Clients, string, string) (runtime.Object, error)

// ItemsFunc is a generic function to return a specific resource array in given namespace
type ItemsFunc func(kube.Clients, string) []runtime.Object

// ContainersFunc is a generic func to return containers
type ContainersFunc func(runtime.Object) []v1.Container

// InitContainersFunc is a generic func to return containers
type InitContainersFunc func(runtime.Object) []v1.Container

// VolumesFunc is a generic func to return volumes
type VolumesFunc func(runtime.Object) []v1.Volume

// UpdateFunc performs the resource update
type UpdateFunc func(kube.Clients, string, runtime.Object) error

// AnnotationsFunc is a generic func to return annotations
type AnnotationsFunc func(runtime.Object) map[string]string

// PodAnnotationsFunc is a generic func to return annotations
type PodAnnotationsFunc func(runtime.Object) map[string]string

// RollingUpgradeFuncs contains generic functions to perform rolling upgrade
type RollingUpgradeFuncs struct {
	ItemFunc           ItemFunc
	ItemsFunc          ItemsFunc
	AnnotationsFunc    AnnotationsFunc
	PodAnnotationsFunc PodAnnotationsFunc
	ContainersFunc     ContainersFunc
	InitContainersFunc InitContainersFunc
	UpdateFunc         UpdateFunc
	VolumesFunc        VolumesFunc
	ResourceType       string
}

// GetDeploymentItem returns the deployment in given namespace
func GetDeploymentItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get deployment %v", err)
		return nil, err
	}

	return deployment, nil
}

// GetDeploymentItems returns the deployments in given namespace
func GetDeploymentItems(clients kube.Clients, namespace string) []runtime.Object {
	deployments, err := clients.KubernetesClient.AppsV1().Deployments(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deployments %v", err)
	}

	items := make([]runtime.Object, len(deployments.Items))
	// Ensure we always have pod annotations to add to
	for i, v := range deployments.Items {
		if v.Spec.Template.ObjectMeta.Annotations == nil {
			annotations := make(map[string]string)
			deployments.Items[i].Spec.Template.ObjectMeta.Annotations = annotations
		}
		items[i] = &deployments.Items[i]
	}

	return items
}

// GetCronJobItem returns the job in given namespace
func GetCronJobItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	cronjob, err := clients.KubernetesClient.BatchV1().CronJobs(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get cronjob %v", err)
		return nil, err
	}

	return cronjob, nil
}

// GetCronJobItems returns the jobs in given namespace
func GetCronJobItems(clients kube.Clients, namespace string) []runtime.Object {
	cronjobs, err := clients.KubernetesClient.BatchV1().CronJobs(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list cronjobs %v", err)
	}

	items := make([]runtime.Object, len(cronjobs.Items))
	// Ensure we always have pod annotations to add to
	for i, v := range cronjobs.Items {
		if v.Spec.JobTemplate.Spec.Template.ObjectMeta.Annotations == nil {
			annotations := make(map[string]string)
			cronjobs.Items[i].Spec.JobTemplate.Spec.Template.ObjectMeta.Annotations = annotations
		}
		items[i] = &cronjobs.Items[i]
	}

	return items
}

// GetDaemonSetItem returns the daemonSet in given namespace
func GetDaemonSetItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	daemonSet, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get daemonSet %v", err)
		return nil, err
	}

	return daemonSet, nil
}

// GetDaemonSetItems returns the daemonSets in given namespace
func GetDaemonSetItems(clients kube.Clients, namespace string) []runtime.Object {
	daemonSets, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list daemonSets %v", err)
	}

	items := make([]runtime.Object, len(daemonSets.Items))
	// Ensure we always have pod annotations to add to
	for i, v := range daemonSets.Items {
		if v.Spec.Template.ObjectMeta.Annotations == nil {
			daemonSets.Items[i].Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		items[i] = &daemonSets.Items[i]
	}

	return items
}

// GetStatefulSetItem returns the statefulSet in given namespace
func GetStatefulSetItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	statefulSet, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get statefulSet %v", err)
		return nil, err
	}

	return statefulSet, nil
}

// GetStatefulSetItems returns the statefulSets in given namespace
func GetStatefulSetItems(clients kube.Clients, namespace string) []runtime.Object {
	statefulSets, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list statefulSets %v", err)
	}

	items := make([]runtime.Object, len(statefulSets.Items))
	// Ensure we always have pod annotations to add to
	for i, v := range statefulSets.Items {
		if v.Spec.Template.ObjectMeta.Annotations == nil {
			statefulSets.Items[i].Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		items[i] = &statefulSets.Items[i]
	}

	return items
}

// GetDeploymentConfigItem returns the deploymentConfig in given namespace
func GetDeploymentConfigItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	deploymentConfig, err := clients.OpenshiftAppsClient.AppsV1().DeploymentConfigs(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get deploymentConfig %v", err)
		return nil, err
	}

	return deploymentConfig, nil
}

// GetDeploymentConfigItems returns the deploymentConfigs in given namespace
func GetDeploymentConfigItems(clients kube.Clients, namespace string) []runtime.Object {
	deploymentConfigs, err := clients.OpenshiftAppsClient.AppsV1().DeploymentConfigs(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list deploymentConfigs %v", err)
	}

	items := make([]runtime.Object, len(deploymentConfigs.Items))
	// Ensure we always have pod annotations to add to
	for i, v := range deploymentConfigs.Items {
		if v.Spec.Template.ObjectMeta.Annotations == nil {
			deploymentConfigs.Items[i].Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		items[i] = &deploymentConfigs.Items[i]
	}

	return items
}

// GetRolloutItem returns the rollout in given namespace
func GetRolloutItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	rollout, err := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get Rollout %v", err)
		return nil, err
	}

	return rollout, nil
}

// GetRolloutItems returns the rollouts in given namespace
func GetRolloutItems(clients kube.Clients, namespace string) []runtime.Object {
	rollouts, err := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list Rollouts %v", err)
	}

	items := make([]runtime.Object, len(rollouts.Items))
	// Ensure we always have pod annotations to add to
	for i, v := range rollouts.Items {
		if v.Spec.Template.ObjectMeta.Annotations == nil {
			rollouts.Items[i].Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		items[i] = &rollouts.Items[i]
	}

	return items
}

// GetDeploymentAnnotations returns the annotations of given deployment
func GetDeploymentAnnotations(item runtime.Object) map[string]string {
	return item.(*appsv1.Deployment).ObjectMeta.Annotations
}

// GetCronJobAnnotations returns the annotations of given cronjob
func GetCronJobAnnotations(item runtime.Object) map[string]string {
	return item.(*batchv1.CronJob).ObjectMeta.Annotations
}

// GetDaemonSetAnnotations returns the annotations of given daemonSet
func GetDaemonSetAnnotations(item runtime.Object) map[string]string {
	return item.(*appsv1.DaemonSet).ObjectMeta.Annotations
}

// GetStatefulSetAnnotations returns the annotations of given statefulSet
func GetStatefulSetAnnotations(item runtime.Object) map[string]string {
	return item.(*appsv1.StatefulSet).ObjectMeta.Annotations
}

// GetDeploymentConfigAnnotations returns the annotations of given deploymentConfig
func GetDeploymentConfigAnnotations(item runtime.Object) map[string]string {
	return item.(*openshiftv1.DeploymentConfig).ObjectMeta.Annotations
}

// GetRolloutAnnotations returns the annotations of given rollout
func GetRolloutAnnotations(item runtime.Object) map[string]string {
	return item.(*argorolloutv1alpha1.Rollout).ObjectMeta.Annotations
}

// GetDeploymentPodAnnotations returns the pod's annotations of given deployment
func GetDeploymentPodAnnotations(item runtime.Object) map[string]string {
	return item.(*appsv1.Deployment).Spec.Template.ObjectMeta.Annotations
}

// GetCronJobPodAnnotations returns the pod's annotations of given cronjob
func GetCronJobPodAnnotations(item runtime.Object) map[string]string {
	return item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.ObjectMeta.Annotations
}

// GetDaemonSetPodAnnotations returns the pod's annotations of given daemonSet
func GetDaemonSetPodAnnotations(item runtime.Object) map[string]string {
	return item.(*appsv1.DaemonSet).Spec.Template.ObjectMeta.Annotations
}

// GetStatefulSetPodAnnotations returns the pod's annotations of given statefulSet
func GetStatefulSetPodAnnotations(item runtime.Object) map[string]string {
	return item.(*appsv1.StatefulSet).Spec.Template.ObjectMeta.Annotations
}

// GetDeploymentConfigPodAnnotations returns the pod's annotations of given deploymentConfig
func GetDeploymentConfigPodAnnotations(item runtime.Object) map[string]string {
	return item.(*openshiftv1.DeploymentConfig).Spec.Template.ObjectMeta.Annotations
}

// GetRolloutPodAnnotations returns the pod's annotations of given rollout
func GetRolloutPodAnnotations(item runtime.Object) map[string]string {
	return item.(*argorolloutv1alpha1.Rollout).Spec.Template.ObjectMeta.Annotations
}

// GetDeploymentContainers returns the containers of given deployment
func GetDeploymentContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.Deployment).Spec.Template.Spec.Containers
}

// GetCronJobContainers returns the containers of given cronjob
func GetCronJobContainers(item runtime.Object) []v1.Container {
	return item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Spec.Containers
}

// GetDaemonSetContainers returns the containers of given daemonSet
func GetDaemonSetContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.DaemonSet).Spec.Template.Spec.Containers
}

// GetStatefulSetContainers returns the containers of given statefulSet
func GetStatefulSetContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.StatefulSet).Spec.Template.Spec.Containers
}

// GetDeploymentConfigContainers returns the containers of given deploymentConfig
func GetDeploymentConfigContainers(item runtime.Object) []v1.Container {
	return item.(*openshiftv1.DeploymentConfig).Spec.Template.Spec.Containers
}

// GetRolloutContainers returns the containers of given rollout
func GetRolloutContainers(item runtime.Object) []v1.Container {
	return item.(*argorolloutv1alpha1.Rollout).Spec.Template.Spec.Containers
}

// GetDeploymentInitContainers returns the containers of given deployment
func GetDeploymentInitContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.Deployment).Spec.Template.Spec.InitContainers
}

// GetCronJobInitContainers returns the containers of given cronjob
func GetCronJobInitContainers(item runtime.Object) []v1.Container {
	return item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Spec.InitContainers
}

// GetDaemonSetInitContainers returns the containers of given daemonSet
func GetDaemonSetInitContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.DaemonSet).Spec.Template.Spec.InitContainers
}

// GetStatefulSetInitContainers returns the containers of given statefulSet
func GetStatefulSetInitContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.StatefulSet).Spec.Template.Spec.InitContainers
}

// GetDeploymentConfigInitContainers returns the containers of given deploymentConfig
func GetDeploymentConfigInitContainers(item runtime.Object) []v1.Container {
	return item.(*openshiftv1.DeploymentConfig).Spec.Template.Spec.InitContainers
}

// GetRolloutInitContainers returns the containers of given rollout
func GetRolloutInitContainers(item runtime.Object) []v1.Container {
	return item.(*argorolloutv1alpha1.Rollout).Spec.Template.Spec.InitContainers
}

// UpdateDeployment performs rolling upgrade on deployment
func UpdateDeployment(clients kube.Clients, namespace string, resource runtime.Object) error {
	deployment := resource.(*appsv1.Deployment)
	_, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// CreateJobFromCronjob performs rolling upgrade on cronjob
func CreateJobFromCronjob(clients kube.Clients, namespace string, resource runtime.Object) error {
	cronJob := resource.(*batchv1.CronJob)
	job := &batchv1.Job{
		ObjectMeta: cronJob.Spec.JobTemplate.ObjectMeta,
		Spec:       cronJob.Spec.JobTemplate.Spec,
	}
	job.GenerateName = cronJob.Name + "-"
	_, err := clients.KubernetesClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, meta_v1.CreateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateDaemonSet performs rolling upgrade on daemonSet
func UpdateDaemonSet(clients kube.Clients, namespace string, resource runtime.Object) error {
	daemonSet := resource.(*appsv1.DaemonSet)
	_, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).Update(context.TODO(), daemonSet, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateStatefulSet performs rolling upgrade on statefulSet
func UpdateStatefulSet(clients kube.Clients, namespace string, resource runtime.Object) error {
	statefulSet := resource.(*appsv1.StatefulSet)
	_, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).Update(context.TODO(), statefulSet, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateDeploymentConfig performs rolling upgrade on deploymentConfig
func UpdateDeploymentConfig(clients kube.Clients, namespace string, resource runtime.Object) error {
	deploymentConfig := resource.(*openshiftv1.DeploymentConfig)
	_, err := clients.OpenshiftAppsClient.AppsV1().DeploymentConfigs(namespace).Update(context.TODO(), deploymentConfig, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// UpdateRollout performs rolling upgrade on rollout
func UpdateRollout(clients kube.Clients, namespace string, resource runtime.Object) error {
	rollout := resource.(*argorolloutv1alpha1.Rollout)
	rolloutBefore, _ := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(context.TODO(), rollout.Name, meta_v1.GetOptions{})
	logrus.Warnf("Before: %+v", rolloutBefore.Spec.Template.Spec.Containers[0].Env)
	logrus.Warnf("After: %+v", rollout.Spec.Template.Spec.Containers[0].Env)
	_, err := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Patch(context.TODO(), rollout.Name, patchtypes.MergePatchType, []byte(fmt.Sprintf(`{"spec": {"restartAt": "%s"}}`, time.Now().Format(time.RFC3339))), meta_v1.PatchOptions{FieldManager: "Reloader"})
	return err
}

// GetDeploymentVolumes returns the Volumes of given deployment
func GetDeploymentVolumes(item runtime.Object) []v1.Volume {
	return item.(*appsv1.Deployment).Spec.Template.Spec.Volumes
}

// GetCronJobVolumes returns the Volumes of given cronjob
func GetCronJobVolumes(item runtime.Object) []v1.Volume {
	return item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Spec.Volumes
}

// GetDaemonSetVolumes returns the Volumes of given daemonSet
func GetDaemonSetVolumes(item runtime.Object) []v1.Volume {
	return item.(*appsv1.DaemonSet).Spec.Template.Spec.Volumes
}

// GetStatefulSetVolumes returns the Volumes of given statefulSet
func GetStatefulSetVolumes(item runtime.Object) []v1.Volume {
	return item.(*appsv1.StatefulSet).Spec.Template.Spec.Volumes
}

// GetDeploymentConfigVolumes returns the Volumes of given deploymentConfig
func GetDeploymentConfigVolumes(item runtime.Object) []v1.Volume {
	return item.(*openshiftv1.DeploymentConfig).Spec.Template.Spec.Volumes
}

// GetRolloutVolumes returns the Volumes of given rollout
func GetRolloutVolumes(item runtime.Object) []v1.Volume {
	return item.(*argorolloutv1alpha1.Rollout).Spec.Template.Spec.Volumes
}
