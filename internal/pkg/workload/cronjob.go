package workload

import (
	"context"
	"maps"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cronJobAccessor implements PodTemplateAccessor for CronJob.
type cronJobAccessor struct {
	cronjob *batchv1.CronJob
}

func (a *cronJobAccessor) GetPodTemplateSpec() *corev1.PodTemplateSpec {
	// CronJob has the pod template nested under JobTemplate.Spec.Template
	return &a.cronjob.Spec.JobTemplate.Spec.Template
}

func (a *cronJobAccessor) GetObjectMeta() *metav1.ObjectMeta {
	return &a.cronjob.ObjectMeta
}

// CronJobWorkload wraps a Kubernetes CronJob.
// Note: CronJobs have a special update mechanism - instead of updating the CronJob itself,
// Reloader creates a new Job from the CronJob's template.
type CronJobWorkload struct {
	*BaseWorkload[*batchv1.CronJob]
}

// NewCronJobWorkload creates a new CronJobWorkload.
func NewCronJobWorkload(c *batchv1.CronJob) *CronJobWorkload {
	original := c.DeepCopy()
	accessor := &cronJobAccessor{cronjob: c}
	return &CronJobWorkload{
		BaseWorkload: NewBaseWorkload(c, original, accessor, KindCronJob),
	}
}

// Ensure CronJobWorkload implements Workload.
var _ Workload = (*CronJobWorkload)(nil)

// Update for CronJob is a no-op - use PerformSpecialUpdate instead.
// CronJobs trigger reloads by creating a new Job from their template.
func (w *CronJobWorkload) Update(ctx context.Context, c client.Client) error {
	// CronJobs don't get updated directly - a new Job is created instead
	// This is handled by PerformSpecialUpdate
	return nil
}

// ResetOriginal is a no-op for CronJobs since they don't use strategic merge patch.
// CronJobs create new Jobs instead of being patched.
func (w *CronJobWorkload) ResetOriginal() {}

func (w *CronJobWorkload) UpdateStrategy() UpdateStrategy {
	return UpdateStrategyCreateNew
}

// PerformSpecialUpdate creates a new Job from the CronJob's template.
// This triggers an immediate execution of the CronJob with updated config.
func (w *CronJobWorkload) PerformSpecialUpdate(ctx context.Context, c client.Client) (bool, error) {
	cronJob := w.Object()

	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	maps.Copy(annotations, cronJob.Spec.JobTemplate.Annotations)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cronJob.Name + "-",
			Namespace:    cronJob.Namespace,
			Annotations:  annotations,
			Labels:       cronJob.Spec.JobTemplate.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cronJob, batchv1.SchemeGroupVersion.WithKind("CronJob")),
			},
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

	if err := c.Create(ctx, job, client.FieldOwner(FieldManager)); err != nil {
		return false, err
	}

	return true, nil
}

func (w *CronJobWorkload) DeepCopy() Workload {
	return NewCronJobWorkload(w.Object().DeepCopy())
}

// GetCronJob returns the underlying CronJob for special handling.
func (w *CronJobWorkload) GetCronJob() *batchv1.CronJob {
	return w.Object()
}
