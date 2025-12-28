package controller

import (
	"context"
	"fmt"
	"time"

	argorolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var runtimeScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(runtimeScheme))
	utilruntime.Must(argorolloutsv1alpha1.AddToScheme(runtimeScheme))
}

// ManagerOptions contains options for creating a new Manager.
type ManagerOptions struct {
	Config     *config.Config
	Log        logr.Logger
	Collectors *metrics.Collectors
}

// NewManager creates a new controller-runtime manager with the given options.
func NewManager(opts ManagerOptions) (ctrl.Manager, error) {
	cfg := opts.Config

	leaseDuration := 15 * time.Second
	renewDeadline := 10 * time.Second
	retryPeriod := 2 * time.Second

	mgrOpts := ctrl.Options{
		Scheme: runtimeScheme,
		Metrics: ctrlmetrics.Options{
			BindAddress: cfg.MetricsAddr,
		},
		HealthProbeBindAddress: cfg.HealthAddr,
		LeaderElection:         cfg.EnableHA,
		LeaderElectionID:       "reloader-leader-election",
		LeaseDuration:          &leaseDuration,
		RenewDeadline:          &renewDeadline,
		RetryPeriod:            &retryPeriod,
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		return nil, fmt.Errorf("creating manager: %w", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("setting up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("setting up ready check: %w", err)
	}

	return mgr, nil
}

// SetupReconcilers sets up all reconcilers with the manager.
func SetupReconcilers(mgr ctrl.Manager, cfg *config.Config, log logr.Logger, collectors *metrics.Collectors) error {
	registry := workload.NewRegistry(cfg.ArgoRolloutsEnabled)
	reloadService := reload.NewService(cfg)
	eventRecorder := events.NewRecorder(mgr.GetEventRecorderFor("reloader"))

	// Create webhook client if URL is configured
	var webhookClient *webhook.Client
	if cfg.WebhookURL != "" {
		webhookClient = webhook.NewClient(cfg.WebhookURL, log.WithName("webhook"))
		log.Info("webhook mode enabled", "url", cfg.WebhookURL)
	}

	if !cfg.IsResourceIgnored("configmaps") {
		if err := (&ConfigMapReconciler{
			Client:        mgr.GetClient(),
			Log:           log.WithName("configmap-reconciler"),
			Config:        cfg,
			ReloadService: reloadService,
			Registry:      registry,
			Collectors:    collectors,
			EventRecorder: eventRecorder,
			WebhookClient: webhookClient,
		}).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("setting up configmap reconciler: %w", err)
		}
	}

	if !cfg.IsResourceIgnored("secrets") {
		if err := (&SecretReconciler{
			Client:        mgr.GetClient(),
			Log:           log.WithName("secret-reconciler"),
			Config:        cfg,
			ReloadService: reloadService,
			Registry:      registry,
			Collectors:    collectors,
			EventRecorder: eventRecorder,
			WebhookClient: webhookClient,
		}).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("setting up secret reconciler: %w", err)
		}
	}

	return nil
}

// RunManager starts the manager and blocks until it stops.
func RunManager(ctx context.Context, mgr ctrl.Manager, log logr.Logger) error {
	log.Info("starting manager")
	return mgr.Start(ctx)
}
