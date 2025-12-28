package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/metadata"
	"github.com/stakater/Reloader/internal/pkg/metrics"
)

// Environment variable names for pod identity in HA mode.
const (
	podNameEnv      = "POD_NAME"
	podNamespaceEnv = "POD_NAMESPACE"
)

// cfg holds the configuration for this reloader instance.
var cfg *config.Config

func main() {
	if err := newReloaderCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newReloaderCommand() *cobra.Command {
	cfg = config.NewDefault()

	cmd := &cobra.Command{
		Use:   "reloader",
		Short: "A watcher for your Kubernetes cluster",
		RunE:  run,
	}

	config.BindFlags(cmd.PersistentFlags(), cfg)
	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	if err := config.ApplyFlags(cfg); err != nil {
		return fmt.Errorf("applying flags: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	if cfg.EnableHA {
		if err := validateHAEnvs(); err != nil {
			return err
		}
		cfg.LeaderElection.Identity = os.Getenv(podNameEnv)
		if cfg.LeaderElection.Namespace == "" {
			cfg.LeaderElection.Namespace = os.Getenv(podNamespaceEnv)
		}
	}

	log, err := configureLogging(cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("configuring logging: %w", err)
	}

	controllerruntime.SetLogger(log)

	log.Info("Starting Reloader")

	if ns := os.Getenv("KUBERNETES_NAMESPACE"); ns == "" {
		log.Info("KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces")
	}

	if len(cfg.NamespaceSelectors) > 0 {
		log.Info("namespace-selector is set", "selectors", cfg.NamespaceSelectorStrings)
	}

	if len(cfg.ResourceSelectors) > 0 {
		log.Info("resource-label-selector is set", "selectors", cfg.ResourceSelectorStrings)
	}

	if cfg.WebhookURL != "" {
		log.Info("webhook-url is set, will only send webhook, no resources will be reloaded", "url", cfg.WebhookURL)
	}

	if cfg.EnableHA {
		log.Info(
			"high-availability mode enabled",
			"leaderElectionID", cfg.LeaderElection.LockName,
			"leaderElectionNamespace", cfg.LeaderElection.Namespace,
		)
	}

	collectors := metrics.SetupPrometheusEndpoint()

	mgr, err := controller.NewManager(
		controller.ManagerOptions{
			Config:     cfg,
			Log:        log,
			Collectors: &collectors,
		},
	)
	if err != nil {
		return fmt.Errorf("creating manager: %w", err)
	}

	if err := controller.SetupReconcilers(mgr, cfg, log, &collectors); err != nil {
		return fmt.Errorf("setting up reconcilers: %w", err)
	}

	if err := metadata.CreateOrUpdate(mgr.GetClient(), cfg, log); err != nil {
		log.Error(err, "Failed to create metadata ConfigMap")
		// Non-fatal, continue starting
	}

	if cfg.EnablePProf {
		go startPProfServer(log)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	log.Info("Starting controller manager")
	if err := controller.RunManager(ctx, mgr, log); err != nil {
		return fmt.Errorf("manager exited with error: %w", err)
	}

	log.Info("Reloader shutdown complete")
	return nil
}

func configureLogging(logFormat, logLevel string) (logr.Logger, error) {
	// Parse log level
	var level zerolog.Level
	switch logLevel {
	case "trace":
		level = zerolog.TraceLevel
	case "debug":
		level = zerolog.DebugLevel
	case "info", "":
		level = zerolog.InfoLevel
	case "warn", "warning":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	default:
		return logr.Logger{}, fmt.Errorf("unsupported log level: %q", logLevel)
	}

	var zl zerolog.Logger
	switch logFormat {
	case "json":
		zl = zerolog.New(os.Stdout).Level(level).With().Timestamp().Logger()
	case "":
		// Human-readable console output
		zl = zerolog.New(
			zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
			},
		).Level(level).With().Timestamp().Logger()
	default:
		return logr.Logger{}, fmt.Errorf("unsupported log format: %q", logFormat)
	}

	return zerologr.New(&zl), nil
}

func validateHAEnvs() error {
	podName := os.Getenv(podNameEnv)
	podNamespace := os.Getenv(podNamespaceEnv)

	if podName == "" {
		return fmt.Errorf("%s not set, cannot run in HA mode", podNameEnv)
	}
	if podNamespace == "" {
		return fmt.Errorf("%s not set, cannot run in HA mode", podNamespaceEnv)
	}
	return nil
}

func startPProfServer(log logr.Logger) {
	log.Info("Starting pprof server", "addr", cfg.PProfAddr)
	if err := http.ListenAndServe(cfg.PProfAddr, nil); err != nil {
		log.Error(err, "Failed to start pprof server")
	}
}
