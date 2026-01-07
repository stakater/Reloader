package controller

import (
	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SecretReconciler watches Secrets and triggers workload reloads.
type SecretReconciler = ResourceReconciler[*corev1.Secret]

// NewSecretReconciler creates a new SecretReconciler with the given dependencies.
func NewSecretReconciler(
	c client.Client,
	log logr.Logger,
	cfg *config.Config,
	reloadService *reload.Service,
	registry *workload.Registry,
	collectors *metrics.Collectors,
	eventRecorder *events.Recorder,
	webhookClient *webhook.Client,
	alerter alerting.Alerter,
	pauseHandler *reload.PauseHandler,
	nsCache *NamespaceCache,
) *SecretReconciler {
	return NewResourceReconciler(
		ResourceReconcilerDeps{
			Client:         c,
			Log:            log,
			Config:         cfg,
			ReloadService:  reloadService,
			Registry:       registry,
			Collectors:     collectors,
			EventRecorder:  eventRecorder,
			WebhookClient:  webhookClient,
			Alerter:        alerter,
			PauseHandler:   pauseHandler,
			NamespaceCache: nsCache,
		},
		ResourceConfig[*corev1.Secret]{
			ResourceType: reload.ResourceTypeSecret,
			NewResource:  func() *corev1.Secret { return &corev1.Secret{} },
			CreateChange: func(s *corev1.Secret, eventType reload.EventType) reload.ResourceChange {
				return reload.SecretChange{Secret: s, EventType: eventType}
			},
			CreatePredicates: func(cfg *config.Config, hasher *reload.Hasher) predicate.Predicate {
				return reload.SecretPredicates(cfg, hasher)
			},
		},
	)
}

// SetupSecretReconciler sets up a Secret reconciler with the manager.
func SetupSecretReconciler(mgr ctrl.Manager, r *SecretReconciler) error {
	return r.SetupWithManager(mgr, &corev1.Secret{})
}

var _ reconcile.Reconciler = &SecretReconciler{}
