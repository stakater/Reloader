// Package config provides configuration management for Reloader.
// It replaces the old global variables pattern with an immutable Config struct.
package config

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
)

// ReloadStrategy defines how Reloader triggers workload restarts.
type ReloadStrategy string

const (
	// ReloadStrategyEnvVars adds/updates environment variables to trigger restart.
	// This is the default and recommended strategy for GitOps compatibility.
	ReloadStrategyEnvVars ReloadStrategy = "env-vars"

	// ReloadStrategyAnnotations adds/updates pod template annotations to trigger restart.
	ReloadStrategyAnnotations ReloadStrategy = "annotations"
)

// ArgoRolloutStrategy defines the strategy for Argo Rollout updates.
type ArgoRolloutStrategy string

const (
	// ArgoRolloutStrategyRestart uses the restart mechanism for Argo Rollouts.
	ArgoRolloutStrategyRestart ArgoRolloutStrategy = "restart"

	// ArgoRolloutStrategyRollout uses the rollout mechanism for Argo Rollouts.
	ArgoRolloutStrategyRollout ArgoRolloutStrategy = "rollout"
)

// Config holds all configuration for Reloader.
// This struct is immutable after creation - all fields should be set during initialization.
type Config struct {
	// Annotations holds customizable annotation keys.
	Annotations AnnotationConfig

	// AutoReloadAll enables automatic reload for all resources without requiring annotations.
	AutoReloadAll bool

	// ReloadStrategy determines how workload restarts are triggered.
	ReloadStrategy ReloadStrategy

	// ArgoRolloutsEnabled enables support for Argo Rollouts workload type.
	ArgoRolloutsEnabled bool

	// ArgoRolloutStrategy determines how Argo Rollouts are updated.
	ArgoRolloutStrategy ArgoRolloutStrategy

	// ReloadOnCreate enables watching for resource creation events.
	ReloadOnCreate bool

	// ReloadOnDelete enables watching for resource deletion events.
	ReloadOnDelete bool

	// SyncAfterRestart triggers a sync operation after a restart is performed.
	SyncAfterRestart bool

	// EnableHA enables high-availability mode with leader election.
	EnableHA bool

	// WebhookURL is an optional URL to send notifications to instead of triggering reload.
	WebhookURL string

	// Filtering configuration
	IgnoredResources   []string // ConfigMaps/Secrets to ignore (case-insensitive)
	IgnoredWorkloads   []string // Workload types to ignore
	IgnoredNamespaces  []string // Namespaces to ignore
	NamespaceSelectors []labels.Selector
	ResourceSelectors  []labels.Selector

	// Raw selector strings (for backward compatibility with old code)
	NamespaceSelectorStrings []string
	ResourceSelectorStrings  []string

	// Logging configuration
	LogFormat string // "json" or "" for default
	LogLevel  string // trace, debug, info, warning, error, fatal, panic

	// Metrics configuration
	MetricsAddr string // Address to serve metrics on (default :9090)

	// Health probe configuration
	HealthAddr string // Address to serve health probes on (default :8081)

	// Profiling configuration
	EnablePProf bool
	PProfAddr   string

	// Alerting configuration
	Alerting AlertingConfig

	// Leader election configuration
	LeaderElection LeaderElectionConfig

	// WatchedNamespace limits watching to a specific namespace (empty = all namespaces)
	WatchedNamespace string

	// SyncPeriod is the period for re-syncing watched resources
	SyncPeriod time.Duration
}

// AnnotationConfig holds all customizable annotation keys.
type AnnotationConfig struct {
	// Prefix is the base prefix for all annotations (default: reloader.stakater.com)
	Prefix string

	// Auto annotations
	Auto          string // reloader.stakater.com/auto
	ConfigmapAuto string // configmap.reloader.stakater.com/auto
	SecretAuto    string // secret.reloader.stakater.com/auto

	// Reload annotations (explicit resource names)
	ConfigmapReload string // configmap.reloader.stakater.com/reload
	SecretReload    string // secret.reloader.stakater.com/reload

	// Exclude annotations
	ConfigmapExclude string // configmaps.exclude.reloader.stakater.com/reload
	SecretExclude    string // secrets.exclude.reloader.stakater.com/reload

	// Ignore annotation
	Ignore string // reloader.stakater.com/ignore

	// Search/Match annotations
	Search string // reloader.stakater.com/search
	Match  string // reloader.stakater.com/match

	// Rollout strategy annotation
	RolloutStrategy string // reloader.stakater.com/rollout-strategy

	// Pause annotations
	PausePeriod string // deployment.reloader.stakater.com/pause-period
	PausedAt    string // deployment.reloader.stakater.com/paused-at

	// Last reloaded from annotation (set by Reloader)
	LastReloadedFrom string // reloader.stakater.com/last-reloaded-from
}

// AlertingConfig holds configuration for alerting integrations.
type AlertingConfig struct {
	// Enabled enables alerting notifications on reload events.
	Enabled bool

	// WebhookURL is the webhook URL to send alerts to.
	WebhookURL string

	// Sink determines the alert format: "slack", "teams", "gchat", or "raw" (default).
	Sink string

	// Proxy is an optional HTTP proxy for webhook requests.
	Proxy string

	// Additional is optional context prepended to alert messages.
	Additional string
}

// LeaderElectionConfig holds configuration for leader election.
type LeaderElectionConfig struct {
	LockName  string
	Namespace string
	Identity  string
}

// NewDefault creates a Config with default values.
func NewDefault() *Config {
	return &Config{
		Annotations:         DefaultAnnotations(),
		AutoReloadAll:       false,
		ReloadStrategy:      ReloadStrategyEnvVars,
		ArgoRolloutsEnabled: false,
		ArgoRolloutStrategy: ArgoRolloutStrategyRollout,
		ReloadOnCreate:      false,
		ReloadOnDelete:      false,
		SyncAfterRestart:    false,
		EnableHA:            false,
		WebhookURL:          "",
		IgnoredResources:    []string{},
		IgnoredWorkloads:    []string{},
		IgnoredNamespaces:   []string{},
		NamespaceSelectors:  []labels.Selector{},
		ResourceSelectors:   []labels.Selector{},
		LogFormat:           "",
		LogLevel:            "info",
		MetricsAddr:         ":9090",
		HealthAddr:          ":8081",
		EnablePProf:         false,
		PProfAddr:           ":6060",
		Alerting:            AlertingConfig{},
		LeaderElection: LeaderElectionConfig{
			LockName: "stakater-reloader-lock",
		},
		WatchedNamespace: "",
		SyncPeriod:       0,
	}
}

// DefaultAnnotations returns the default annotation configuration.
func DefaultAnnotations() AnnotationConfig {
	return AnnotationConfig{
		Prefix:           "reloader.stakater.com",
		Auto:             "reloader.stakater.com/auto",
		ConfigmapAuto:    "configmap.reloader.stakater.com/auto",
		SecretAuto:       "secret.reloader.stakater.com/auto",
		ConfigmapReload:  "configmap.reloader.stakater.com/reload",
		SecretReload:     "secret.reloader.stakater.com/reload",
		ConfigmapExclude: "configmaps.exclude.reloader.stakater.com/reload",
		SecretExclude:    "secrets.exclude.reloader.stakater.com/reload",
		Ignore:           "reloader.stakater.com/ignore",
		Search:           "reloader.stakater.com/search",
		Match:            "reloader.stakater.com/match",
		RolloutStrategy:  "reloader.stakater.com/rollout-strategy",
		PausePeriod:      "deployment.reloader.stakater.com/pause-period",
		PausedAt:         "deployment.reloader.stakater.com/paused-at",
		LastReloadedFrom: "reloader.stakater.com/last-reloaded-from",
	}
}

// IsResourceIgnored checks if a resource name should be ignored (case-insensitive).
func (c *Config) IsResourceIgnored(name string) bool {
	for _, ignored := range c.IgnoredResources {
		if equalFold(ignored, name) {
			return true
		}
	}
	return false
}

// IsWorkloadIgnored checks if a workload type should be ignored (case-insensitive).
func (c *Config) IsWorkloadIgnored(workloadType string) bool {
	for _, ignored := range c.IgnoredWorkloads {
		if equalFold(ignored, workloadType) {
			return true
		}
	}
	return false
}

// IsNamespaceIgnored checks if a namespace should be ignored.
func (c *Config) IsNamespaceIgnored(namespace string) bool {
	for _, ignored := range c.IgnoredNamespaces {
		if ignored == namespace {
			return true
		}
	}
	return false
}

// equalFold is a simple case-insensitive string comparison.
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c1, c2 := s[i], t[i]
		if c1 != c2 {
			// Convert to lowercase for comparison
			if 'A' <= c1 && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if 'A' <= c2 && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				return false
			}
		}
	}
	return true
}
