// Package config provides configuration management for Reloader.
package config

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
)

// ReloadStrategy defines how Reloader triggers workload restarts.
type ReloadStrategy string

const (
	ReloadStrategyEnvVars     ReloadStrategy = "env-vars"
	ReloadStrategyAnnotations ReloadStrategy = "annotations"
)

// ArgoRolloutStrategy defines the strategy for Argo Rollout updates.
type ArgoRolloutStrategy string

const (
	ArgoRolloutStrategyRestart ArgoRolloutStrategy = "restart"
	ArgoRolloutStrategyRollout ArgoRolloutStrategy = "rollout"
)

// Config holds all configuration for Reloader.
type Config struct {
	Annotations         AnnotationConfig
	AutoReloadAll       bool
	ReloadStrategy      ReloadStrategy
	ArgoRolloutsEnabled bool
	ArgoRolloutStrategy ArgoRolloutStrategy
	ReloadOnCreate      bool
	ReloadOnDelete      bool
	SyncAfterRestart    bool
	EnableHA            bool
	WebhookURL          string

	IgnoredResources         []string
	IgnoredWorkloads         []string
	IgnoredNamespaces        []string
	NamespaceSelectors       []labels.Selector
	ResourceSelectors        []labels.Selector
	NamespaceSelectorStrings []string
	ResourceSelectorStrings  []string

	LogFormat   string
	LogLevel    string
	MetricsAddr string
	HealthAddr  string
	EnablePProf bool
	PProfAddr   string

	Alerting         AlertingConfig
	LeaderElection   LeaderElectionConfig
	WatchedNamespace string
	SyncPeriod       time.Duration
}

// AnnotationConfig holds customizable annotation keys.
type AnnotationConfig struct {
	Prefix           string
	Auto             string
	ConfigmapAuto    string
	SecretAuto       string
	ConfigmapReload  string
	SecretReload     string
	ConfigmapExclude string
	SecretExclude    string
	Ignore           string
	Search           string
	Match            string
	RolloutStrategy  string
	PausePeriod      string
	PausedAt         string
	LastReloadedFrom string
}

// AlertingConfig holds configuration for alerting integrations.
type AlertingConfig struct {
	Enabled    bool
	WebhookURL string
	Sink       string
	Proxy      string
	Additional string
}

// LeaderElectionConfig holds configuration for leader election.
type LeaderElectionConfig struct {
	LockName        string
	Namespace       string
	Identity        string
	LeaseDuration   time.Duration
	RenewDeadline   time.Duration
	RetryPeriod     time.Duration
	ReleaseOnCancel bool
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
			LockName:        "reloader-leader-election",
			LeaseDuration:   15 * time.Second,
			RenewDeadline:   10 * time.Second,
			RetryPeriod:     2 * time.Second,
			ReleaseOnCancel: true,
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

func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c1, c2 := s[i], t[i]
		if c1 != c2 {
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
