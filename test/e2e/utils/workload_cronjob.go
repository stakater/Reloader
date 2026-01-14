package utils

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
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

// WaitReady waits for the CronJob to exist (CronJobs are "ready" immediately after creation).
func (a *CronJobAdapter) WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return WaitForCronJobExists(ctx, a.client, namespace, name, timeout)
}

// WaitReloaded waits for the CronJob to have the reload annotation OR for a triggered Job.
func (a *CronJobAdapter) WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	return WaitForCronJobReloaded(ctx, a.client, namespace, name, annotationKey, timeout)
}

// WaitEnvVar is not supported for CronJobs as they don't use env var reload strategy.
func (a *CronJobAdapter) WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	// CronJobs don't support env var strategy
	return false, nil
}

// SupportsEnvVarStrategy returns false as CronJobs don't support env var reload strategy.
func (a *CronJobAdapter) SupportsEnvVarStrategy() bool {
	return false
}

// RequiresSpecialHandling returns true as CronJobs use job triggering instead of rolling restart.
func (a *CronJobAdapter) RequiresSpecialHandling() bool {
	return true
}

// WaitForTriggeredJob waits for Reloader to trigger a new Job from this CronJob.
func (a *CronJobAdapter) WaitForTriggeredJob(ctx context.Context, namespace, cronJobName string, timeout time.Duration) (bool, error) {
	return WaitForCronJobTriggeredJob(ctx, a.client, namespace, cronJobName, timeout)
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
