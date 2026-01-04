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
	PauseHandler  *reload.PauseHandler

	handler     *ReloadHandler
	initialized bool
	initOnce    sync.Once
}

// Reconcile handles ConfigMap events and triggers workload reloads as needed.
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	log := r.Log.WithValues("configmap", req.NamespacedName)

	r.initOnce.Do(func() {
		r.initialized = true
		log.Info("ConfigMap controller initialized")
	})

	r.Collectors.RecordEventReceived("reconcile", "configmap")

	var cm corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
		if errors.IsNotFound(err) {
			if r.Config.ReloadOnDelete {
				r.Collectors.RecordEventReceived("delete", "configmap")
				result, err := r.handleDelete(ctx, req, log)
				if err != nil {
					r.Collectors.RecordReconcile("error", time.Since(startTime))
				} else {
					r.Collectors.RecordReconcile("success", time.Since(startTime))
				}
				return result, err
			}
			r.Collectors.RecordSkipped("not_found")
			r.Collectors.RecordReconcile("success", time.Since(startTime))
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ConfigMap")
		r.Collectors.RecordError("get_configmap")
		r.Collectors.RecordReconcile("error", time.Since(startTime))
		return ctrl.Result{}, err
	}

	if r.Config.IsNamespaceIgnored(cm.Namespace) {
		log.V(1).Info("skipping ConfigMap in ignored namespace")
		r.Collectors.RecordSkipped("ignored_namespace")
		r.Collectors.RecordReconcile("success", time.Since(startTime))
		return ctrl.Result{}, nil
	}

	result, err := r.reloadHandler().Process(ctx, cm.Namespace, cm.Name, reload.ResourceTypeConfigMap,
		func(workloads []workload.WorkloadAccessor) []reload.ReloadDecision {
			return r.ReloadService.Process(reload.ConfigMapChange{
				ConfigMap: &cm,
				EventType: reload.EventTypeUpdate,
			}, workloads)
		}, log)

	if err != nil {
		r.Collectors.RecordReconcile("error", time.Since(startTime))
	} else {
		r.Collectors.RecordReconcile("success", time.Since(startTime))
	}
	return result, err
}

func (r *ConfigMapReconciler) handleDelete(ctx context.Context, req ctrl.Request, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling ConfigMap deletion")

	cm := &corev1.ConfigMap{}
	cm.Name = req.Name
	cm.Namespace = req.Namespace

	return r.reloadHandler().Process(ctx, req.Namespace, req.Name, reload.ResourceTypeConfigMap,
		func(workloads []workload.WorkloadAccessor) []reload.ReloadDecision {
			return r.ReloadService.Process(reload.ConfigMapChange{
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
			PauseHandler:  r.PauseHandler,
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
