package utils

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	openshiftappsv1 "github.com/openshift/api/apps/v1"
)

// Deployment accessors
var (
	DeploymentPodTemplate PodTemplateAccessor[*appsv1.Deployment] = func(d *appsv1.Deployment) *corev1.PodTemplateSpec {
		return &d.Spec.Template
	}
	DeploymentAnnotations AnnotationAccessor[*appsv1.Deployment] = func(d *appsv1.Deployment) map[string]string {
		return d.Annotations
	}
	DeploymentContainers ContainerAccessor[*appsv1.Deployment] = func(d *appsv1.Deployment) []corev1.Container {
		return d.Spec.Template.Spec.Containers
	}
	DeploymentIsReady StatusAccessor[*appsv1.Deployment] = func(d *appsv1.Deployment) bool {
		if d.Spec.Replicas == nil {
			return false
		}
		return d.Status.ReadyReplicas == *d.Spec.Replicas &&
			d.Status.UpdatedReplicas == *d.Spec.Replicas &&
			d.Status.AvailableReplicas == *d.Spec.Replicas
	}
)

// DaemonSet accessors
var (
	DaemonSetPodTemplate PodTemplateAccessor[*appsv1.DaemonSet] = func(d *appsv1.DaemonSet) *corev1.PodTemplateSpec {
		return &d.Spec.Template
	}
	DaemonSetAnnotations AnnotationAccessor[*appsv1.DaemonSet] = func(d *appsv1.DaemonSet) map[string]string {
		return d.Annotations
	}
	DaemonSetContainers ContainerAccessor[*appsv1.DaemonSet] = func(d *appsv1.DaemonSet) []corev1.Container {
		return d.Spec.Template.Spec.Containers
	}
	DaemonSetIsReady StatusAccessor[*appsv1.DaemonSet] = func(d *appsv1.DaemonSet) bool {
		return d.Status.DesiredNumberScheduled > 0 &&
			d.Status.NumberReady == d.Status.DesiredNumberScheduled
	}
)

// StatefulSet accessors
var (
	StatefulSetPodTemplate PodTemplateAccessor[*appsv1.StatefulSet] = func(s *appsv1.StatefulSet) *corev1.PodTemplateSpec {
		return &s.Spec.Template
	}
	StatefulSetAnnotations AnnotationAccessor[*appsv1.StatefulSet] = func(s *appsv1.StatefulSet) map[string]string {
		return s.Annotations
	}
	StatefulSetContainers ContainerAccessor[*appsv1.StatefulSet] = func(s *appsv1.StatefulSet) []corev1.Container {
		return s.Spec.Template.Spec.Containers
	}
	StatefulSetIsReady StatusAccessor[*appsv1.StatefulSet] = func(s *appsv1.StatefulSet) bool {
		if s.Spec.Replicas == nil {
			return false
		}
		return s.Status.ReadyReplicas == *s.Spec.Replicas
	}
)

// Job accessors
var (
	JobPodTemplate PodTemplateAccessor[*batchv1.Job] = func(j *batchv1.Job) *corev1.PodTemplateSpec {
		return &j.Spec.Template
	}
	JobAnnotations AnnotationAccessor[*batchv1.Job] = func(j *batchv1.Job) map[string]string {
		return j.Annotations
	}
	JobContainers ContainerAccessor[*batchv1.Job] = func(j *batchv1.Job) []corev1.Container {
		return j.Spec.Template.Spec.Containers
	}
	JobIsReady StatusAccessor[*batchv1.Job] = func(j *batchv1.Job) bool {
		return j.Status.Active > 0 || j.Status.Succeeded > 0
	}
	JobUID UIDAccessor[*batchv1.Job] = func(j *batchv1.Job) types.UID {
		return j.UID
	}
)

// CronJob accessors
var (
	CronJobPodTemplate PodTemplateAccessor[*batchv1.CronJob] = func(c *batchv1.CronJob) *corev1.PodTemplateSpec {
		return &c.Spec.JobTemplate.Spec.Template
	}
	CronJobAnnotations AnnotationAccessor[*batchv1.CronJob] = func(c *batchv1.CronJob) map[string]string {
		return c.Annotations
	}
	CronJobContainers ContainerAccessor[*batchv1.CronJob] = func(c *batchv1.CronJob) []corev1.Container {
		return c.Spec.JobTemplate.Spec.Template.Spec.Containers
	}
	CronJobExists StatusAccessor[*batchv1.CronJob] = func(c *batchv1.CronJob) bool {
		return true
	}
)

// Argo Rollout accessors
var (
	RolloutPodTemplate PodTemplateAccessor[*rolloutsv1alpha1.Rollout] = func(r *rolloutsv1alpha1.Rollout) *corev1.PodTemplateSpec {
		return &r.Spec.Template
	}
	RolloutAnnotations AnnotationAccessor[*rolloutsv1alpha1.Rollout] = func(r *rolloutsv1alpha1.Rollout) map[string]string {
		return r.Annotations
	}
	RolloutContainers ContainerAccessor[*rolloutsv1alpha1.Rollout] = func(r *rolloutsv1alpha1.Rollout) []corev1.Container {
		return r.Spec.Template.Spec.Containers
	}
	RolloutIsReady StatusAccessor[*rolloutsv1alpha1.Rollout] = func(r *rolloutsv1alpha1.Rollout) bool {
		if r.Spec.Replicas == nil {
			return false
		}
		return r.Status.ReadyReplicas == *r.Spec.Replicas
	}
	RolloutHasRestartAt StatusAccessor[*rolloutsv1alpha1.Rollout] = func(r *rolloutsv1alpha1.Rollout) bool {
		return r.Spec.RestartAt != nil
	}
)

// OpenShift DeploymentConfig accessors
var (
	DeploymentConfigPodTemplate PodTemplateAccessor[*openshiftappsv1.DeploymentConfig] = func(d *openshiftappsv1.DeploymentConfig) *corev1.PodTemplateSpec {
		return d.Spec.Template
	}
	DeploymentConfigAnnotations AnnotationAccessor[*openshiftappsv1.DeploymentConfig] = func(d *openshiftappsv1.DeploymentConfig) map[string]string {
		return d.Annotations
	}
	DeploymentConfigContainers ContainerAccessor[*openshiftappsv1.DeploymentConfig] = func(d *openshiftappsv1.DeploymentConfig) []corev1.Container {
		if d.Spec.Template == nil {
			return nil
		}
		return d.Spec.Template.Spec.Containers
	}
	DeploymentConfigIsReady StatusAccessor[*openshiftappsv1.DeploymentConfig] = func(d *openshiftappsv1.DeploymentConfig) bool {
		return d.Status.ReadyReplicas == d.Spec.Replicas
	}
)

// SecretProviderClassPodStatus accessors
var (
	SPCPSIsMounted StatusAccessor[*csiv1.SecretProviderClassPodStatus] = func(s *csiv1.SecretProviderClassPodStatus) bool {
		return s.Status.Mounted
	}
	SPCPSClassName ValueAccessor[*csiv1.SecretProviderClassPodStatus, string] = func(s *csiv1.SecretProviderClassPodStatus) string {
		return s.Status.SecretProviderClassName
	}
	SPCPSPodName ValueAccessor[*csiv1.SecretProviderClassPodStatus, string] = func(s *csiv1.SecretProviderClassPodStatus) string {
		return s.Status.PodName
	}
	// SPCPSVersions returns concatenated versions of all objects for change detection.
	SPCPSVersions ValueAccessor[*csiv1.SecretProviderClassPodStatus, string] = func(s *csiv1.SecretProviderClassPodStatus) string {
		if len(s.Status.Objects) == 0 {
			return ""
		}
		var versions []string
		for _, obj := range s.Status.Objects {
			versions = append(versions, obj.Version)
		}
		return strings.Join(versions, ",")
	}
)
