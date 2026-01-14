package utils

import (
	"context"
	"errors"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// JobAdapter implements WorkloadAdapter for Kubernetes Jobs.
// Note: Jobs are handled specially by Reloader - they are recreated rather than updated.
type JobAdapter struct {
	client kubernetes.Interface
}

// NewJobAdapter creates a new JobAdapter.
func NewJobAdapter(client kubernetes.Interface) *JobAdapter {
	return &JobAdapter{client: client}
}

// Type returns the workload type.
func (a *JobAdapter) Type() WorkloadType {
	return WorkloadJob
}

// Create creates a Job with the given config.
func (a *JobAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildJobOptions(cfg)
	_, err := CreateJob(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the Job.
func (a *JobAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteJob(ctx, a.client, namespace, name)
}

// WaitReady waits for the Job to be ready (has active or succeeded pods) using watches.
func (a *JobAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.BatchV1().Jobs(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, IsReady(JobIsReady), timeout)
	return err
}

// WaitReloaded waits for the Job to be recreated (new UID) using watches.
// For Jobs, Reloader recreates the Job rather than updating annotations.
func (a *JobAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	// For Jobs, we check if it was recreated by looking for a new UID
	// This requires storing the original UID before the test
	// For simplicity, we use the same pattern as other workloads
	// The test should verify recreation using WaitForRecreation instead
	return false, nil
}

// WaitEnvVar is not supported for Jobs as they don't use env var reload strategy.
func (a *JobAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return false, nil
}

// WaitRecreated waits for the Job to be recreated with a different UID using watches.
func (a *JobAdapter) WaitRecreated(ctx context.Context, namespace, name, originalUID string, timeout time.Duration) (string, bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.BatchV1().Jobs(namespace).Watch(ctx, opts)
	}
	job, err := WatchUntil(ctx, watchFunc, name, HasDifferentUID(JobUID, types.UID(originalUID)), timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(job.UID), true, nil
}

// SupportsEnvVarStrategy returns false as Jobs don't support env var reload strategy.
func (a *JobAdapter) SupportsEnvVarStrategy() bool {
	return false
}

// RequiresSpecialHandling returns true as Jobs are recreated by Reloader.
func (a *JobAdapter) RequiresSpecialHandling() bool {
	return true
}

// GetOriginalUID retrieves the current UID of the Job for recreation verification.
func (a *JobAdapter) GetOriginalUID(ctx context.Context, namespace, name string) (string, error) {
	job, err := a.client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(job.UID), nil
}

// buildJobOptions converts WorkloadConfig to JobOption slice.
func buildJobOptions(cfg WorkloadConfig) []JobOption {
	return []JobOption{
		func(job *batchv1.Job) {
			// Set annotations on Job level (where Reloader checks them)
			if len(cfg.Annotations) > 0 {
				if job.Annotations == nil {
					job.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					job.Annotations[k] = v
				}
			}
			ApplyWorkloadConfig(&job.Spec.Template, cfg)
		},
	}
}
