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
	fs.BoolVar(&cfg.AutoReloadAll, "auto-reload-all", cfg.AutoReloadAll,
		"Automatically reload all resources when their configmaps/secrets are updated, without requiring annotations")

	// Reload strategy
	fs.StringVar((*string)(&cfg.ReloadStrategy), "reload-strategy", string(cfg.ReloadStrategy),
		"Strategy for triggering workload restart: 'env-vars' (default, GitOps friendly) or 'annotations'")

	// Argo Rollouts
	fs.StringVar(&fv.isArgoRollouts, "is-argo-rollouts", "false",
		"Enable Argo Rollouts support (true/false)")

	// Event watching
	fs.StringVar(&fv.reloadOnCreate, "reload-on-create", "false",
		"Reload when configmaps/secrets are created (true/false)")
	fs.StringVar(&fv.reloadOnDelete, "reload-on-delete", "false",
		"Reload when configmaps/secrets are deleted (true/false)")

	// Sync after restart
	fs.BoolVar(&cfg.SyncAfterRestart, "sync-after-restart", cfg.SyncAfterRestart,
		"Trigger sync operation after restart")

	// High availability
	fs.BoolVar(&cfg.EnableHA, "enable-ha", cfg.EnableHA,
		"Enable high-availability mode with leader election")

	// Webhook
	fs.StringVar(&cfg.WebhookURL, "webhook-url", cfg.WebhookURL,
		"URL to send notification instead of triggering reload")

	// Filtering - resources
	fs.StringVar(&fv.ignoredResources, "resources-to-ignore", "",
		"Comma-separated list of configmap/secret names to ignore (case-insensitive)")
	fs.StringVar(&fv.ignoredWorkloads, "workload-types-to-ignore", "",
		"Comma-separated list of workload types to ignore (Deployment, DaemonSet, StatefulSet)")
	fs.StringVar(&fv.ignoredNamespaces, "namespaces-to-ignore", "",
		"Comma-separated list of namespaces to ignore")

	// Filtering - selectors
	fs.StringVar(&fv.namespaceSelectors, "namespace-selector", "",
		"Comma-separated list of namespace label selectors")
	fs.StringVar(&fv.resourceSelectors, "resource-label-selector", "",
		"Comma-separated list of resource label selectors")

	// Logging
	fs.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat,
		"Log format: 'json' or empty for default")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel,
		"Log level: trace, debug, info, warning, error, fatal, panic")

	// Metrics
	fs.StringVar(&cfg.MetricsAddr, "metrics-addr", cfg.MetricsAddr,
		"Address to serve metrics on")

	// Profiling
	fs.BoolVar(&cfg.EnablePProf, "enable-pprof", cfg.EnablePProf,
		"Enable pprof profiling server")
	fs.StringVar(&cfg.PProfAddr, "pprof-addr", cfg.PProfAddr,
		"Address for pprof server")

	// Annotation customization
	fs.StringVar(&cfg.Annotations.Auto, "auto-annotation", cfg.Annotations.Auto,
		"Custom annotation for auto-reload")
	fs.StringVar(&cfg.Annotations.ConfigmapAuto, "configmap-auto-annotation", cfg.Annotations.ConfigmapAuto,
		"Custom annotation for configmap auto-reload")
	fs.StringVar(&cfg.Annotations.SecretAuto, "secret-auto-annotation", cfg.Annotations.SecretAuto,
		"Custom annotation for secret auto-reload")
	fs.StringVar(&cfg.Annotations.ConfigmapReload, "configmap-reload-annotation", cfg.Annotations.ConfigmapReload,
		"Custom annotation for configmap reload")
	fs.StringVar(&cfg.Annotations.SecretReload, "secret-reload-annotation", cfg.Annotations.SecretReload,
		"Custom annotation for secret reload")
	fs.StringVar(&cfg.Annotations.Ignore, "ignore-annotation", cfg.Annotations.Ignore,
		"Custom annotation for ignoring resources")
	fs.StringVar(&cfg.Annotations.Search, "search-annotation", cfg.Annotations.Search,
		"Custom annotation for search-based matching")
	fs.StringVar(&cfg.Annotations.Match, "match-annotation", cfg.Annotations.Match,
		"Custom annotation for match-based matching")

	// Watched namespace (for single-namespace mode)
	fs.StringVar(&cfg.WatchedNamespace, "watch-namespace", cfg.WatchedNamespace,
		"Namespace to watch (empty for all namespaces)")
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

	// Parse selectors
	var err error
	cfg.NamespaceSelectors, err = ParseSelectors(splitAndTrim(fv.namespaceSelectors))
	if err != nil {
		return err
	}
	cfg.ResourceSelectors, err = ParseSelectors(splitAndTrim(fv.resourceSelectors))
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
