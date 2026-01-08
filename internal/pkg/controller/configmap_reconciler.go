package controller

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
)

// ConfigMapReconciler watches ConfigMaps and triggers workload reloads.
type ConfigMapReconciler = ResourceReconciler[*corev1.ConfigMap]

// NewConfigMapReconciler creates a new ConfigMapReconciler with the given dependencies.
func NewConfigMapReconciler(
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
) *ConfigMapReconciler {
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
		ResourceConfig[*corev1.ConfigMap]{
			ResourceType: reload.ResourceTypeConfigMap,
			NewResource:  func() *corev1.ConfigMap { return &corev1.ConfigMap{} },
			CreateChange: func(cm *corev1.ConfigMap, eventType reload.EventType) reload.ResourceChange {
				return reload.ConfigMapChange{ConfigMap: cm, EventType: eventType}
			},
			CreatePredicates: func(cfg *config.Config, hasher *reload.Hasher) predicate.Predicate {
				return reload.ConfigMapPredicates(cfg, hasher)
			},
		},
	)
}

// SetupConfigMapReconciler sets up a ConfigMap reconciler with the manager.
func SetupConfigMapReconciler(mgr ctrl.Manager, r *ConfigMapReconciler) error {
	return r.SetupWithManager(mgr, &corev1.ConfigMap{})
}

var _ reconcile.Reconciler = &ConfigMapReconciler{}
