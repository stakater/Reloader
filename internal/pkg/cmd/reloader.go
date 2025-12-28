package cmd

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/leadership"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stakater/Reloader/pkg/kube"
)

// cfg holds the configuration for this reloader instance.
// It is populated by flag parsing and used throughout the application.
var cfg *config.Config

// NewReloaderCommand starts the reloader controller
func NewReloaderCommand() *cobra.Command {
	// Create config with defaults
	cfg = config.NewDefault()

	cmd := &cobra.Command{
		Use:     "reloader",
		Short:   "A watcher for your Kubernetes cluster",
		PreRunE: validateFlags,
		Run:     startReloader,
	}

	// Bind flags to the new config package
	config.BindFlags(cmd.PersistentFlags(), cfg)

	return cmd
}

func validateFlags(*cobra.Command, []string) error {
	// Apply post-parse flag processing (converts string flags to proper types)
	if err := config.ApplyFlags(cfg); err != nil {
		return fmt.Errorf("applying flags: %w", err)
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	// Sync new config to old options package for backward compatibility
	// This bridge allows existing code to keep working during migration
	syncConfigToOptions(cfg)

	// Validate that HA options are correct
	if cfg.EnableHA {
		if err := validateHAEnvs(); err != nil {
			return err
		}
	}

	return nil
}

// syncConfigToOptions bridges the new Config struct to the old options package.
// This allows existing code to continue working during the migration period.
// TODO: Remove this once all code is migrated to use Config directly.
func syncConfigToOptions(cfg *config.Config) {
	options.AutoReloadAll = cfg.AutoReloadAll
	options.ConfigmapUpdateOnChangeAnnotation = cfg.Annotations.ConfigmapReload
	options.SecretUpdateOnChangeAnnotation = cfg.Annotations.SecretReload
	options.ReloaderAutoAnnotation = cfg.Annotations.Auto
	options.ConfigmapReloaderAutoAnnotation = cfg.Annotations.ConfigmapAuto
	options.SecretReloaderAutoAnnotation = cfg.Annotations.SecretAuto
	options.IgnoreResourceAnnotation = cfg.Annotations.Ignore
	options.ConfigmapExcludeReloaderAnnotation = cfg.Annotations.ConfigmapExclude
	options.SecretExcludeReloaderAnnotation = cfg.Annotations.SecretExclude
	options.AutoSearchAnnotation = cfg.Annotations.Search
	options.SearchMatchAnnotation = cfg.Annotations.Match
	options.RolloutStrategyAnnotation = cfg.Annotations.RolloutStrategy
	options.PauseDeploymentAnnotation = cfg.Annotations.PausePeriod
	options.PauseDeploymentTimeAnnotation = cfg.Annotations.PausedAt
	options.LogFormat = cfg.LogFormat
	options.LogLevel = cfg.LogLevel
	options.WebhookUrl = cfg.WebhookURL
	options.ResourcesToIgnore = cfg.IgnoredResources
	options.WorkloadTypesToIgnore = cfg.IgnoredWorkloads
	options.NamespacesToIgnore = cfg.IgnoredNamespaces
	options.NamespaceSelectors = cfg.NamespaceSelectorStrings
	options.ResourceSelectors = cfg.ResourceSelectorStrings
	options.EnableHA = cfg.EnableHA
	options.SyncAfterRestart = cfg.SyncAfterRestart
	options.EnablePProf = cfg.EnablePProf
	options.PProfAddr = cfg.PProfAddr

	// Convert ReloadStrategy to string for old options
	options.ReloadStrategy = string(cfg.ReloadStrategy)

	// Convert bool flags to string for old options (IsArgoRollouts, ReloadOnCreate, ReloadOnDelete)
	if cfg.ArgoRolloutsEnabled {
		options.IsArgoRollouts = "true"
	} else {
		options.IsArgoRollouts = "false"
	}
	if cfg.ReloadOnCreate {
		options.ReloadOnCreate = "true"
	} else {
		options.ReloadOnCreate = "false"
	}
	if cfg.ReloadOnDelete {
		options.ReloadOnDelete = "true"
	} else {
		options.ReloadOnDelete = "false"
	}
}

func configureLogging(logFormat, logLevel string) error {
	switch logFormat {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		// just let the library use default on empty string.
		if logFormat != "" {
			return fmt.Errorf("unsupported logging formatter: %q", logFormat)
		}
	}
	// set log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	logrus.SetLevel(level)
	return nil
}

func validateHAEnvs() error {
	podName, podNamespace := getHAEnvs()

	if podName == "" {
		return fmt.Errorf("%s not set, cannot run in HA mode without %s set", constants.PodNameEnv, constants.PodNameEnv)
	}
	if podNamespace == "" {
		return fmt.Errorf("%s not set, cannot run in HA mode without %s set", constants.PodNamespaceEnv, constants.PodNamespaceEnv)
	}
	return nil
}

func getHAEnvs() (string, string) {
	podName := os.Getenv(constants.PodNameEnv)
	podNamespace := os.Getenv(constants.PodNamespaceEnv)

	return podName, podNamespace
}

func startReloader(cmd *cobra.Command, args []string) {
	common.GetCommandLineOptions()
	err := configureLogging(cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		logrus.Warn(err)
	}

	logrus.Info("Starting Reloader")
	isGlobal := false
	currentNamespace := os.Getenv("KUBERNETES_NAMESPACE")
	if len(currentNamespace) == 0 {
		currentNamespace = v1.NamespaceAll
		isGlobal = true
		logrus.Warnf("KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces.")
	}

	// create the clientset
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		logrus.Fatal(err)
	}

	// Use config's IgnoredResources (already validated and normalized to lowercase)
	ignoredResourcesList := util.List(cfg.IgnoredResources)

	ignoredNamespacesList := cfg.IgnoredNamespaces
	namespaceLabelSelector := ""

	if isGlobal {
		namespaceLabelSelector, err = common.GetNamespaceLabelSelector(options.NamespaceSelectors)
		if err != nil {
			logrus.Fatal(err)
		}
	}

	resourceLabelSelector, err := common.GetResourceLabelSelector(options.ResourceSelectors)
	if err != nil {
		logrus.Fatal(err)
	}

	if len(namespaceLabelSelector) > 0 {
		logrus.Warnf("namespace-selector is set, will only detect changes in namespaces with these labels: %s.", namespaceLabelSelector)
	}

	if len(resourceLabelSelector) > 0 {
		logrus.Warnf("resource-label-selector is set, will only detect changes on resources with these labels: %s.", resourceLabelSelector)
	}

	if cfg.WebhookURL != "" {
		logrus.Warnf("webhook-url is set, will only send webhook, no resources will be reloaded")
	}

	collectors := metrics.SetupPrometheusEndpoint()

	var controllers []*controller.Controller
	for k := range kube.ResourceMap {
		if ignoredResourcesList.Contains(k) || (len(namespaceLabelSelector) == 0 && k == "namespaces") {
			continue
		}

		c, err := controller.NewController(clientset, k, currentNamespace, ignoredNamespacesList, namespaceLabelSelector, resourceLabelSelector, collectors)
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		controllers = append(controllers, c)

		// If HA is enabled we only run the controller when we're the leader
		if cfg.EnableHA {
			continue
		}
		// Now let's start the controller
		stop := make(chan struct{})
		defer close(stop)
		logrus.Infof("Starting Controller to watch resource type: %s", k)
		go c.Run(1, stop)
	}

	// Run leadership election
	if cfg.EnableHA {
		podName, podNamespace := getHAEnvs()
		lock := leadership.GetNewLock(clientset.CoordinationV1(), constants.LockName, podName, podNamespace)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go leadership.RunLeaderElection(lock, ctx, cancel, podName, controllers)
	}

	common.PublishMetaInfoConfigmap(clientset)

	if cfg.EnablePProf {
		go startPProfServer()
	}

	leadership.SetupLivenessEndpoint()
	logrus.Fatal(http.ListenAndServe(cfg.MetricsAddr, nil))
}

func startPProfServer() {
	logrus.Infof("Starting pprof server on %s", cfg.PProfAddr)
	if err := http.ListenAndServe(cfg.PProfAddr, nil); err != nil {
		logrus.Errorf("Failed to start pprof server: %v", err)
	}
}
