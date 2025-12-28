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

// SecretReconciler watches Secrets and triggers workload reloads.
type SecretReconciler struct {
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

// Reconcile handles Secret events and triggers workload reloads as needed.
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", req.NamespacedName)

	r.initOnce.Do(func() {
		r.initialized = true
		log.Info("Secret controller initialized")
	})

	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			if r.Config.ReloadOnDelete {
				return r.handleDelete(ctx, req, log)
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get Secret")
		return ctrl.Result{}, err
	}

	if r.Config.IsNamespaceIgnored(secret.Namespace) {
		log.V(1).Info("skipping Secret in ignored namespace")
		return ctrl.Result{}, nil
	}

	return r.reloadHandler().Process(ctx, secret.Namespace, secret.Name, reload.ResourceTypeSecret,
		func(workloads []workload.WorkloadAccessor) []reload.ReloadDecision {
			return r.ReloadService.Process(reload.SecretChange{
				Secret:    &secret,
				EventType: reload.EventTypeUpdate,
			}, workloads)
		}, log)
}

func (r *SecretReconciler) handleDelete(ctx context.Context, req ctrl.Request, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling Secret deletion")

	secret := &corev1.Secret{}
	secret.Name = req.Name
	secret.Namespace = req.Namespace

	return r.reloadHandler().Process(ctx, req.Namespace, req.Name, reload.ResourceTypeSecret,
		func(workloads []workload.WorkloadAccessor) []reload.ReloadDecision {
			return r.ReloadService.Process(reload.SecretChange{
				Secret:    secret,
				EventType: reload.EventTypeDelete,
			}, workloads)
		}, log)
}

func (r *SecretReconciler) reloadHandler() *ReloadHandler {
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
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(BuildEventFilter(
			reload.SecretPredicates(r.Config, r.ReloadService.Hasher()),
			r.Config, &r.initialized,
		)).
		Complete(r)
}

var _ reconcile.Reconciler = &SecretReconciler{}
