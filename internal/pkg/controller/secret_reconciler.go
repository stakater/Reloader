package controller

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SecretReconciler watches Secrets and triggers workload reloads.
type SecretReconciler struct {
	client.Client
	Log           logr.Logger
	Config        *config.Config
	ReloadService *reload.Service
	Registry      *workload.Registry
	Collectors    *metrics.Collectors

	// initialized tracks whether initial sync has completed.
	// Used to skip create events during startup unless SyncAfterRestart is enabled.
	initialized bool
	initOnce    sync.Once
}

// Reconcile handles Secret events and triggers workload reloads as needed.
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", req.NamespacedName)

	// Mark as initialized after first reconcile (caches are synced at this point)
	r.initOnce.Do(func() {
		r.initialized = true
		log.Info("Secret controller initialized")
	})

	// Fetch the Secret
	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			// Secret was deleted - handle if ReloadOnDelete is enabled
			if r.Config.ReloadOnDelete {
				return r.handleDelete(ctx, req, log)
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get Secret")
		return ctrl.Result{}, err
	}

	// Check if namespace should be ignored
	if r.Config.IsNamespaceIgnored(secret.Namespace) {
		log.V(1).Info("skipping Secret in ignored namespace")
		return ctrl.Result{}, nil
	}

	// Get all workloads in the same namespace
	workloads, err := r.listWorkloads(ctx, secret.Namespace)
	if err != nil {
		log.Error(err, "failed to list workloads")
		return ctrl.Result{}, err
	}

	// Evaluate which workloads should be reloaded
	change := reload.SecretChange{
		Secret:    &secret,
		EventType: reload.EventTypeUpdate,
	}
	decisions := r.ReloadService.ProcessSecret(change, workloads)

	// Apply reloads
	for _, decision := range decisions {
		if !decision.ShouldReload {
			continue
		}

		log.Info("reloading workload",
			"workload", decision.Workload.GetName(),
			"kind", decision.Workload.Kind(),
			"reason", decision.Reason,
		)

		updated, err := r.ReloadService.ApplyReload(
			ctx,
			decision.Workload,
			secret.Name,
			reload.ResourceTypeSecret,
			secret.Namespace,
			decision.Hash,
			decision.AutoReload,
		)
		if err != nil {
			log.Error(err, "failed to apply reload",
				"workload", decision.Workload.GetName(),
				"kind", decision.Workload.Kind(),
			)
			r.recordMetrics(false, secret.Namespace)
			continue
		}

		if updated {
			// Persist the changes
			if err := r.Update(ctx, decision.Workload.GetObject()); err != nil {
				log.Error(err, "failed to update workload",
					"workload", decision.Workload.GetName(),
					"kind", decision.Workload.Kind(),
				)
				r.recordMetrics(false, secret.Namespace)
				continue
			}
			r.recordMetrics(true, secret.Namespace)
			log.Info("workload reloaded successfully",
				"workload", decision.Workload.GetName(),
				"kind", decision.Workload.Kind(),
			)
		}
	}

	return ctrl.Result{}, nil
}

// handleDelete handles Secret deletion events.
func (r *SecretReconciler) handleDelete(ctx context.Context, req ctrl.Request, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling Secret deletion")

	// Get all workloads in the namespace
	workloads, err := r.listWorkloads(ctx, req.Namespace)
	if err != nil {
		log.Error(err, "failed to list workloads")
		return ctrl.Result{}, err
	}

	// For delete events, we create a change with empty Secret
	// The service will use an empty hash
	change := reload.SecretChange{
		Secret:    &corev1.Secret{},
		EventType: reload.EventTypeDelete,
	}
	change.Secret.Name = req.Name
	change.Secret.Namespace = req.Namespace

	decisions := r.ReloadService.ProcessSecret(change, workloads)

	// Apply reloads for delete
	for _, decision := range decisions {
		if !decision.ShouldReload {
			continue
		}

		log.Info("reloading workload due to Secret deletion",
			"workload", decision.Workload.GetName(),
			"kind", decision.Workload.Kind(),
		)

		updated, err := r.ReloadService.ApplyReload(
			ctx,
			decision.Workload,
			req.Name,
			reload.ResourceTypeSecret,
			req.Namespace,
			decision.Hash,
			decision.AutoReload,
		)
		if err != nil {
			log.Error(err, "failed to apply reload for deletion")
			r.recordMetrics(false, req.Namespace)
			continue
		}

		if updated {
			if err := r.Update(ctx, decision.Workload.GetObject()); err != nil {
				log.Error(err, "failed to update workload")
				r.recordMetrics(false, req.Namespace)
				continue
			}
			r.recordMetrics(true, req.Namespace)
		}
	}

	return ctrl.Result{}, nil
}

// listWorkloads returns all workloads in the given namespace.
func (r *SecretReconciler) listWorkloads(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
	var result []workload.WorkloadAccessor

	for _, kind := range r.Registry.SupportedKinds() {
		// Skip ignored workload types
		if r.Config.IsWorkloadIgnored(string(kind)) {
			continue
		}

		workloads, err := r.listWorkloadsByKind(ctx, namespace, kind)
		if err != nil {
			return nil, err
		}
		result = append(result, workloads...)
	}

	return result, nil
}

// listWorkloadsByKind lists workloads of a specific kind in the namespace.
func (r *SecretReconciler) listWorkloadsByKind(ctx context.Context, namespace string, kind workload.Kind) ([]workload.WorkloadAccessor, error) {
	switch kind {
	case workload.KindDeployment:
		return r.listDeployments(ctx, namespace)
	case workload.KindDaemonSet:
		return r.listDaemonSets(ctx, namespace)
	case workload.KindStatefulSet:
		return r.listStatefulSets(ctx, namespace)
	case workload.KindJob:
		return r.listJobs(ctx, namespace)
	case workload.KindCronJob:
		return r.listCronJobs(ctx, namespace)
	default:
		return nil, nil
	}
}

func (r *SecretReconciler) listDeployments(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
	var list appsv1.DeploymentList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]workload.WorkloadAccessor, len(list.Items))
	for i := range list.Items {
		result[i] = workload.NewDeploymentWorkload(&list.Items[i])
	}
	return result, nil
}

func (r *SecretReconciler) listDaemonSets(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
	var list appsv1.DaemonSetList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]workload.WorkloadAccessor, len(list.Items))
	for i := range list.Items {
		result[i] = workload.NewDaemonSetWorkload(&list.Items[i])
	}
	return result, nil
}

func (r *SecretReconciler) listStatefulSets(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
	var list appsv1.StatefulSetList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]workload.WorkloadAccessor, len(list.Items))
	for i := range list.Items {
		result[i] = workload.NewStatefulSetWorkload(&list.Items[i])
	}
	return result, nil
}

func (r *SecretReconciler) listJobs(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
	var list batchv1.JobList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]workload.WorkloadAccessor, len(list.Items))
	for i := range list.Items {
		result[i] = workload.NewJobWorkload(&list.Items[i])
	}
	return result, nil
}

func (r *SecretReconciler) listCronJobs(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
	var list batchv1.CronJobList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]workload.WorkloadAccessor, len(list.Items))
	for i := range list.Items {
		result[i] = workload.NewCronJobWorkload(&list.Items[i])
	}
	return result, nil
}

// recordMetrics records reload metrics.
func (r *SecretReconciler) recordMetrics(success bool, namespace string) {
	if r.Collectors == nil {
		return
	}
	// TODO: Integrate with existing metrics collectors
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	hasher := r.ReloadService.Hasher()

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(predicate.And(
			reload.SecretPredicates(r.Config, hasher),
			reload.NamespaceFilterPredicate(r.Config),
			reload.LabelSelectorPredicate(r.Config),
			reload.IgnoreAnnotationPredicate(r.Config),
			r.createEventFilter(),
		)).
		Complete(r)
}

// createEventFilter filters create events based on initialization state.
func (r *SecretReconciler) createEventFilter() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// During startup, skip create events unless SyncAfterRestart is enabled
			if !r.initialized && !r.Config.SyncAfterRestart {
				return false
			}
			// After initialization, only process creates if ReloadOnCreate is enabled
			return r.Config.ReloadOnCreate
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.Config.ReloadOnDelete
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// Ensure SecretReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &SecretReconciler{}
