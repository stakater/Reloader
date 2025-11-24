package callbacks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	patchtypes "k8s.io/apimachinery/pkg/types"

	"maps"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
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

// PatchFunc performs the resource patch
type PatchFunc func(kube.Clients, string, runtime.Object, patchtypes.PatchType, []byte) error

// PatchTemplateFunc is a generic func to return strategic merge JSON patch template
type PatchTemplatesFunc func() PatchTemplates

// AnnotationsFunc is a generic func to return annotations
type AnnotationsFunc func(runtime.Object) map[string]string

// PodAnnotationsFunc is a generic func to return annotations
type PodAnnotationsFunc func(runtime.Object) map[string]string

// RollingUpgradeFuncs contains generic functions to perform rolling upgrade
type RollingUpgradeFuncs struct {
	ItemFunc               ItemFunc
	ItemsFunc              ItemsFunc
	AnnotationsFunc        AnnotationsFunc
	PodAnnotationsFunc     PodAnnotationsFunc
	ContainersFunc         ContainersFunc
	ContainerPatchPathFunc ContainersFunc
	InitContainersFunc     InitContainersFunc
	UpdateFunc             UpdateFunc
	PatchFunc              PatchFunc
	PatchTemplatesFunc     PatchTemplatesFunc
	VolumesFunc            VolumesFunc
	ResourceType           string
	SupportsPatch          bool
}

// PatchTemplates contains merge JSON patch templates
type PatchTemplates struct {
	AnnotationTemplate   string
	EnvVarTemplate       string
	DeleteEnvVarTemplate string
}

// GetDeploymentItem returns the deployment in given namespace
func GetDeploymentItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get deployment %v", err)
		return nil, err
	}

	if deployment.Spec.Template.Annotations == nil {
		annotations := make(map[string]string)
		deployment.Spec.Template.Annotations = annotations
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
		if v.Spec.Template.Annotations == nil {
			annotations := make(map[string]string)
			deployments.Items[i].Spec.Template.Annotations = annotations
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
		if v.Spec.JobTemplate.Spec.Template.Annotations == nil {
			annotations := make(map[string]string)
			cronjobs.Items[i].Spec.JobTemplate.Spec.Template.Annotations = annotations
		}
		items[i] = &cronjobs.Items[i]
	}

	return items
}

// GetJobItem returns the job in given namespace
func GetJobItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	job, err := clients.KubernetesClient.BatchV1().Jobs(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get job %v", err)
		return nil, err
	}

	return job, nil
}

// GetJobItems returns the jobs in given namespace
func GetJobItems(clients kube.Clients, namespace string) []runtime.Object {
	jobs, err := clients.KubernetesClient.BatchV1().Jobs(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list jobs %v", err)
	}

	items := make([]runtime.Object, len(jobs.Items))
	// Ensure we always have pod annotations to add to
	for i, v := range jobs.Items {
		if v.Spec.Template.Annotations == nil {
			annotations := make(map[string]string)
			jobs.Items[i].Spec.Template.Annotations = annotations
		}
		items[i] = &jobs.Items[i]
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
		if v.Spec.Template.Annotations == nil {
			daemonSets.Items[i].Spec.Template.Annotations = make(map[string]string)
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
		if v.Spec.Template.Annotations == nil {
			statefulSets.Items[i].Spec.Template.Annotations = make(map[string]string)
		}
		items[i] = &statefulSets.Items[i]
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
		if v.Spec.Template.Annotations == nil {
			rollouts.Items[i].Spec.Template.Annotations = make(map[string]string)
		}
		items[i] = &rollouts.Items[i]
	}

	return items
}

// GetDeploymentAnnotations returns the annotations of given deployment
func GetDeploymentAnnotations(item runtime.Object) map[string]string {
	if item.(*appsv1.Deployment).Annotations == nil {
		item.(*appsv1.Deployment).Annotations = make(map[string]string)
	}
	return item.(*appsv1.Deployment).Annotations
}

// GetCronJobAnnotations returns the annotations of given cronjob
func GetCronJobAnnotations(item runtime.Object) map[string]string {
	if item.(*batchv1.CronJob).Annotations == nil {
		item.(*batchv1.CronJob).Annotations = make(map[string]string)
	}
	return item.(*batchv1.CronJob).Annotations
}

// GetJobAnnotations returns the annotations of given job
func GetJobAnnotations(item runtime.Object) map[string]string {
	if item.(*batchv1.Job).Annotations == nil {
		item.(*batchv1.Job).Annotations = make(map[string]string)
	}
	return item.(*batchv1.Job).Annotations
}

// GetDaemonSetAnnotations returns the annotations of given daemonSet
func GetDaemonSetAnnotations(item runtime.Object) map[string]string {
	if item.(*appsv1.DaemonSet).Annotations == nil {
		item.(*appsv1.DaemonSet).Annotations = make(map[string]string)
	}
	return item.(*appsv1.DaemonSet).Annotations
}

// GetStatefulSetAnnotations returns the annotations of given statefulSet
func GetStatefulSetAnnotations(item runtime.Object) map[string]string {
	if item.(*appsv1.StatefulSet).Annotations == nil {
		item.(*appsv1.StatefulSet).Annotations = make(map[string]string)
	}
	return item.(*appsv1.StatefulSet).Annotations
}

// GetRolloutAnnotations returns the annotations of given rollout
func GetRolloutAnnotations(item runtime.Object) map[string]string {
	if item.(*argorolloutv1alpha1.Rollout).Annotations == nil {
		item.(*argorolloutv1alpha1.Rollout).Annotations = make(map[string]string)
	}
	return item.(*argorolloutv1alpha1.Rollout).Annotations
}

// GetDeploymentPodAnnotations returns the pod's annotations of given deployment
func GetDeploymentPodAnnotations(item runtime.Object) map[string]string {
	if item.(*appsv1.Deployment).Spec.Template.Annotations == nil {
		item.(*appsv1.Deployment).Spec.Template.Annotations = make(map[string]string)
	}
	return item.(*appsv1.Deployment).Spec.Template.Annotations
}

// GetCronJobPodAnnotations returns the pod's annotations of given cronjob
func GetCronJobPodAnnotations(item runtime.Object) map[string]string {
	if item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Annotations == nil {
		item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Annotations = make(map[string]string)
	}
	return item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Annotations
}

// GetJobPodAnnotations returns the pod's annotations of given job
func GetJobPodAnnotations(item runtime.Object) map[string]string {
	if item.(*batchv1.Job).Spec.Template.Annotations == nil {
		item.(*batchv1.Job).Spec.Template.Annotations = make(map[string]string)
	}
	return item.(*batchv1.Job).Spec.Template.Annotations
}

// GetDaemonSetPodAnnotations returns the pod's annotations of given daemonSet
func GetDaemonSetPodAnnotations(item runtime.Object) map[string]string {
	if item.(*appsv1.DaemonSet).Spec.Template.Annotations == nil {
		item.(*appsv1.DaemonSet).Spec.Template.Annotations = make(map[string]string)
	}
	return item.(*appsv1.DaemonSet).Spec.Template.Annotations
}

// GetStatefulSetPodAnnotations returns the pod's annotations of given statefulSet
func GetStatefulSetPodAnnotations(item runtime.Object) map[string]string {
	if item.(*appsv1.StatefulSet).Spec.Template.Annotations == nil {
		item.(*appsv1.StatefulSet).Spec.Template.Annotations = make(map[string]string)
	}
	return item.(*appsv1.StatefulSet).Spec.Template.Annotations
}

// GetRolloutPodAnnotations returns the pod's annotations of given rollout
func GetRolloutPodAnnotations(item runtime.Object) map[string]string {
	if item.(*argorolloutv1alpha1.Rollout).Spec.Template.Annotations == nil {
		item.(*argorolloutv1alpha1.Rollout).Spec.Template.Annotations = make(map[string]string)
	}
	return item.(*argorolloutv1alpha1.Rollout).Spec.Template.Annotations
}

// GetDeploymentContainers returns the containers of given deployment
func GetDeploymentContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.Deployment).Spec.Template.Spec.Containers
}

// GetCronJobContainers returns the containers of given cronjob
func GetCronJobContainers(item runtime.Object) []v1.Container {
	return item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Spec.Containers
}

// GetJobContainers returns the containers of given job
func GetJobContainers(item runtime.Object) []v1.Container {
	return item.(*batchv1.Job).Spec.Template.Spec.Containers
}

// GetDaemonSetContainers returns the containers of given daemonSet
func GetDaemonSetContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.DaemonSet).Spec.Template.Spec.Containers
}

// GetStatefulSetContainers returns the containers of given statefulSet
func GetStatefulSetContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.StatefulSet).Spec.Template.Spec.Containers
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

// GetJobInitContainers returns the containers of given job
func GetJobInitContainers(item runtime.Object) []v1.Container {
	return item.(*batchv1.Job).Spec.Template.Spec.InitContainers
}

// GetDaemonSetInitContainers returns the containers of given daemonSet
func GetDaemonSetInitContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.DaemonSet).Spec.Template.Spec.InitContainers
}

// GetStatefulSetInitContainers returns the containers of given statefulSet
func GetStatefulSetInitContainers(item runtime.Object) []v1.Container {
	return item.(*appsv1.StatefulSet).Spec.Template.Spec.InitContainers
}

// GetRolloutInitContainers returns the containers of given rollout
func GetRolloutInitContainers(item runtime.Object) []v1.Container {
	return item.(*argorolloutv1alpha1.Rollout).Spec.Template.Spec.InitContainers
}

// GetPatchTemplates returns patch templates
func GetPatchTemplates() PatchTemplates {
	return PatchTemplates{
		AnnotationTemplate:   `{"spec":{"template":{"metadata":{"annotations":{"%s":"%s"}}}}}`,                                   // strategic merge patch
		EnvVarTemplate:       `{"spec":{"template":{"spec":{"containers":[{"name":"%s","env":[{"name":"%s","value":"%s"}]}]}}}}`, // strategic merge patch
		DeleteEnvVarTemplate: `[{"op":"remove","path":"/spec/template/spec/containers/%d/env/%d"}]`,                              // JSON patch
	}
}

// UpdateDeployment performs rolling upgrade on deployment
func UpdateDeployment(clients kube.Clients, namespace string, resource runtime.Object) error {
	deployment := resource.(*appsv1.Deployment)
	_, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

// PatchDeployment performs rolling upgrade on deployment
func PatchDeployment(clients kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
	deployment := resource.(*appsv1.Deployment)
	_, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Patch(context.TODO(), deployment.Name, patchType, bytes, meta_v1.PatchOptions{FieldManager: "Reloader"})
	return err
}

// CreateJobFromCronjob performs rolling upgrade on cronjob
func CreateJobFromCronjob(clients kube.Clients, namespace string, resource runtime.Object) error {
	cronJob := resource.(*batchv1.CronJob)

	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	maps.Copy(annotations, cronJob.Spec.JobTemplate.Annotations)

	job := &batchv1.Job{
		ObjectMeta: meta_v1.ObjectMeta{
			GenerateName:    cronJob.Name + "-",
			Namespace:       cronJob.Namespace,
			Annotations:     annotations,
			Labels:          cronJob.Spec.JobTemplate.Labels,
			OwnerReferences: []meta_v1.OwnerReference{*meta_v1.NewControllerRef(cronJob, batchv1.SchemeGroupVersion.WithKind("CronJob"))},
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}
	_, err := clients.KubernetesClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, meta_v1.CreateOptions{FieldManager: "Reloader"})
	return err
}

func PatchCronJob(clients kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
	return errors.New("not supported patching: CronJob")
}

// ReCreateJobFromjob performs rolling upgrade on job
func ReCreateJobFromjob(clients kube.Clients, namespace string, resource runtime.Object) error {
	oldJob := resource.(*batchv1.Job)
	job := oldJob.DeepCopy()

	// Delete the old job
	policy := meta_v1.DeletePropagationBackground
	err := clients.KubernetesClient.BatchV1().Jobs(namespace).Delete(context.TODO(), job.Name, meta_v1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil {
		return err
	}

	// Remove fields that should not be specified when creating a new Job
	job.ResourceVersion = ""
	job.UID = ""
	job.CreationTimestamp = meta_v1.Time{}
	job.Status = batchv1.JobStatus{}

	// Remove problematic labels
	delete(job.Spec.Template.Labels, "controller-uid")
	delete(job.Spec.Template.Labels, batchv1.ControllerUidLabel)
	delete(job.Spec.Template.Labels, batchv1.JobNameLabel)
	delete(job.Spec.Template.Labels, "job-name")

	// Remove the selector to allow it to be auto-generated
	job.Spec.Selector = nil

	// Create the new job with same spec
	_, err = clients.KubernetesClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, meta_v1.CreateOptions{FieldManager: "Reloader"})
	return err
}

func PatchJob(clients kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
	return errors.New("not supported patching: Job")
}

// UpdateDaemonSet performs rolling upgrade on daemonSet
func UpdateDaemonSet(clients kube.Clients, namespace string, resource runtime.Object) error {
	daemonSet := resource.(*appsv1.DaemonSet)
	_, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).Update(context.TODO(), daemonSet, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

func PatchDaemonSet(clients kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
	daemonSet := resource.(*appsv1.DaemonSet)
	_, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).Patch(context.TODO(), daemonSet.Name, patchType, bytes, meta_v1.PatchOptions{FieldManager: "Reloader"})
	return err
}

// UpdateStatefulSet performs rolling upgrade on statefulSet
func UpdateStatefulSet(clients kube.Clients, namespace string, resource runtime.Object) error {
	statefulSet := resource.(*appsv1.StatefulSet)
	_, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).Update(context.TODO(), statefulSet, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	return err
}

func PatchStatefulSet(clients kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
	statefulSet := resource.(*appsv1.StatefulSet)
	_, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).Patch(context.TODO(), statefulSet.Name, patchType, bytes, meta_v1.PatchOptions{FieldManager: "Reloader"})
	return err
}

// UpdateRollout performs rolling upgrade on rollout
func UpdateRollout(clients kube.Clients, namespace string, resource runtime.Object) error {
	rollout := resource.(*argorolloutv1alpha1.Rollout)
	strategy := rollout.GetAnnotations()[options.RolloutStrategyAnnotation]
	var err error
	switch options.ToArgoRolloutStrategy(strategy) {
	case options.RestartStrategy:
		_, err = clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Patch(context.TODO(), rollout.Name, patchtypes.MergePatchType, []byte(fmt.Sprintf(`{"spec": {"restartAt": "%s"}}`, time.Now().Format(time.RFC3339))), meta_v1.PatchOptions{FieldManager: "Reloader"})
	case options.RolloutStrategy:
		_, err = clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Update(context.TODO(), rollout, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	}
	return err
}

func PatchRollout(clients kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
	return errors.New("not supported patching: Rollout")
}

// GetDeploymentVolumes returns the Volumes of given deployment
func GetDeploymentVolumes(item runtime.Object) []v1.Volume {
	return item.(*appsv1.Deployment).Spec.Template.Spec.Volumes
}

// GetCronJobVolumes returns the Volumes of given cronjob
func GetCronJobVolumes(item runtime.Object) []v1.Volume {
	return item.(*batchv1.CronJob).Spec.JobTemplate.Spec.Template.Spec.Volumes
}

// GetJobVolumes returns the Volumes of given job
func GetJobVolumes(item runtime.Object) []v1.Volume {
	return item.(*batchv1.Job).Spec.Template.Spec.Volumes
}

// GetDaemonSetVolumes returns the Volumes of given daemonSet
func GetDaemonSetVolumes(item runtime.Object) []v1.Volume {
	return item.(*appsv1.DaemonSet).Spec.Template.Spec.Volumes
}

// GetStatefulSetVolumes returns the Volumes of given statefulSet
func GetStatefulSetVolumes(item runtime.Object) []v1.Volume {
	return item.(*appsv1.StatefulSet).Spec.Template.Spec.Volumes
}

// GetRolloutVolumes returns the Volumes of given rollout
func GetRolloutVolumes(item runtime.Object) []v1.Volume {
	return item.(*argorolloutv1alpha1.Rollout).Spec.Template.Spec.Volumes
}

// Knative Service GVR (Group Version Resource)
var knativeServiceGVR = schema.GroupVersionResource{
	Group:    "serving.knative.dev",
	Version:  "v1",
	Resource: "services",
}

// GetKnativeServiceItem returns the Knative Service in given namespace
func GetKnativeServiceItem(clients kube.Clients, name string, namespace string) (runtime.Object, error) {
	if clients.DynamicClient == nil {
		return nil, fmt.Errorf("dynamic client not available")
	}
	
	service, err := clients.DynamicClient.Resource(knativeServiceGVR).Namespace(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get Knative Service %v", err)
		return nil, err
	}

	// Ensure template annotations exist
	template, found, err := unstructured.NestedMap(service.Object, "spec", "template", "metadata")
	if err != nil {
		return nil, err
	}
	if !found {
		template = make(map[string]interface{})
	}
	if _, found := template["annotations"]; !found {
		template["annotations"] = make(map[string]interface{})
		unstructured.SetNestedMap(service.Object, template, "spec", "template", "metadata")
	}

	return service, nil
}

// GetKnativeServiceItems returns the Knative Services in given namespace
func GetKnativeServiceItems(clients kube.Clients, namespace string) []runtime.Object {
	if clients.DynamicClient == nil {
		logrus.Warnf("Dynamic client not available, skipping Knative Services")
		return []runtime.Object{}
	}

	services, err := clients.DynamicClient.Resource(knativeServiceGVR).Namespace(namespace).List(context.TODO(), meta_v1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list Knative Services %v", err)
		return []runtime.Object{}
	}

	items := make([]runtime.Object, len(services.Items))
	for i, v := range services.Items {
		// Ensure template annotations exist
		template, found, err := unstructured.NestedMap(v.Object, "spec", "template", "metadata")
		if err == nil {
			if !found {
				template = make(map[string]interface{})
			}
			if _, found := template["annotations"]; !found {
				template["annotations"] = make(map[string]interface{})
				unstructured.SetNestedMap(v.Object, template, "spec", "template", "metadata")
			}
		}
		items[i] = &v
	}

	return items
}

// GetKnativeServiceAnnotations returns the annotations of given Knative Service
func GetKnativeServiceAnnotations(item runtime.Object) map[string]string {
	service := item.(*unstructured.Unstructured)
	annotations, found, err := unstructured.NestedStringMap(service.Object, "metadata", "annotations")
	if err != nil || !found {
		annotations = make(map[string]string)
		unstructured.SetNestedStringMap(service.Object, annotations, "metadata", "annotations")
	}
	return annotations
}

// GetKnativeServicePodAnnotations returns the pod's annotations of given Knative Service
func GetKnativeServicePodAnnotations(item runtime.Object) map[string]string {
	service := item.(*unstructured.Unstructured)
	annotations, found, err := unstructured.NestedStringMap(service.Object, "spec", "template", "metadata", "annotations")
	if err != nil || !found {
		annotations = make(map[string]string)
		unstructured.SetNestedStringMap(service.Object, annotations, "spec", "template", "metadata", "annotations")
	}
	return annotations
}

// GetKnativeServiceContainers returns the containers of given Knative Service
func GetKnativeServiceContainers(item runtime.Object) []v1.Container {
	service := item.(*unstructured.Unstructured)
	containers, found, err := unstructured.NestedSlice(service.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || len(containers) == 0 {
		return []v1.Container{}
	}

	// Convert unstructured containers to v1.Container
	result := make([]v1.Container, 0, len(containers))
	for _, c := range containers {
		containerMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		container := v1.Container{}
		if name, ok := containerMap["name"].(string); ok {
			container.Name = name
		}
		if image, ok := containerMap["image"].(string); ok {
			container.Image = image
		}
		// Handle env vars
		if env, ok := containerMap["env"].([]interface{}); ok {
			container.Env = make([]v1.EnvVar, 0, len(env))
			for _, e := range env {
				if envMap, ok := e.(map[string]interface{}); ok {
					envVar := v1.EnvVar{}
					if name, ok := envMap["name"].(string); ok {
						envVar.Name = name
					}
					if value, ok := envMap["value"].(string); ok {
						envVar.Value = value
					}
					// Handle valueFrom if needed
					if valueFrom, ok := envMap["valueFrom"].(map[string]interface{}); ok {
						if configMapKeyRef, ok := valueFrom["configMapKeyRef"].(map[string]interface{}); ok {
							envVar.ValueFrom = &v1.EnvVarSource{
								ConfigMapKeyRef: &v1.ConfigMapKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: configMapKeyRef["name"].(string),
									},
									Key: configMapKeyRef["key"].(string),
								},
							}
						} else if secretKeyRef, ok := valueFrom["secretKeyRef"].(map[string]interface{}); ok {
							envVar.ValueFrom = &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretKeyRef["name"].(string),
									},
									Key: secretKeyRef["key"].(string),
								},
							}
						}
					}
					container.Env = append(container.Env, envVar)
				}
			}
		}
		// Handle envFrom
		if envFrom, ok := containerMap["envFrom"].([]interface{}); ok {
			container.EnvFrom = make([]v1.EnvFromSource, 0, len(envFrom))
			for _, ef := range envFrom {
				if envFromMap, ok := ef.(map[string]interface{}); ok {
					envFromSource := v1.EnvFromSource{}
					if configMapRef, ok := envFromMap["configMapRef"].(map[string]interface{}); ok {
						envFromSource.ConfigMapRef = &v1.ConfigMapEnvSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: configMapRef["name"].(string),
							},
						}
					}
					if secretRef, ok := envFromMap["secretRef"].(map[string]interface{}); ok {
						envFromSource.SecretRef = &v1.SecretEnvSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: secretRef["name"].(string),
							},
						}
					}
					container.EnvFrom = append(container.EnvFrom, envFromSource)
				}
			}
		}
		result = append(result, container)
	}
	return result
}

// GetKnativeServiceInitContainers returns the init containers of given Knative Service
func GetKnativeServiceInitContainers(item runtime.Object) []v1.Container {
	service := item.(*unstructured.Unstructured)
	containers, found, err := unstructured.NestedSlice(service.Object, "spec", "template", "spec", "initContainers")
	if err != nil || !found {
		return []v1.Container{}
	}

	// Similar conversion as GetKnativeServiceContainers but simpler for init containers
	result := make([]v1.Container, 0, len(containers))
	for _, c := range containers {
		containerMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		container := v1.Container{}
		if name, ok := containerMap["name"].(string); ok {
			container.Name = name
		}
		result = append(result, container)
	}
	return result
}

// GetKnativeServiceVolumes returns the Volumes of given Knative Service
func GetKnativeServiceVolumes(item runtime.Object) []v1.Volume {
	service := item.(*unstructured.Unstructured)
	volumes, found, err := unstructured.NestedSlice(service.Object, "spec", "template", "spec", "volumes")
	if err != nil || !found {
		return []v1.Volume{}
	}

	result := make([]v1.Volume, 0, len(volumes))
	for _, v := range volumes {
		volumeMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		volume := v1.Volume{}
		if name, ok := volumeMap["name"].(string); ok {
			volume.Name = name
		}
		if configMap, ok := volumeMap["configMap"].(map[string]interface{}); ok {
			volume.ConfigMap = &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: configMap["name"].(string),
				},
			}
		}
		if secret, ok := volumeMap["secret"].(map[string]interface{}); ok {
			volume.Secret = &v1.SecretVolumeSource{
				SecretName: secret["secretName"].(string),
			}
		}
		result = append(result, volume)
	}
	return result
}

// UpdateKnativeService performs rolling upgrade on Knative Service by updating spec.template to trigger new revision
// IMPORTANT: We update the Service spec.template, NOT the underlying Deployment. This triggers Knative to create
// a new revision, which is the correct way to update Knative Services. Directly modifying the Deployment would
// cause Knative's controller to reconcile and revert the changes (see: https://github.com/knative/serving/issues/14705)
func UpdateKnativeService(clients kube.Clients, namespace string, resource runtime.Object) error {
	service := resource.(*unstructured.Unstructured)
	if clients.DynamicClient == nil {
		return fmt.Errorf("dynamic client not available")
	}

	// Ensure we're updating the Service, not the Deployment
	// The service object should have spec.template which we've already modified
	_, err := clients.DynamicClient.Resource(knativeServiceGVR).Namespace(namespace).Update(context.TODO(), service, meta_v1.UpdateOptions{FieldManager: "Reloader"})
	if err != nil {
		logrus.Errorf("Failed to update Knative Service %s in namespace %s: %v", service.GetName(), namespace, err)
		return err
	}
	logrus.Infof("Successfully updated Knative Service %s in namespace %s, new revision will be created", service.GetName(), namespace)
	return nil
}

// PatchKnativeService performs patch on Knative Service
// IMPORTANT: We patch the Service spec.template, NOT the underlying Deployment. This triggers Knative to create
// a new revision, which is the correct way to update Knative Services. Directly modifying the Deployment would
// cause Knative's controller to reconcile and revert the changes (see: https://github.com/knative/serving/issues/14705)
func PatchKnativeService(clients kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
	service := resource.(*unstructured.Unstructured)
	if clients.DynamicClient == nil {
		return fmt.Errorf("dynamic client not available")
	}

	_, err := clients.DynamicClient.Resource(knativeServiceGVR).Namespace(namespace).Patch(context.TODO(), service.GetName(), patchType, bytes, meta_v1.PatchOptions{FieldManager: "Reloader"})
	if err != nil {
		logrus.Errorf("Failed to patch Knative Service %s in namespace %s: %v", service.GetName(), namespace, err)
		return err
	}
	logrus.Infof("Successfully patched Knative Service %s in namespace %s, new revision will be created", service.GetName(), namespace)
	return nil
}

// GetKnativeServicePatchTemplates returns patch templates for Knative Services
func GetKnativeServicePatchTemplates() PatchTemplates {
	return PatchTemplates{
		AnnotationTemplate:   `{"spec":{"template":{"metadata":{"annotations":{"%s":"%s"}}}}}`,                                   // strategic merge patch
		EnvVarTemplate:       `{"spec":{"template":{"spec":{"containers":[{"name":"%s","env":[{"name":"%s","value":"%s"}]}]}}}}`, // strategic merge patch
		DeleteEnvVarTemplate: `[{"op":"remove","path":"/spec/template/spec/containers/%d/env/%d"}]`,                              // JSON patch
	}
}
