package controller

import (
	"context"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateWorkloadWithRetry updates a workload with exponential backoff on conflict.
// On conflict, it re-fetches the object, re-applies the reload changes, and retries.
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
	var updated bool
	isFirstAttempt := true

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// On retry, re-fetch the object to get the latest ResourceVersion
		if !isFirstAttempt {
			obj := wl.GetObject()
			key := client.ObjectKeyFromObject(obj)
			if err := c.Get(ctx, key, obj); err != nil {
				if errors.IsNotFound(err) {
					// Object was deleted, nothing to update
					return nil
				}
				return err
			}
		}
		isFirstAttempt = false

		// Apply reload changes (this modifies the workload in-place)
		var applyErr error
		updated, applyErr = reloadService.ApplyReload(
			ctx,
			wl,
			resourceName,
			resourceType,
			namespace,
			hash,
			autoReload,
		)
		if applyErr != nil {
			return applyErr
		}

		if !updated {
			return nil
		}

		// Attempt update with field ownership
		return c.Update(ctx, wl.GetObject(), client.FieldOwner(FieldManager))
	})

	return updated, err
}
