package controller

import (
	"context"
	"fmt"

	argorolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/go-logr/logr"
	openshiftv1 "github.com/openshift/api/apps/v1"
	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var runtimeScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(runtimeScheme))
	utilruntime.Must(argorolloutsv1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(openshiftv1.AddToScheme(runtimeScheme))
}

// ManagerOptions contains options for creating a new Manager.
type ManagerOptions struct {
	Config     *config.Config
	Log        logr.Logger
	Collectors *metrics.Collectors
}

// NewManager creates a new controller-runtime manager with the given options.
// This follows controller-runtime and operator-sdk conventions for leader election.
func NewManager(opts ManagerOptions) (ctrl.Manager, error) {
	cfg := opts.Config
	le := cfg.LeaderElection

	mgrOpts := ctrl.Options{
		Scheme: runtimeScheme,
		Metrics: ctrlmetrics.Options{
			BindAddress: cfg.MetricsAddr,
		},
		HealthProbeBindAddress: cfg.HealthAddr,

		// Leader election configuration following operator-sdk best practices:
		// - LeaderElection enables/disables leader election
		// - LeaderElectionID is the name of the lease resource
		// - LeaderElectionNamespace where the lease is created (defaults to pod namespace)
		// - LeaderElectionReleaseOnCancel allows faster failover by releasing the lock on shutdown
		LeaderElection:                cfg.EnableHA,
		LeaderElectionID:              le.LockName,
		LeaderElectionNamespace:       le.Namespace,
		LeaderElectionReleaseOnCancel: le.ReleaseOnCancel,
		LeaseDuration:                 &le.LeaseDuration,
		RenewDeadline:                 &le.RenewDeadline,
		RetryPeriod:                   &le.RetryPeriod,
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		return nil, fmt.Errorf("creating manager: %w", err)
	}

	// Add health and readiness probes.
	// The healthz probe reports whether the manager is running.
	// The readyz probe reports whether the manager is ready to serve requests.
	// When leader election is enabled, readyz will fail until this instance becomes leader.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("setting up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("setting up ready check: %w", err)
	}

	return mgr, nil
}

// NewManagerWithRestConfig creates a new controller-runtime manager with the given rest.Config.
// This is useful for testing where you have a pre-existing cluster configuration.
func NewManagerWithRestConfig(opts ManagerOptions, restConfig *rest.Config) (ctrl.Manager, error) {
	cfg := opts.Config
	le := cfg.LeaderElection

	mgrOpts := ctrl.Options{
		Scheme: runtimeScheme,
		Metrics: ctrlmetrics.Options{
			BindAddress: "0", // Disable metrics server in tests
		},
		HealthProbeBindAddress: "0", // Disable health probes in tests

		// Leader election configuration
		LeaderElection:                cfg.EnableHA,
		LeaderElectionID:              le.LockName,
		LeaderElectionNamespace:       le.Namespace,
		LeaderElectionReleaseOnCancel: le.ReleaseOnCancel,
		LeaseDuration:                 &le.LeaseDuration,
		RenewDeadline:                 &le.RenewDeadline,
		RetryPeriod:                   &le.RetryPeriod,
	}

	mgr, err := ctrl.NewManager(restConfig, mgrOpts)
	if err != nil {
		return nil, fmt.Errorf("creating manager: %w", err)
	}

	return mgr, nil
}

// SetupReconcilers sets up all reconcilers with the manager.
func SetupReconcilers(mgr ctrl.Manager, cfg *config.Config, log logr.Logger, collectors *metrics.Collectors) error {
	registry := workload.NewRegistry(workload.RegistryOptions{
		ArgoRolloutsEnabled:     cfg.ArgoRolloutsEnabled,
		DeploymentConfigEnabled: cfg.DeploymentConfigEnabled,
	})
	reloadService := reload.NewService(cfg)
	eventRecorder := events.NewRecorder(mgr.GetEventRecorderFor("reloader"))
	pauseHandler := reload.NewPauseHandler(cfg)

	// Create alerter based on configuration
	alerter := alerting.NewAlerter(cfg)
	if cfg.Alerting.Enabled {
		log.Info("alerting enabled", "sink", cfg.Alerting.Sink)
	}

	// Create webhook client if URL is configured
	var webhookClient *webhook.Client
	if cfg.WebhookURL != "" {
		webhookClient = webhook.NewClient(cfg.WebhookURL, log.WithName("webhook"))
		log.Info("webhook mode enabled", "url", cfg.WebhookURL)
	}

	// Setup ConfigMap reconciler
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
			Alerter:       alerter,
			PauseHandler:  pauseHandler,
		}).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("setting up configmap reconciler: %w", err)
		}
	}

	// Setup Secret reconciler
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
			Alerter:       alerter,
			PauseHandler:  pauseHandler,
		}).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("setting up secret reconciler: %w", err)
		}
	}

	// Setup Namespace reconciler if namespace selectors are configured
	if len(cfg.NamespaceSelectors) > 0 {
		nsCache := NewNamespaceCache(true)
		if err := (&NamespaceReconciler{
			Client: mgr.GetClient(),
			Log:    log.WithName("namespace-reconciler"),
			Config: cfg,
			Cache:  nsCache,
		}).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("setting up namespace reconciler: %w", err)
		}
		log.Info("namespace reconciler enabled for label selector filtering")
	}

	// Setup Deployment reconciler for pause handling
	if err := (&DeploymentReconciler{
		Client:       mgr.GetClient(),
		Log:          log.WithName("deployment-reconciler"),
		Config:       cfg,
		PauseHandler: pauseHandler,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setting up deployment reconciler: %w", err)
	}

	return nil
}

// RunManager starts the manager and blocks until it stops.
func RunManager(ctx context.Context, mgr ctrl.Manager, log logr.Logger) error {
	log.Info("starting manager")
	return mgr.Start(ctx)
}
