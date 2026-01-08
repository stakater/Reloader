package controller

import (
	"context"
	"fmt"

	argorolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/go-logr/logr"
	openshiftv1 "github.com/openshift/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
)

var runtimeScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(runtimeScheme))
}

// AddOptionalSchemes adds optional workload type schemes if enabled.
func AddOptionalSchemes(argoRolloutsEnabled, deploymentConfigEnabled bool) {
	if argoRolloutsEnabled {
		utilruntime.Must(argorolloutsv1alpha1.AddToScheme(runtimeScheme))
	}
	if deploymentConfigEnabled {
		utilruntime.Must(openshiftv1.AddToScheme(runtimeScheme))
	}
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

	if cfg.WatchedNamespace != "" {
		mgrOpts.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				cfg.WatchedNamespace: {},
			},
		}
		opts.Log.Info("namespace filtering enabled", "namespace", cfg.WatchedNamespace)
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

	if cfg.WatchedNamespace != "" {
		mgrOpts.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				cfg.WatchedNamespace: {},
			},
		}
	}

	mgr, err := ctrl.NewManager(restConfig, mgrOpts)
	if err != nil {
		return nil, fmt.Errorf("creating manager: %w", err)
	}

	return mgr, nil
}

// SetupReconcilers sets up all reconcilers with the manager.
func SetupReconcilers(mgr ctrl.Manager, cfg *config.Config, log logr.Logger, collectors *metrics.Collectors) error {
	registry := workload.NewRegistry(
		workload.RegistryOptions{
			ArgoRolloutsEnabled:       cfg.ArgoRolloutsEnabled,
			DeploymentConfigEnabled:   cfg.DeploymentConfigEnabled,
			RolloutStrategyAnnotation: cfg.Annotations.RolloutStrategy,
		},
	)
	reloadService := reload.NewService(cfg, log.WithName("reload"))
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

	// Create namespace cache if namespace selectors are configured.
	// This cache is shared between the namespace reconciler and resource reconcilers.
	var nsCache *NamespaceCache
	if len(cfg.NamespaceSelectors) > 0 {
		nsCache = NewNamespaceCache(true)
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

	// Setup ConfigMap reconciler
	if !cfg.IsResourceIgnored("configmaps") {
		cmReconciler := NewConfigMapReconciler(
			mgr.GetClient(),
			log.WithName("configmap-reconciler"),
			cfg,
			reloadService,
			registry,
			collectors,
			eventRecorder,
			webhookClient,
			alerter,
			pauseHandler,
			nsCache,
		)
		if err := SetupConfigMapReconciler(mgr, cmReconciler); err != nil {
			return fmt.Errorf("setting up configmap reconciler: %w", err)
		}
	}

	// Setup Secret reconciler
	if !cfg.IsResourceIgnored("secrets") {
		secretReconciler := NewSecretReconciler(
			mgr.GetClient(),
			log.WithName("secret-reconciler"),
			cfg,
			reloadService,
			registry,
			collectors,
			eventRecorder,
			webhookClient,
			alerter,
			pauseHandler,
			nsCache,
		)
		if err := SetupSecretReconciler(mgr, secretReconciler); err != nil {
			return fmt.Errorf("setting up secret reconciler: %w", err)
		}
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
