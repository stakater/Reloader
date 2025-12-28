package controller

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	handler     *ReloadHandler
	initialized bool
	initOnce    sync.Once
}

// Reconcile handles ConfigMap events and triggers workload reloads as needed.
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("configmap", req.NamespacedName)

	r.initOnce.Do(func() {
		r.initialized = true
		log.Info("ConfigMap controller initialized")
	})

	var cm corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
		if errors.IsNotFound(err) {
			if r.Config.ReloadOnDelete {
				return r.handleDelete(ctx, req, log)
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	if r.Config.IsNamespaceIgnored(cm.Namespace) {
		log.V(1).Info("skipping ConfigMap in ignored namespace")
		return ctrl.Result{}, nil
	}

	return r.reloadHandler().Process(ctx, cm.Namespace, cm.Name, "ConfigMap", reload.ResourceTypeConfigMap,
		func(workloads []workload.WorkloadAccessor) []reload.ReloadDecision {
			return r.ReloadService.ProcessConfigMap(reload.ConfigMapChange{
				ConfigMap: &cm,
				EventType: reload.EventTypeUpdate,
			}, workloads)
		}, log)
}

// FieldManager is the field manager name used for server-side apply.
const FieldManager = "reloader"

func (r *ConfigMapReconciler) handleDelete(ctx context.Context, req ctrl.Request, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling ConfigMap deletion")

	cm := &corev1.ConfigMap{}
	cm.Name = req.Name
	cm.Namespace = req.Namespace

	return r.reloadHandler().Process(ctx, req.Namespace, req.Name, "ConfigMap", reload.ResourceTypeConfigMap,
		func(workloads []workload.WorkloadAccessor) []reload.ReloadDecision {
			return r.ReloadService.ProcessConfigMap(reload.ConfigMapChange{
				ConfigMap: cm,
				EventType: reload.EventTypeDelete,
			}, workloads)
		}, log)
}

func (r *ConfigMapReconciler) reloadHandler() *ReloadHandler {
	if r.handler == nil {
		r.handler = &ReloadHandler{
			Client:        r.Client,
			Lister:        workload.NewLister(r.Client, r.Registry, r.Config),
			ReloadService: r.ReloadService,
			WebhookClient: r.WebhookClient,
			Collectors:    r.Collectors,
			EventRecorder: r.EventRecorder,
			Alerter:       r.Alerter,
		}
	}
	return r.handler
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(BuildEventFilter(
			reload.ConfigMapPredicates(r.Config, r.ReloadService.Hasher()),
			r.Config, &r.initialized,
		)).
		Complete(r)
}

var _ reconcile.Reconciler = &ConfigMapReconciler{}
