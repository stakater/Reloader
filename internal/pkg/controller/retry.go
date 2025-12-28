package controller

import (
	"context"
	"maps"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateWorkloadWithRetry updates a workload with exponential backoff on conflict.
// On conflict, it re-fetches the object, re-applies the reload changes, and retries.
// For Jobs and CronJobs, special handling is applied:
// - Jobs are deleted and recreated with the same spec
// - CronJobs create a new Job from their template
// For Argo Rollouts, special handling is applied based on the rollout strategy annotation.
func UpdateWorkloadWithRetry(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	// Handle special workload types
	switch wl.Kind() {
	case workload.KindJob:
		return updateJobWithRecreate(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload)
	case workload.KindCronJob:
		return updateCronJobWithNewJob(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload)
	case workload.KindArgoRollout:
		return updateArgoRollout(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload)
	default:
		return updateStandardWorkload(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload)
	}
}

// retryWithReload wraps the common retry logic for workload updates.
// It handles re-fetching on conflict, applying reload changes, and calling the update function.
func retryWithReload(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
	updateFn func() error,
) (bool, error) {
	var updated bool
	isFirstAttempt := true

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if !isFirstAttempt {
			obj := wl.GetObject()
			key := client.ObjectKeyFromObject(obj)
			if err := c.Get(ctx, key, obj); err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}
		}
		isFirstAttempt = false

		var applyErr error
		updated, applyErr = reloadService.ApplyReload(ctx, wl, resourceName, resourceType, namespace, hash, autoReload)
		if applyErr != nil {
			return applyErr
		}

		if !updated {
			return nil
		}

		return updateFn()
	})

	return updated, err
}

// updateStandardWorkload updates Deployments, DaemonSets, StatefulSets, etc.
func updateStandardWorkload(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	return retryWithReload(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload,
		func() error {
			return c.Update(ctx, wl.GetObject(), client.FieldOwner(FieldManager))
		})
}

// updateJobWithRecreate deletes the Job and recreates it with the updated spec.
// Jobs are immutable after creation, so we must delete and recreate.
func updateJobWithRecreate(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	jobWl, ok := wl.(*workload.JobWorkload)
	if !ok {
		return false, nil
	}

	// Apply reload changes to the workload
	updated, err := reloadService.ApplyReload(
		ctx,
		wl,
		resourceName,
		resourceType,
		namespace,
		hash,
		autoReload,
	)
	if err != nil {
		return false, err
	}

	if !updated {
		return false, nil
	}

	oldJob := jobWl.GetJob()
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

// updateCronJobWithNewJob creates a new Job from the CronJob's template.
// CronJobs don't get updated directly; instead, a new Job is triggered.
func updateCronJobWithNewJob(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	cronJobWl, ok := wl.(*workload.CronJobWorkload)
	if !ok {
		return false, nil
	}

	// Apply reload changes to get the updated spec
	updated, err := reloadService.ApplyReload(
		ctx,
		wl,
		resourceName,
		resourceType,
		namespace,
		hash,
		autoReload,
	)
	if err != nil {
		return false, err
	}

	if !updated {
		return false, nil
	}

	cronJob := cronJobWl.GetCronJob()

	// Build annotations for the new Job
	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	maps.Copy(annotations, cronJob.Spec.JobTemplate.Annotations)

	// Create a new Job from the CronJob template
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

// updateArgoRollout updates an Argo Rollout using its custom Update method.
// This handles the rollout strategy annotation to determine whether to do
// a standard rollout or set the restartAt field.
func updateArgoRollout(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	rolloutWl, ok := wl.(*workload.RolloutWorkload)
	if !ok {
		return false, nil
	}

	return retryWithReload(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload,
		func() error {
			return rolloutWl.Update(ctx, c)
		})
}
