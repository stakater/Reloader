package workload

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// jobAccessor implements PodTemplateAccessor for Job.
type jobAccessor struct {
	job *batchv1.Job
}

func (a *jobAccessor) GetPodTemplateSpec() *corev1.PodTemplateSpec {
	return &a.job.Spec.Template
}

func (a *jobAccessor) GetObjectMeta() *metav1.ObjectMeta {
	return &a.job.ObjectMeta
}

// JobWorkload wraps a Kubernetes Job.
// Note: Jobs have a special update mechanism - instead of updating the Job,
// Reloader deletes and recreates it with the same spec.
type JobWorkload struct {
	*BaseWorkload[*batchv1.Job]
}

// NewJobWorkload creates a new JobWorkload.
func NewJobWorkload(j *batchv1.Job) *JobWorkload {
	original := j.DeepCopy()
	accessor := &jobAccessor{job: j}
	return &JobWorkload{
		BaseWorkload: NewBaseWorkload(j, original, accessor, KindJob),
	}
}

// Ensure JobWorkload implements Workload.
var _ Workload = (*JobWorkload)(nil)

// Update for Job is a no-op - use PerformSpecialUpdate instead.
// Jobs trigger reloads by being deleted and recreated.
func (w *JobWorkload) Update(ctx context.Context, c client.Client) error {
	// Jobs don't get updated directly - they are deleted and recreated
	// This is handled by PerformSpecialUpdate
	return nil
}

// ResetOriginal is a no-op for Jobs since they don't use strategic merge patch.
// Jobs are deleted and recreated instead of being patched.
func (w *JobWorkload) ResetOriginal() {}

func (w *JobWorkload) UpdateStrategy() UpdateStrategy {
	return UpdateStrategyRecreate
}

// PerformSpecialUpdate deletes the Job and recreates it with the updated spec.
// This is necessary because Jobs are immutable after creation.
func (w *JobWorkload) PerformSpecialUpdate(ctx context.Context, c client.Client) (bool, error) {
	oldJob := w.Object()
	newJob := oldJob.DeepCopy()

	// Delete the old job with background propagation
	policy := metav1.DeletePropagationBackground
	if err := c.Delete(ctx, oldJob, &client.DeleteOptions{
		PropagationPolicy: &policy,
	}); err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}
	}

	// Clear fields that should not be specified when creating a new Job
	newJob.ResourceVersion = ""
	newJob.UID = ""
	newJob.CreationTimestamp = metav1.Time{}
	newJob.Status = batchv1.JobStatus{}

	// Remove problematic labels that are auto-generated
	delete(newJob.Spec.Template.Labels, "controller-uid")
	delete(newJob.Spec.Template.Labels, batchv1.ControllerUidLabel)
	delete(newJob.Spec.Template.Labels, batchv1.JobNameLabel)
	delete(newJob.Spec.Template.Labels, "job-name")

	// Remove the selector to allow it to be auto-generated
	newJob.Spec.Selector = nil

	// Create the new job with same spec
	if err := c.Create(ctx, newJob, client.FieldOwner(FieldManager)); err != nil {
		return false, err
	}

	return true, nil
}

func (w *JobWorkload) DeepCopy() Workload {
	return NewJobWorkload(w.Object().DeepCopy())
}

// GetJob returns the underlying Job for special handling.
func (w *JobWorkload) GetJob() *batchv1.Job {
	return w.Object()
}
