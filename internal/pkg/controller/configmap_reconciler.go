package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
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

// ConfigMapReconciler watches ConfigMaps and triggers workload reloads.
type ConfigMapReconciler struct {
	client.Client
	Log           logr.Logger
	Config        *config.Config
	ReloadService *reload.Service
	Registry      *workload.Registry
	Collectors    *metrics.Collectors
	EventRecorder *events.Recorder
	WebhookClient *webhook.Client
	Alerter       alerting.Alerter

	initialized bool
	initOnce    sync.Once
}

// Reconcile handles ConfigMap events and triggers workload reloads as needed.
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("configmap", req.NamespacedName)

	// Mark as initialized after first reconcile (caches are synced at this point)
	r.initOnce.Do(func() {
		r.initialized = true
		log.Info("ConfigMap controller initialized")
	})

	// Fetch the ConfigMap
	var cm corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap was deleted - handle if ReloadOnDelete is enabled
			if r.Config.ReloadOnDelete {
				return r.handleDelete(ctx, req, log)
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	// Check if namespace should be ignored
	if r.Config.IsNamespaceIgnored(cm.Namespace) {
		log.V(1).Info("skipping ConfigMap in ignored namespace")
		return ctrl.Result{}, nil
	}

	// Get all workloads in the same namespace
	workloads, err := r.listWorkloads(ctx, cm.Namespace)
	if err != nil {
		log.Error(err, "failed to list workloads")
		return ctrl.Result{}, err
	}

	// Evaluate which workloads should be reloaded
	change := reload.ConfigMapChange{
		ConfigMap: &cm,
		EventType: reload.EventTypeUpdate,
	}
	decisions := r.ReloadService.ProcessConfigMap(change, workloads)

	// Collect workloads that should be reloaded
	var workloadsToReload []reload.ReloadDecision
	for _, decision := range decisions {
		if decision.ShouldReload {
			workloadsToReload = append(workloadsToReload, decision)
		}
	}

	// If webhook is configured, send notification instead of modifying workloads
	if r.WebhookClient.IsConfigured() && len(workloadsToReload) > 0 {
		return r.sendWebhookNotification(ctx, cm.Name, cm.Namespace, reload.ResourceTypeConfigMap, workloadsToReload, log)
	}

	// Apply reloads with conflict retry
	for _, decision := range workloadsToReload {
		log.Info("reloading workload",
			"workload", decision.Workload.GetName(),
			"kind", decision.Workload.Kind(),
			"reason", decision.Reason,
		)

		updated, err := UpdateWorkloadWithRetry(
			ctx,
			r.Client,
			r.ReloadService,
			decision.Workload,
			cm.Name,
			reload.ResourceTypeConfigMap,
			cm.Namespace,
			decision.Hash,
			decision.AutoReload,
		)
		if err != nil {
			log.Error(err, "failed to update workload",
				"workload", decision.Workload.GetName(),
				"kind", decision.Workload.Kind(),
			)
			r.EventRecorder.ReloadFailed(decision.Workload.GetObject(), "ConfigMap", cm.Name, err)
			r.recordMetrics(false, cm.Namespace)
			continue
		}

		if updated {
			r.EventRecorder.ReloadSuccess(decision.Workload.GetObject(), "ConfigMap", cm.Name)
			r.recordMetrics(true, cm.Namespace)
			log.Info("workload reloaded successfully",
				"workload", decision.Workload.GetName(),
				"kind", decision.Workload.Kind(),
			)

			// Send alert notification
			if err := r.Alerter.Send(ctx, alerting.AlertMessage{
				WorkloadKind:      string(decision.Workload.Kind()),
				WorkloadName:      decision.Workload.GetName(),
				WorkloadNamespace: decision.Workload.GetNamespace(),
				ResourceKind:      "ConfigMap",
				ResourceName:      cm.Name,
				ResourceNamespace: cm.Namespace,
				Timestamp:         time.Now(),
			}); err != nil {
				log.Error(err, "failed to send alert")
			}
		}
	}

	return ctrl.Result{}, nil
}

// FieldManager is the field manager name used for server-side apply.
const FieldManager = "reloader"

// handleDelete handles ConfigMap deletion events.
func (r *ConfigMapReconciler) handleDelete(ctx context.Context, req ctrl.Request, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling ConfigMap deletion")

	// Get all workloads in the namespace
	workloads, err := r.listWorkloads(ctx, req.Namespace)
	if err != nil {
		log.Error(err, "failed to list workloads")
		return ctrl.Result{}, err
	}

	// For delete events, we create a change with empty ConfigMap
	change := reload.ConfigMapChange{
		ConfigMap: &corev1.ConfigMap{},
		EventType: reload.EventTypeDelete,
	}
	change.ConfigMap.Name = req.Name
	change.ConfigMap.Namespace = req.Namespace

	decisions := r.ReloadService.ProcessConfigMap(change, workloads)

	// Collect workloads that should be reloaded
	var workloadsToReload []reload.ReloadDecision
	for _, decision := range decisions {
		if decision.ShouldReload {
			workloadsToReload = append(workloadsToReload, decision)
		}
	}

	// If webhook is configured, send notification instead of modifying workloads
	if r.WebhookClient.IsConfigured() && len(workloadsToReload) > 0 {
		return r.sendWebhookNotification(ctx, req.Name, req.Namespace, reload.ResourceTypeConfigMap, workloadsToReload, log)
	}

	// Apply reloads for delete with conflict retry
	for _, decision := range workloadsToReload {
		log.Info("reloading workload due to ConfigMap deletion",
			"workload", decision.Workload.GetName(),
			"kind", decision.Workload.Kind(),
		)

		updated, err := UpdateWorkloadWithRetry(
			ctx,
			r.Client,
			r.ReloadService,
			decision.Workload,
			req.Name,
			reload.ResourceTypeConfigMap,
			req.Namespace,
			decision.Hash,
			decision.AutoReload,
		)
		if err != nil {
			log.Error(err, "failed to update workload")
			r.EventRecorder.ReloadFailed(decision.Workload.GetObject(), "ConfigMap", req.Name, err)
			r.recordMetrics(false, req.Namespace)
			continue
		}

		if updated {
			r.EventRecorder.ReloadSuccess(decision.Workload.GetObject(), "ConfigMap", req.Name)
			r.recordMetrics(true, req.Namespace)

			// Send alert notification
			if err := r.Alerter.Send(ctx, alerting.AlertMessage{
				WorkloadKind:      string(decision.Workload.Kind()),
				WorkloadName:      decision.Workload.GetName(),
				WorkloadNamespace: decision.Workload.GetNamespace(),
				ResourceKind:      "ConfigMap",
				ResourceName:      req.Name,
				ResourceNamespace: req.Namespace,
				Timestamp:         time.Now(),
			}); err != nil {
				log.Error(err, "failed to send alert")
			}
		}
	}

	return ctrl.Result{}, nil
}

// listWorkloads returns all workloads in the given namespace.
func (r *ConfigMapReconciler) listWorkloads(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
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
func (r *ConfigMapReconciler) listWorkloadsByKind(ctx context.Context, namespace string, kind workload.Kind) ([]workload.WorkloadAccessor, error) {
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

func (r *ConfigMapReconciler) listDeployments(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
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

func (r *ConfigMapReconciler) listDaemonSets(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
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

func (r *ConfigMapReconciler) listStatefulSets(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
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

func (r *ConfigMapReconciler) listJobs(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
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

func (r *ConfigMapReconciler) listCronJobs(ctx context.Context, namespace string) ([]workload.WorkloadAccessor, error) {
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
func (r *ConfigMapReconciler) recordMetrics(success bool, namespace string) {
	r.Collectors.RecordReload(success, namespace)
}

// sendWebhookNotification sends a webhook notification instead of modifying workloads.
func (r *ConfigMapReconciler) sendWebhookNotification(
	ctx context.Context,
	resourceName, namespace string,
	resourceType reload.ResourceType,
	decisions []reload.ReloadDecision,
	log logr.Logger,
) (ctrl.Result, error) {
	var workloads []webhook.WorkloadInfo
	var hash string
	for _, d := range decisions {
		workloads = append(workloads, webhook.WorkloadInfo{
			Kind:      string(d.Workload.Kind()),
			Name:      d.Workload.GetName(),
			Namespace: d.Workload.GetNamespace(),
		})
		if hash == "" {
			hash = d.Hash
		}
	}

	payload := webhook.Payload{
		Kind:         string(resourceType),
		Namespace:    namespace,
		ResourceName: resourceName,
		ResourceType: string(resourceType),
		Hash:         hash,
		Timestamp:    time.Now().UTC(),
		Workloads:    workloads,
	}

	if err := r.WebhookClient.Send(ctx, payload); err != nil {
		log.Error(err, "failed to send webhook notification")
		r.recordMetrics(false, namespace)
		return ctrl.Result{}, err
	}

	log.Info("webhook notification sent",
		"resource", resourceName,
		"workloadCount", len(workloads),
	)
	r.recordMetrics(true, namespace)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	hasher := r.ReloadService.Hasher()

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.And(
			reload.ConfigMapPredicates(r.Config, hasher),
			reload.NamespaceFilterPredicate(r.Config),
			reload.LabelSelectorPredicate(r.Config),
			reload.IgnoreAnnotationPredicate(r.Config),
			r.createEventFilter(),
		)).
		Complete(r)
}

// createEventFilter filters create events based on initialization state.
func (r *ConfigMapReconciler) createEventFilter() predicate.Predicate {
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

// Ensure ConfigMapReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ConfigMapReconciler{}
