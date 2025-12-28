package config

import (
	"strings"

	"github.com/spf13/pflag"
)

// flagValues holds intermediate string values from CLI flags
// that need further parsing into the Config struct.
type flagValues struct {
	namespaceSelectors string
	resourceSelectors  string
	ignoredResources   string
	ignoredWorkloads   string
	ignoredNamespaces  string
	isArgoRollouts     string
	reloadOnCreate     string
	reloadOnDelete     string
}

var fv flagValues

// BindFlags binds configuration flags to the provided flag set.
// Call this before parsing flags, then call ApplyFlags after parsing.
func BindFlags(fs *pflag.FlagSet, cfg *Config) {
	// Auto reload
	fs.BoolVar(
		&cfg.AutoReloadAll, "auto-reload-all", cfg.AutoReloadAll,
		"Automatically reload all resources when their configmaps/secrets are updated, without requiring annotations",
	)

	// Reload strategy
	fs.StringVar(
		(*string)(&cfg.ReloadStrategy), "reload-strategy", string(cfg.ReloadStrategy),
		"Strategy for triggering workload restart: 'env-vars' (default, GitOps friendly) or 'annotations'",
	)

	// Argo Rollouts
	fs.StringVar(
		&fv.isArgoRollouts, "is-Argo-Rollouts", "false",
		"Enable Argo Rollouts support (true/false)",
	)

	// Event watching
	fs.StringVar(
		&fv.reloadOnCreate, "reload-on-create", "false",
		"Reload when configmaps/secrets are created (true/false)",
	)
	fs.StringVar(
		&fv.reloadOnDelete, "reload-on-delete", "false",
		"Reload when configmaps/secrets are deleted (true/false)",
	)

	// Sync after restart
	fs.BoolVar(
		&cfg.SyncAfterRestart, "sync-after-restart", cfg.SyncAfterRestart,
		"Trigger sync operation after restart",
	)

	// High availability / Leader election
	fs.BoolVar(
		&cfg.EnableHA, "enable-ha", cfg.EnableHA,
		"Enable high-availability mode with leader election",
	)
	fs.StringVar(
		&cfg.LeaderElection.LockName, "leader-election-id", cfg.LeaderElection.LockName,
		"Name of the lease resource for leader election",
	)
	fs.StringVar(
		&cfg.LeaderElection.Namespace, "leader-election-namespace", cfg.LeaderElection.Namespace,
		"Namespace for the leader election lease (defaults to pod namespace)",
	)
	fs.DurationVar(
		&cfg.LeaderElection.LeaseDuration, "leader-election-lease-duration", cfg.LeaderElection.LeaseDuration,
		"Duration that non-leader candidates will wait before attempting to acquire leadership",
	)
	fs.DurationVar(
		&cfg.LeaderElection.RenewDeadline, "leader-election-renew-deadline", cfg.LeaderElection.RenewDeadline,
		"Duration that the acting leader will retry refreshing leadership before giving up",
	)
	fs.DurationVar(
		&cfg.LeaderElection.RetryPeriod, "leader-election-retry-period", cfg.LeaderElection.RetryPeriod,
		"Duration between leader election retries",
	)
	fs.BoolVar(
		&cfg.LeaderElection.ReleaseOnCancel, "leader-election-release-on-cancel", cfg.LeaderElection.ReleaseOnCancel,
		"Release the leader lock when the manager is stopped",
	)

	// Webhook
	fs.StringVar(
		&cfg.WebhookURL, "webhook-url", cfg.WebhookURL,
		"URL to send notification instead of triggering reload",
	)

	// Filtering - resources (use StringVar not StringSliceVar for simpler parsing)
	fs.StringVar(
		&fv.ignoredResources, "resources-to-ignore", "",
		"Comma-separated list of resources to ignore (valid options: 'configMaps' or 'secrets')",
	)
	fs.StringVar(
		&fv.ignoredWorkloads, "ignored-workload-types", "",
		"Comma-separated list of workload types to ignore (valid options: 'jobs', 'cronjobs', or both)",
	)
	fs.StringVar(
		&fv.ignoredNamespaces, "namespaces-to-ignore", "",
		"Comma-separated list of namespaces to ignore",
	)

	// Filtering - selectors
	fs.StringVar(
		&fv.namespaceSelectors, "namespace-selector", "",
		"Comma-separated list of namespace label selectors",
	)
	fs.StringVar(
		&fv.resourceSelectors, "resource-label-selector", "",
		"Comma-separated list of resource label selectors",
	)

	// Logging
	fs.StringVar(
		&cfg.LogFormat, "log-format", cfg.LogFormat,
		"Log format: 'json' or empty for default",
	)
	fs.StringVar(
		&cfg.LogLevel, "log-level", cfg.LogLevel,
		"Log level: trace, debug, info, warning, error, fatal, panic",
	)

	// Metrics
	fs.StringVar(
		&cfg.MetricsAddr, "metrics-addr", cfg.MetricsAddr,
		"Address to serve metrics on",
	)

	// Health probes
	fs.StringVar(
		&cfg.HealthAddr, "health-addr", cfg.HealthAddr,
		"Address to serve health probes on",
	)

	// Profiling
	fs.BoolVar(
		&cfg.EnablePProf, "enable-pprof", cfg.EnablePProf,
		"Enable pprof profiling server",
	)
	fs.StringVar(
		&cfg.PProfAddr, "pprof-addr", cfg.PProfAddr,
		"Address for pprof server",
	)

	// Annotation customization (flag names match v1 for backward compatibility)
	fs.StringVar(
		&cfg.Annotations.Auto, "auto-annotation", cfg.Annotations.Auto,
		"Annotation to detect changes in secrets/configmaps",
	)
	fs.StringVar(
		&cfg.Annotations.ConfigmapAuto, "configmap-auto-annotation", cfg.Annotations.ConfigmapAuto,
		"Annotation to detect changes in configmaps",
	)
	fs.StringVar(
		&cfg.Annotations.SecretAuto, "secret-auto-annotation", cfg.Annotations.SecretAuto,
		"Annotation to detect changes in secrets",
	)
	fs.StringVar(
		&cfg.Annotations.ConfigmapReload, "configmap-annotation", cfg.Annotations.ConfigmapReload,
		"Annotation to detect changes in configmaps, specified by name",
	)
	fs.StringVar(
		&cfg.Annotations.SecretReload, "secret-annotation", cfg.Annotations.SecretReload,
		"Annotation to detect changes in secrets, specified by name",
	)
	fs.StringVar(
		&cfg.Annotations.Search, "auto-search-annotation", cfg.Annotations.Search,
		"Annotation to detect changes in configmaps or secrets tagged with special match annotation",
	)
	fs.StringVar(
		&cfg.Annotations.Match, "search-match-annotation", cfg.Annotations.Match,
		"Annotation to mark secrets or configmaps to match the search",
	)
	fs.StringVar(
		&cfg.Annotations.PausePeriod, "pause-deployment-annotation", cfg.Annotations.PausePeriod,
		"Annotation to define the time period to pause a deployment after a configmap/secret change",
	)
	fs.StringVar(
		&cfg.Annotations.PausedAt, "pause-deployment-time-annotation", cfg.Annotations.PausedAt,
		"Annotation to indicate when a deployment was paused by Reloader",
	)

	// Watched namespace (for single-namespace mode)
	fs.StringVar(
		&cfg.WatchedNamespace, "watch-namespace", cfg.WatchedNamespace,
		"Namespace to watch (empty for all namespaces)",
	)
}

// ApplyFlags applies flag values that need post-processing.
// Call this after parsing flags.
func ApplyFlags(cfg *Config) error {
	// Parse boolean string flags
	cfg.ArgoRolloutsEnabled = parseBoolString(fv.isArgoRollouts)
	cfg.ReloadOnCreate = parseBoolString(fv.reloadOnCreate)
	cfg.ReloadOnDelete = parseBoolString(fv.reloadOnDelete)

	// Parse comma-separated lists
	cfg.IgnoredResources = splitAndTrim(fv.ignoredResources)
	cfg.IgnoredWorkloads = splitAndTrim(fv.ignoredWorkloads)
	cfg.IgnoredNamespaces = splitAndTrim(fv.ignoredNamespaces)

	// Store raw selector strings
	cfg.NamespaceSelectorStrings = splitAndTrim(fv.namespaceSelectors)
	cfg.ResourceSelectorStrings = splitAndTrim(fv.resourceSelectors)

	// Parse selectors into labels.Selector
	var err error
	cfg.NamespaceSelectors, err = ParseSelectors(cfg.NamespaceSelectorStrings)
	if err != nil {
		return err
	}
	cfg.ResourceSelectors, err = ParseSelectors(cfg.ResourceSelectorStrings)
	if err != nil {
		return err
	}

	return nil
}

// parseBoolString parses a string as a boolean, defaulting to false.
func parseBoolString(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
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
