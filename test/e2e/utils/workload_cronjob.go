package utils

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// CronJobAdapter implements WorkloadAdapter for Kubernetes CronJobs.
type CronJobAdapter struct {
	client kubernetes.Interface
}

// NewCronJobAdapter creates a new CronJobAdapter.
func NewCronJobAdapter(client kubernetes.Interface) *CronJobAdapter {
	return &CronJobAdapter{client: client}
}

// Type returns the workload type.
func (a *CronJobAdapter) Type() WorkloadType {
	return WorkloadCronJob
}

// Create creates a CronJob with the given config.
func (a *CronJobAdapter) Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error {
	opts := buildCronJobOptions(cfg)
	_, err := CreateCronJob(ctx, a.client, namespace, name, opts...)
	return err
}

// Delete removes the CronJob.
func (a *CronJobAdapter) Delete(ctx context.Context, namespace, name string) error {
	return DeleteCronJob(ctx, a.client, namespace, name)
}

// WaitReady waits for the CronJob to exist using watches.
func (a *CronJobAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.BatchV1().CronJobs(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, Always[*batchv1.CronJob](), timeout)
	return err
}

// WaitReloaded waits for the CronJob pod template to have the reload annotation using watches.
func (a *CronJobAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.BatchV1().CronJobs(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, name, HasPodTemplateAnnotation(CronJobPodTemplate, annotationKey), timeout)
	return HandleWatchResult(err)
}

// WaitEnvVar returns an error because CronJobs don't support env var reload strategy.
func (a *CronJobAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	return false, ErrUnsupportedOperation
}

// SupportsEnvVarStrategy returns false as CronJobs don't support env var reload strategy.
func (a *CronJobAdapter) SupportsEnvVarStrategy() bool {
	return false
}

// RequiresSpecialHandling returns true as CronJobs use job triggering instead of rolling restart.
func (a *CronJobAdapter) RequiresSpecialHandling() bool {
	return true
}

// WaitForTriggeredJob waits for Reloader to trigger a new Job from this CronJob using watches.
func (a *CronJobAdapter) WaitForTriggeredJob(ctx context.Context, namespace, cronJobName string, timeout time.Duration) (bool, error) {
	watchFunc := func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return a.client.BatchV1().Jobs(namespace).Watch(ctx, opts)
	}
	_, err := WatchUntil(ctx, watchFunc, "", IsTriggeredJobForCronJob(cronJobName), timeout)
	return HandleWatchResult(err)
}

// buildCronJobOptions converts WorkloadConfig to CronJobOption slice.
func buildCronJobOptions(cfg WorkloadConfig) []CronJobOption {
	return []CronJobOption{
		func(cj *batchv1.CronJob) {
			// Set annotations on CronJob level (where Reloader checks them)
			if len(cfg.Annotations) > 0 {
				if cj.Annotations == nil {
					cj.Annotations = make(map[string]string)
				}
				for k, v := range cfg.Annotations {
					cj.Annotations[k] = v
				}
			}
			// CronJob has nested JobTemplate
			ApplyWorkloadConfig(&cj.Spec.JobTemplate.Spec.Template, cfg)
		},
	}
}
