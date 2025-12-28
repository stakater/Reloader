// Package config provides configuration management for Reloader.
package config

import (
	"strings"
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
	Annotations         AnnotationConfig    `json:"annotations"`
	AutoReloadAll       bool                `json:"autoReloadAll"`
	ReloadStrategy      ReloadStrategy      `json:"reloadStrategy"`
	ArgoRolloutsEnabled bool                `json:"argoRolloutsEnabled"`
	ArgoRolloutStrategy ArgoRolloutStrategy `json:"argoRolloutStrategy"`
	ReloadOnCreate      bool                `json:"reloadOnCreate"`
	ReloadOnDelete      bool                `json:"reloadOnDelete"`
	SyncAfterRestart    bool                `json:"syncAfterRestart"`
	EnableHA            bool                `json:"enableHA"`
	WebhookURL          string              `json:"webhookUrl,omitempty"`

	IgnoredResources         []string          `json:"ignoredResources,omitempty"`
	IgnoredWorkloads         []string          `json:"ignoredWorkloads,omitempty"`
	IgnoredNamespaces        []string          `json:"ignoredNamespaces,omitempty"`
	NamespaceSelectors       []labels.Selector `json:"-"`
	ResourceSelectors        []labels.Selector `json:"-"`
	NamespaceSelectorStrings []string          `json:"namespaceSelectors,omitempty"`
	ResourceSelectorStrings  []string          `json:"resourceSelectors,omitempty"`

	LogFormat   string `json:"logFormat,omitempty"`
	LogLevel    string `json:"logLevel"`
	MetricsAddr string `json:"metricsAddr"`
	HealthAddr  string `json:"healthAddr"`
	EnablePProf bool   `json:"enablePProf"`
	PProfAddr   string `json:"pprofAddr,omitempty"`

	Alerting         AlertingConfig       `json:"alerting"`
	LeaderElection   LeaderElectionConfig `json:"leaderElection"`
	WatchedNamespace string               `json:"watchedNamespace,omitempty"`
	SyncPeriod       time.Duration        `json:"syncPeriod"`
}

// AnnotationConfig holds customizable annotation keys.
type AnnotationConfig struct {
	Prefix           string `json:"prefix"`
	Auto             string `json:"auto"`
	ConfigmapAuto    string `json:"configmapAuto"`
	SecretAuto       string `json:"secretAuto"`
	ConfigmapReload  string `json:"configmapReload"`
	SecretReload     string `json:"secretReload"`
	ConfigmapExclude string `json:"configmapExclude"`
	SecretExclude    string `json:"secretExclude"`
	Ignore           string `json:"ignore"`
	Search           string `json:"search"`
	Match            string `json:"match"`
	RolloutStrategy  string `json:"rolloutStrategy"`
	PausePeriod      string `json:"pausePeriod"`
	PausedAt         string `json:"pausedAt"`
	LastReloadedFrom string `json:"lastReloadedFrom"`
}

// AlertingConfig holds configuration for alerting integrations.
type AlertingConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl,omitempty"`
	Sink       string `json:"sink,omitempty"`
	Proxy      string `json:"proxy,omitempty"`
	Additional string `json:"additional,omitempty"`
	Structured bool   `json:"structured,omitempty"` // For raw sink: send structured JSON instead of plain text
}

// LeaderElectionConfig holds configuration for leader election.
type LeaderElectionConfig struct {
	LockName        string        `json:"lockName"`
	Namespace       string        `json:"namespace,omitempty"`
	Identity        string        `json:"identity,omitempty"`
	LeaseDuration   time.Duration `json:"leaseDuration"`
	RenewDeadline   time.Duration `json:"renewDeadline"`
	RetryPeriod     time.Duration `json:"retryPeriod"`
	ReleaseOnCancel bool          `json:"releaseOnCancel"`
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
		if strings.EqualFold(ignored, name) {
			return true
		}
	}
	return false
}

// IsWorkloadIgnored checks if a workload type should be ignored (case-insensitive).
func (c *Config) IsWorkloadIgnored(workloadType string) bool {
	for _, ignored := range c.IgnoredWorkloads {
		if strings.EqualFold(ignored, workloadType) {
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
