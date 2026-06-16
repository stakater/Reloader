package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
)

// UpdateObjectWithRetry updates a Kubernetes object with retry on conflict.
// It re-fetches the object on each retry attempt and calls modifyFn to apply changes.
// The modifyFn receives the latest version of the object and should modify it in place.
// If modifyFn returns false, the update is skipped (e.g., if the condition no longer applies).
func UpdateObjectWithRetry(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	modifyFn func() (shouldUpdate bool, err error),
) error {
	return retry.RetryOnConflict(
		retry.DefaultBackoff, func() error {
			if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}

			shouldUpdate, err := modifyFn()
			if err != nil {
				return err
			}

			if !shouldUpdate {
				return nil
			}

			return c.Update(ctx, obj, client.FieldOwner(workload.FieldManager))
		},
	)
}

// UpdateWorkloadWithRetry updates a workload with exponential backoff on conflict.
// On conflict, it re-fetches the object, re-applies the reload changes, and retries.
// Workloads use their UpdateStrategy to determine how they're updated:
// - UpdateStrategyPatch: uses strategic merge patch with retry (most workloads)
// - UpdateStrategyRecreate: deletes and recreates (Jobs)
// - UpdateStrategyCreateNew: creates a new resource from template (CronJobs)
// Deployments have additional pause handling for paused rollouts.
func UpdateWorkloadWithRetry(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	pauseHandler *reload.PauseHandler,
	wl workload.Workload,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	switch wl.UpdateStrategy() {
	case workload.UpdateStrategyRecreate, workload.UpdateStrategyCreateNew:
		return updateWithSpecialStrategy(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload)
	default:
		// UpdateStrategyPatch: use standard retry logic with special handling for Deployments
		if wl.Kind() == workload.KindDeployment {
			return updateDeploymentWithPause(ctx, c, reloadService, pauseHandler, wl, resourceName, resourceType, namespace, hash, autoReload)
		}
		return updateStandardWorkload(ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload)
	}
}

// retryWithReload wraps the common retry logic for workload updates.
// It handles re-fetching on conflict, applying reload changes, and calling the update function.
func retryWithReload(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.Workload,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
	updateFn func() error,
) (bool, error) {
	var updated bool
	isFirstAttempt := true

	err := retry.RetryOnConflict(
		retry.DefaultBackoff, func() error {
			if !isFirstAttempt {
				obj := wl.GetObject()
				key := client.ObjectKeyFromObject(obj)
				if err := c.Get(ctx, key, obj); err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				wl.ResetOriginal()
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
		},
	)

	return updated, err
}

// updateStandardWorkload updates DaemonSets, StatefulSets, etc.
func updateStandardWorkload(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.Workload,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	return retryWithReload(
		ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload,
		func() error {
			return wl.Update(ctx, c)
		},
	)
}

// updateDeploymentWithPause updates a Deployment and applies pause if configured.
func updateDeploymentWithPause(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	pauseHandler *reload.PauseHandler,
	wl workload.Workload,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	shouldPause := pauseHandler != nil && pauseHandler.ShouldPause(wl)

	return retryWithReload(
		ctx, c, reloadService, wl, resourceName, resourceType, namespace, hash, autoReload,
		func() error {
			if shouldPause {
				if err := pauseHandler.ApplyPause(wl); err != nil {
					return err
				}
			}
			return wl.Update(ctx, c)
		},
	)
}

// updateWithSpecialStrategy handles workloads that don't use standard patch.
// It applies reload changes, then delegates to the workload's PerformSpecialUpdate.
func updateWithSpecialStrategy(
	ctx context.Context,
	c client.Client,
	reloadService *reload.Service,
	wl workload.Workload,
	resourceName string,
	resourceType reload.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
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

	return wl.PerformSpecialUpdate(ctx, c)
}
