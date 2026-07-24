package flags

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/stakater/Reloader/pkg/config"
)

// v is the viper instance for configuration.
var v *viper.Viper

func init() {
	v = viper.New()
	// Convert flag names like "alert-webhook-url" to env vars like "ALERT_WEBHOOK_URL"
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
}

// BindFlags binds configuration flags to the provided flag set.
// Call this before parsing flags, then call ApplyFlags after parsing.
func BindFlags(fs *pflag.FlagSet, cfg *config.Config) {
	// Auto reload
	fs.Bool(
		"auto-reload-all", cfg.AutoReloadAll,
		"Automatically reload all resources when their configmaps/secrets are updated, without requiring annotations",
	)

	// Reload strategy
	fs.String(
		"reload-strategy", string(cfg.ReloadStrategy),
		"Strategy for triggering workload restart: 'env-vars' (default, GitOps friendly) or 'annotations'",
	)

	// Argo Rollouts
	fs.String(
		"is-Argo-Rollouts", "false",
		"Enable Argo Rollouts support (true/false)",
	)

	// OpenShift DeploymentConfig
	fs.String(
		"is-openshift", "",
		"Enable OpenShift DeploymentConfig support (true/false/auto). Empty or 'auto' enables auto-detection",
	)

	// CSI integration
	fs.Bool(
		"enable-csi-integration", cfg.CSIIntegrationEnabled,
		"Enable CSI SecretProviderClass integration (requires secrets-store CSI driver CRDs)",
	)

	// Event watching
	fs.String(
		"reload-on-create", "false",
		"Reload when configmaps/secrets are created (true/false)",
	)
	fs.String(
		"reload-on-delete", "false",
		"Reload when configmaps/secrets are deleted (true/false)",
	)

	// Sync after restart
	fs.Bool(
		"sync-after-restart", cfg.SyncAfterRestart,
		"Trigger sync operation after restart",
	)

	// High availability / Leader election
	fs.Bool(
		"enable-ha", cfg.EnableHA,
		"Enable high-availability mode with leader election",
	)
	fs.String(
		"leader-election-id", cfg.LeaderElection.LockName,
		"Name of the lease resource for leader election",
	)
	fs.String(
		"leader-election-namespace", cfg.LeaderElection.Namespace,
		"Namespace for the leader election lease (defaults to pod namespace)",
	)
	fs.Duration(
		"leader-election-lease-duration", cfg.LeaderElection.LeaseDuration,
		"Duration that non-leader candidates will wait before attempting to acquire leadership",
	)
	fs.Duration(
		"leader-election-renew-deadline", cfg.LeaderElection.RenewDeadline,
		"Duration that the acting leader will retry refreshing leadership before giving up",
	)
	fs.Duration(
		"leader-election-retry-period", cfg.LeaderElection.RetryPeriod,
		"Duration between leader election retries",
	)
	fs.Bool(
		"leader-election-release-on-cancel", cfg.LeaderElection.ReleaseOnCancel,
		"Release the leader lock when the manager is stopped",
	)

	// Webhook
	fs.String(
		"webhook-url", cfg.WebhookURL,
		"URL to send notification instead of triggering reload",
	)

	// Filtering - resources
	fs.String(
		"resources-to-ignore", "",
		"Comma-separated list of resources to ignore (valid options: 'configMaps' or 'secrets')",
	)
	fs.String(
		"ignored-workload-types", "",
		"Comma-separated list of workload types to ignore (valid options: 'jobs', 'cronjobs', or both)",
	)
	fs.String(
		"namespaces-to-ignore", "",
		"Comma-separated list of namespaces to ignore",
	)

	// Filtering - selectors
	fs.StringSlice(
		"namespace-selector", nil,
		"Namespace label selectors (can be specified multiple times)",
	)
	fs.StringSlice(
		"resource-label-selector", nil,
		"Resource label selectors (can be specified multiple times)",
	)

	// Logging
	fs.String(
		"log-format", cfg.LogFormat,
		"Log format: 'json' or empty for default",
	)
	fs.String(
		"log-level", cfg.LogLevel,
		"Log level: trace, debug, info, warning, error, fatal, panic",
	)

	// Metrics
	fs.String(
		"metrics-addr", cfg.MetricsAddr,
		"Address to serve metrics on",
	)

	// Health probes
	fs.String(
		"health-addr", cfg.HealthAddr,
		"Address to serve health probes on",
	)

	// Profiling
	fs.Bool(
		"enable-pprof", cfg.EnablePProf,
		"Enable pprof profiling server",
	)
	fs.String(
		"pprof-addr", cfg.PProfAddr,
		"Address for pprof server",
	)

	// Annotation customization (flag names match v1 for backward compatibility)
	fs.String(
		"auto-annotation", cfg.Annotations.Auto,
		"Annotation to detect changes in secrets/configmaps",
	)
	fs.String(
		"configmap-auto-annotation", cfg.Annotations.ConfigmapAuto,
		"Annotation to detect changes in configmaps",
	)
	fs.String(
		"secret-auto-annotation", cfg.Annotations.SecretAuto,
		"Annotation to detect changes in secrets",
	)
	fs.String(
		"configmap-annotation", cfg.Annotations.ConfigmapReload,
		"Annotation to detect changes in configmaps, specified by name",
	)
	fs.String(
		"secret-annotation", cfg.Annotations.SecretReload,
		"Annotation to detect changes in secrets, specified by name",
	)
	fs.String(
		"configmap-exclude-annotation", cfg.Annotations.ConfigmapExclude,
		"Annotation to exclude named configmaps from triggering reloads",
	)
	fs.String(
		"secret-exclude-annotation", cfg.Annotations.SecretExclude,
		"Annotation to exclude named secrets from triggering reloads",
	)
	fs.String(
		"secretproviderclass-auto-annotation", cfg.Annotations.SecretProviderClassAuto,
		"Annotation to detect changes in secret provider classes (CSI)",
	)
	fs.String(
		"secretproviderclass-annotation", cfg.Annotations.SecretProviderClassReload,
		"Annotation to detect changes in secret provider classes (CSI), specified by name",
	)
	fs.String(
		"secretproviderclass-exclude-annotation", cfg.Annotations.SecretProviderClassExclude,
		"Annotation to exclude named secret provider classes (CSI) from triggering reloads",
	)
	fs.String(
		"auto-search-annotation", cfg.Annotations.Search,
		"Annotation to detect changes in configmaps or secrets tagged with special match annotation",
	)
	fs.String(
		"search-match-annotation", cfg.Annotations.Match,
		"Annotation to mark secrets or configmaps to match the search",
	)
	fs.String(
		"ignore-annotation", cfg.Annotations.Ignore,
		"Annotation to ignore changes on watched resources",
	)
	fs.String(
		"pause-deployment-annotation", cfg.Annotations.PausePeriod,
		"Annotation to define the time period to pause a deployment after a configmap/secret change",
	)
	fs.String(
		"pause-deployment-time-annotation", cfg.Annotations.PausedAt,
		"Annotation to indicate when a deployment was paused by Reloader",
	)

	// Watched namespaces (scoped mode). Empty means watch all namespaces;
	// KUBERNETES_NAMESPACE (env) is used as a single-namespace fallback.
	fs.StringSlice(
		"namespaces", nil,
		"explicit list of namespaces to watch (scoped mode; creates no ClusterRole)",
	)

	// Alerting
	fs.Bool(
		"alert-on-reload", cfg.Alerting.Enabled,
		"Enable sending alerts when resources are reloaded",
	)
	fs.String(
		"alert-webhook-url", cfg.Alerting.WebhookURL,
		"Webhook URL to send alerts to",
	)
	fs.String(
		"alert-sink", cfg.Alerting.Sink,
		"Alert sink type: 'slack', 'teams', 'gchat', or 'raw' (default)",
	)
	fs.String(
		"alert-proxy", cfg.Alerting.Proxy,
		"Proxy URL for alert webhook requests",
	)
	fs.String(
		"alert-additional-info", cfg.Alerting.Additional,
		"Additional info to include in alerts (e.g., cluster name)",
	)
	fs.Bool(
		"alert-structured", cfg.Alerting.Structured,
		"For raw sink: send structured JSON instead of plain text",
	)

	// Bind pflags to viper
	_ = v.BindPFlags(fs)

	// Bind legacy env var names that don't match the automatic conversion
	// (flag "alert-proxy" -> env "ALERT_PROXY", but legacy is "ALERT_WEBHOOK_PROXY")
	_ = v.BindEnv("alert-proxy", "ALERT_PROXY", "ALERT_WEBHOOK_PROXY")
}

// LoggingFlags returns the log format and level from parsed flags/env. The
// caller uses these to configure logging before ApplyFlags runs, so ApplyFlags
// can log warnings through a ready logger.
func LoggingFlags() (format, level string) {
	return v.GetString("log-format"), v.GetString("log-level")
}

// ApplyFlags applies flag values from viper to the config struct. Call this
// after parsing flags. It finalizes namespace scope and logs any warnings it
// produces through the given logger.
func ApplyFlags(cfg *config.Config, log logr.Logger) error {
	// Boolean flags
	cfg.AutoReloadAll = v.GetBool("auto-reload-all")
	cfg.SyncAfterRestart = v.GetBool("sync-after-restart")
	cfg.EnableHA = v.GetBool("enable-ha")
	cfg.EnablePProf = v.GetBool("enable-pprof")
	cfg.CSIIntegrationEnabled = v.GetBool("enable-csi-integration")

	// Boolean string flags (legacy format: "true"/"false" strings)
	cfg.ArgoRolloutsEnabled = parseBoolString(v.GetString("is-Argo-Rollouts"))
	cfg.ReloadOnCreate = parseBoolString(v.GetString("reload-on-create"))
	cfg.ReloadOnDelete = parseBoolString(v.GetString("reload-on-delete"))

	switch strings.ToLower(strings.TrimSpace(v.GetString("is-openshift"))) {
	case "true":
		cfg.DeploymentConfigEnabled = true
	case "false":
		cfg.DeploymentConfigEnabled = false
	default:
	}

	// String flags
	cfg.ReloadStrategy = config.ReloadStrategy(v.GetString("reload-strategy"))
	cfg.WebhookURL = v.GetString("webhook-url")
	cfg.LogFormat = v.GetString("log-format")
	cfg.LogLevel = v.GetString("log-level")
	cfg.MetricsAddr = v.GetString("metrics-addr")
	cfg.HealthAddr = v.GetString("health-addr")
	cfg.PProfAddr = v.GetString("pprof-addr")
	// Namespace scope: an explicit --namespaces list takes precedence (scoped
	// mode); otherwise fall back to KUBERNETES_NAMESPACE for single-namespace
	// mode; an empty result means global (all-namespaces) mode.
	// Trim and drop empty entries from the slice to prevent empty strings from
	// being treated as "watch all namespaces" by controller-runtime.
	cfg.WatchedNamespaces = trimAndDropEmptyStrings(v.GetStringSlice("namespaces"))
	if len(cfg.WatchedNamespaces) == 0 {
		if ns := v.GetString("KUBERNETES_NAMESPACE"); ns != "" {
			cfg.WatchedNamespaces = []string{ns}
		}
	}

	// Leader election
	cfg.LeaderElection.LockName = v.GetString("leader-election-id")
	cfg.LeaderElection.Namespace = v.GetString("leader-election-namespace")
	cfg.LeaderElection.LeaseDuration = v.GetDuration("leader-election-lease-duration")
	cfg.LeaderElection.RenewDeadline = v.GetDuration("leader-election-renew-deadline")
	cfg.LeaderElection.RetryPeriod = v.GetDuration("leader-election-retry-period")
	cfg.LeaderElection.ReleaseOnCancel = v.GetBool("leader-election-release-on-cancel")

	// Annotations
	cfg.Annotations.Auto = v.GetString("auto-annotation")
	cfg.Annotations.ConfigmapAuto = v.GetString("configmap-auto-annotation")
	cfg.Annotations.SecretAuto = v.GetString("secret-auto-annotation")
	cfg.Annotations.ConfigmapReload = v.GetString("configmap-annotation")
	cfg.Annotations.SecretReload = v.GetString("secret-annotation")
	cfg.Annotations.ConfigmapExclude = v.GetString("configmap-exclude-annotation")
	cfg.Annotations.SecretExclude = v.GetString("secret-exclude-annotation")
	cfg.Annotations.SecretProviderClassAuto = v.GetString("secretproviderclass-auto-annotation")
	cfg.Annotations.SecretProviderClassReload = v.GetString("secretproviderclass-annotation")
	cfg.Annotations.SecretProviderClassExclude = v.GetString("secretproviderclass-exclude-annotation")
	cfg.Annotations.Search = v.GetString("auto-search-annotation")
	cfg.Annotations.Match = v.GetString("search-match-annotation")
	cfg.Annotations.Ignore = v.GetString("ignore-annotation")
	cfg.Annotations.PausePeriod = v.GetString("pause-deployment-annotation")
	cfg.Annotations.PausedAt = v.GetString("pause-deployment-time-annotation")

	// Alerting
	cfg.Alerting.Enabled = v.GetBool("alert-on-reload")
	cfg.Alerting.WebhookURL = v.GetString("alert-webhook-url")
	cfg.Alerting.Sink = strings.ToLower(v.GetString("alert-sink"))
	cfg.Alerting.Proxy = v.GetString("alert-proxy")
	cfg.Alerting.Additional = v.GetString("alert-additional-info")
	cfg.Alerting.Structured = v.GetBool("alert-structured")

	// Special case: if webhook URL is set, auto-enable alerting
	if cfg.Alerting.WebhookURL != "" {
		cfg.Alerting.Enabled = true
	}

	// Parse comma-separated lists
	cfg.IgnoredResources = splitAndTrim(v.GetString("resources-to-ignore"))
	cfg.IgnoredWorkloads = splitAndTrim(v.GetString("ignored-workload-types"))
	cfg.IgnoredNamespaces = splitAndTrim(v.GetString("namespaces-to-ignore"))

	// Get selector slices and join with comma
	nsSelectors := v.GetStringSlice("namespace-selector")
	resSelectors := v.GetStringSlice("resource-label-selector")

	if len(nsSelectors) > 0 {
		cfg.NamespaceSelectorStrings = nsSelectors
	}
	if len(resSelectors) > 0 {
		cfg.ResourceSelectorStrings = resSelectors
	}

	if len(nsSelectors) > 0 {
		joinedNS := strings.Join(nsSelectors, ",")
		selector, err := labels.Parse(joinedNS)
		if err != nil {
			return fmt.Errorf("invalid selector %q: %w", joinedNS, err)
		}
		cfg.NamespaceSelectors = []labels.Selector{selector}
	}
	if len(resSelectors) > 0 {
		joinedRes := strings.Join(resSelectors, ",")
		selector, err := labels.Parse(joinedRes)
		if err != nil {
			return fmt.Errorf("invalid selector %q: %w", joinedRes, err)
		}
		cfg.ResourceSelectors = []labels.Selector{selector}
	}

	// Ensure duration defaults are preserved if not set
	if cfg.LeaderElection.LeaseDuration == 0 {
		cfg.LeaderElection.LeaseDuration = 15 * time.Second
	}
	if cfg.LeaderElection.RenewDeadline == 0 {
		cfg.LeaderElection.RenewDeadline = 10 * time.Second
	}
	if cfg.LeaderElection.RetryPeriod == 0 {
		cfg.LeaderElection.RetryPeriod = 2 * time.Second
	}

	// Namespace-selector and namespaces-to-ignore are only honored in global
	// mode; in scoped or single-namespace mode the watched set is already
	// explicit, so drop them and log where it happens.
	if !cfg.IsGlobalMode() {
		if len(cfg.NamespaceSelectors) > 0 {
			log.Info("namespace-selector is set but is only honored in global mode; ignoring it")
			cfg.NamespaceSelectors = nil
			cfg.NamespaceSelectorStrings = nil
		}
		if len(cfg.IgnoredNamespaces) > 0 {
			log.Info("namespaces-to-ignore is set but is only honored in global mode; ignoring it")
			cfg.IgnoredNamespaces = nil
		}
	}
	return nil
}

// parseBoolString parses a string as a boolean, defaulting to false.
func parseBoolString(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}

// ShouldAutoDetectOpenShift returns true if OpenShift DeploymentConfig support
// should be auto-detected (i.e., the --is-openshift flag was not explicitly set).
func ShouldAutoDetectOpenShift() bool {
	val := strings.ToLower(strings.TrimSpace(v.GetString("is-openshift")))
	return val == "" || val == "auto"
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// trimAndDropEmptyStrings trims whitespace from each string in a slice and drops empty entries.
func trimAndDropEmptyStrings(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
