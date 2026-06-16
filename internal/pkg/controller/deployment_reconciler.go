package controller

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/reload"
)

// DeploymentReconciler reconciles Deployment objects to handle pause expiration.
// This reconciler watches for deployments that were paused by Reloader and
// unpauses them when the pause period expires.
type DeploymentReconciler struct {
	client.Client
	Log          logr.Logger
	Config       *config.Config
	PauseHandler *reload.PauseHandler
}

// Reconcile handles Deployment pause expiration.
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("deployment", req.NamespacedName)
	log.V(1).Info("reconciling deployment", "namespace", req.Namespace, "name", req.Name)

	var deploy appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deploy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if this deployment was paused by Reloader
	if !r.PauseHandler.IsPausedByReloader(&deploy) {
		return ctrl.Result{}, nil
	}

	// Check if pause period has expired
	expired, remainingTime, err := r.PauseHandler.CheckPauseExpired(&deploy)
	if err != nil {
		log.Error(err, "Failed to check pause expiration")
		return ctrl.Result{}, err
	}

	if !expired {
		// Still within pause period - requeue to check again
		log.V(1).Info("Deployment pause not yet expired", "remaining", remainingTime)
		return ctrl.Result{RequeueAfter: remainingTime}, nil
	}

	log.Info("Unpausing deployment after pause period expired")
	err = UpdateObjectWithRetry(
		ctx, r.Client, &deploy, func() (bool, error) {
			if !r.PauseHandler.IsPausedByReloader(&deploy) {
				return false, nil
			}
			r.PauseHandler.ClearPause(&deploy)
			return true, nil
		},
	)

	if err != nil {
		log.Error(err, "Failed to unpause deployment")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the DeploymentReconciler with the manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		WithEventFilter(r.pausedByReloaderPredicate()).
		Complete(r)
}

// pausedByReloaderPredicate returns a predicate that only selects deployments
// that have been paused by Reloader (have the paused-at annotation).
func (r *DeploymentReconciler) pausedByReloaderPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(
		func(obj client.Object) bool {
			annotations := obj.GetAnnotations()
			if annotations == nil {
				return false
			}

			// Only process if deployment has our paused-at annotation
			_, hasPausedAt := annotations[r.Config.Annotations.PausedAt]
			return hasPausedAt
		},
	)
}
