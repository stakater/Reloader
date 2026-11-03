package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
)

// dependencyGracePeriod is how long a dependency may show neither the change nor
// a pending entry for it before it is taken to have skipped the reload. It covers
// the informer cache catching up with the dependency's own update.
const dependencyGracePeriod = 30 * time.Second

// dependencyState summarizes the dependencies of one pending reload.
type dependencyState int

const (
	dependenciesWaiting dependencyState = iota
	dependenciesDone
	dependenciesFailed
)

// DependentReloadReconciler applies reloads that were deferred by the reload-after
// annotation once the listed dependencies finished reloading. One instance runs
// per workload kind and only sees workloads carrying the pending-reload annotation.
type DependentReloadReconciler struct {
	Client        client.Client
	Log           logr.Logger
	Config        *config.Config
	Kind          workload.Kind
	Registry      *workload.Registry
	ReloadService *reload.Service
	Handler       *ReloadHandler
	EventRecorder *events.Recorder
	Collectors    *metrics.Collectors
}

// Reconcile resolves at most one pending reload per call: a reload changes the
// workload, so later entries are handled on the next pass with a fresh object.
func (r *DependentReloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues(strings.ToLower(string(r.Kind)), req.NamespacedName)

	obj := r.Registry.NewObject(r.Kind)
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	pending, err := reload.ParsePendingReloads(obj.GetAnnotations()[r.Config.Annotations.PendingReload])
	if err != nil {
		log.Error(err, "discarding unreadable pending reloads")
		return ctrl.Result{}, r.clearPending(ctx, obj, nil)
	}
	if len(pending) == 0 {
		return ctrl.Result{}, nil
	}

	wl, err := r.Registry.FromObject(obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	now := time.Now()
	resolved := map[string]string{}
	for _, key := range pending.SortedKeys() {
		p := pending[key]
		state, reason, err := r.dependenciesState(ctx, req.Namespace, p, now)
		if err != nil {
			if now.Before(p.Deadline) {
				return ctrl.Result{}, err
			}
			// Errors that outlast the timeout (e.g. missing RBAC) must not pin the entry forever.
			log.Error(err, "failed to check reload-after dependencies", "resource", key)
			state, reason = dependenciesWaiting, "dependencies that could not be checked"
		}

		switch state {
		case dependenciesDone:
			decision := reload.ReloadDecision{
				Workload:     wl,
				ShouldReload: true,
				AutoReload:   p.AutoReload,
				Reason:       "reload-after dependencies finished",
				Hash:         p.Hash,
			}
			failed := r.Handler.reloadNow(ctx, p.ResourceName, p.ResourceNamespace, p.ResourceType, []reload.ReloadDecision{decision}, log)
			if failed > 0 && now.Before(p.Deadline) {
				// Keep the entry and try again on the next pass.
				return ctrl.Result{RequeueAfter: r.Config.ReloadAfterPollInterval}, nil
			}
			if failed > 0 {
				r.skip(obj, p, "reload kept failing until the reload-after timeout", "dependency_timeout", log)
			}
			resolved[key] = p.Hash
			return r.finish(ctx, obj, resolved, len(pending))
		case dependenciesFailed:
			r.skip(obj, p, reason, "dependency_failed", log)
			resolved[key] = p.Hash
		case dependenciesWaiting:
			if now.After(p.Deadline) {
				r.skip(obj, p, "timed out waiting for "+reason, "dependency_timeout", log)
				resolved[key] = p.Hash
			} else {
				log.V(1).Info("waiting for reload-after dependencies", "resource", key, "waitingFor", reason)
			}
		}
	}

	return r.finish(ctx, obj, resolved, len(pending))
}

// finish drops the resolved entries and requeues while anything is still pending.
func (r *DependentReloadReconciler) finish(ctx context.Context, obj client.Object, resolved map[string]string, total int) (ctrl.Result, error) {
	if len(resolved) > 0 {
		if err := r.clearPending(ctx, obj, resolved); err != nil {
			return ctrl.Result{}, err
		}
	}
	if len(resolved) < total {
		return ctrl.Result{RequeueAfter: r.Config.ReloadAfterPollInterval}, nil
	}
	return ctrl.Result{}, nil
}

// dependenciesState checks every dependency of p. The returned reason names the
// dependency that is still awaited or that failed.
func (r *DependentReloadReconciler) dependenciesState(
	ctx context.Context,
	namespace string,
	p reload.PendingReload,
	now time.Time,
) (dependencyState, string, error) {
	applied := r.ReloadService.AppliedCheck(p.ResourceName, p.ResourceType, p.Hash)

	for _, dep := range p.Dependencies {
		obj := r.Registry.NewObject(dep.Kind)
		if obj == nil {
			return dependenciesFailed, fmt.Sprintf("dependency %s is of a kind that is not enabled", dep), nil
		}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: dep.Name}, obj); err != nil {
			if !errors.IsNotFound(err) {
				return dependenciesWaiting, "", err
			}
			// A reloaded Job is deleted and recreated, so it can briefly be missing.
			if dep.Kind == workload.KindJob && now.Sub(p.Since) < dependencyGracePeriod {
				return dependenciesWaiting, dep.String(), nil
			}
			return dependenciesFailed, fmt.Sprintf("dependency %s not found", dep), nil
		}
		depWl, err := r.Registry.FromObject(obj)
		if err != nil {
			return dependenciesFailed, fmt.Sprintf("dependency %s: %v", dep, err), nil
		}

		progress, err := depWl.ReloadProgress(ctx, r.Client, applied)
		if err != nil {
			return dependenciesWaiting, "", err
		}
		switch progress {
		case workload.ProgressDone:
			continue
		case workload.ProgressFailed:
			return dependenciesFailed, fmt.Sprintf("dependency %s failed", dep), nil
		case workload.ProgressRunning:
			return dependenciesWaiting, dep.String(), nil
		case workload.ProgressNotApplied:
			if r.hasPending(depWl, p) || now.Sub(p.Since) < dependencyGracePeriod {
				return dependenciesWaiting, dep.String(), nil
			}
			return dependenciesFailed, fmt.Sprintf("dependency %s did not reload", dep), nil
		}
	}
	return dependenciesDone, "", nil
}

// hasPending reports whether the dependency itself waits to apply the same change.
func (r *DependentReloadReconciler) hasPending(dep workload.Workload, p reload.PendingReload) bool {
	pending, err := reload.ParsePendingReloads(dep.GetAnnotations()[r.Config.Annotations.PendingReload])
	if err != nil {
		return false
	}
	entry, ok := pending[p.Key()]
	return ok && entry.Hash == p.Hash
}

func (r *DependentReloadReconciler) skip(obj client.Object, p reload.PendingReload, reason, metric string, log logr.Logger) {
	log.Info(
		"skipping deferred reload", "resource", p.Key(), "reason", reason,
	)
	r.EventRecorder.ReloadSkipped(obj, p.ResourceType.Kind(), p.ResourceName, reason)
	r.Collectors.RecordSkipped(metric)
}

// clearPending removes the resolved entries (key → hash) from the pending-reload
// annotation; a nil map removes the annotation. An entry replaced by a newer
// change in the meantime has a different hash and is kept.
func (r *DependentReloadReconciler) clearPending(ctx context.Context, obj client.Object, resolved map[string]string) error {
	key := r.Config.Annotations.PendingReload
	return UpdateObjectWithRetry(
		ctx, r.Client, obj, func() (bool, error) {
			annotations := obj.GetAnnotations()
			if _, ok := annotations[key]; !ok {
				return false, nil
			}
			pending, err := reload.ParsePendingReloads(annotations[key])
			if err != nil || resolved == nil {
				delete(annotations, key)
				obj.SetAnnotations(annotations)
				return true, nil
			}
			changed := false
			for k, hash := range resolved {
				if entry, ok := pending[k]; ok && entry.Hash == hash {
					delete(pending, k)
					changed = true
				}
			}
			if !changed {
				return false, nil
			}
			value, err := pending.Encode()
			if err != nil {
				return false, err
			}
			if value == "" {
				delete(annotations, key)
			} else {
				annotations[key] = value
			}
			obj.SetAnnotations(annotations)
			return true, nil
		},
	)
}

// SetupWithManager registers the reconciler for its workload kind.
func (r *DependentReloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	key := r.Config.Annotations.PendingReload
	return ctrl.NewControllerManagedBy(mgr).
		Named("reload-after-" + strings.ToLower(string(r.Kind))).
		For(r.Registry.NewObject(r.Kind)).
		WithEventFilter(
			predicate.NewPredicateFuncs(
				func(obj client.Object) bool {
					_, ok := obj.GetAnnotations()[key]
					return ok
				},
			),
		).
		Complete(r)
}
