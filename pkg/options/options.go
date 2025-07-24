package options

import (
	"strconv"

	"github.com/stakater/Reloader/internal/pkg/constants"
)

func init() {
	InitializeReloaderOptions()
}

// ReloaderOptions contains all configurable options for the Reloader controller.
// These options control how Reloader behaves when watching for changes in ConfigMaps and Secrets.
type ReloaderOptions struct {
	// AutoReloadAll enables automatic reloading of all resources when their corresponding ConfigMaps/Secrets are updated
	AutoReloadAll bool `json:"autoReloadAll"`
	// ConfigmapUpdateOnChangeAnnotation is the annotation key used to detect changes in ConfigMaps specified by name
	ConfigmapUpdateOnChangeAnnotation string `json:"configmapUpdateOnChangeAnnotation"`
	// SecretUpdateOnChangeAnnotation is the annotation key used to detect changes in Secrets specified by name
	SecretUpdateOnChangeAnnotation string `json:"secretUpdateOnChangeAnnotation"`
	// ReloaderAutoAnnotation is the annotation key used to detect changes in any referenced ConfigMaps or Secrets
	ReloaderAutoAnnotation string `json:"reloaderAutoAnnotation"`
	// IgnoreResourceAnnotation is the annotation key used to ignore resources from being watched
	IgnoreResourceAnnotation string `json:"ignoreResourceAnnotation"`
	// ConfigmapReloaderAutoAnnotation is the annotation key used to detect changes in ConfigMaps only
	ConfigmapReloaderAutoAnnotation string `json:"configmapReloaderAutoAnnotation"`
	// SecretReloaderAutoAnnotation is the annotation key used to detect changes in Secrets only
	SecretReloaderAutoAnnotation string `json:"secretReloaderAutoAnnotation"`
	// ConfigmapExcludeReloaderAnnotation is the annotation key containing comma-separated list of ConfigMaps to exclude from watching
	ConfigmapExcludeReloaderAnnotation string `json:"configmapExcludeReloaderAnnotation"`
	// SecretExcludeReloaderAnnotation is the annotation key containing comma-separated list of Secrets to exclude from watching
	SecretExcludeReloaderAnnotation string `json:"secretExcludeReloaderAnnotation"`
	// AutoSearchAnnotation is the annotation key used to detect changes in ConfigMaps/Secrets tagged with SearchMatchAnnotation
	AutoSearchAnnotation string `json:"autoSearchAnnotation"`
	// SearchMatchAnnotation is the annotation key used to tag ConfigMaps/Secrets to be found by AutoSearchAnnotation
	SearchMatchAnnotation string `json:"searchMatchAnnotation"`
	// RolloutStrategyAnnotation is the annotation key used to define the rollout update strategy for workloads
	RolloutStrategyAnnotation string `json:"rolloutStrategyAnnotation"`
	// PauseDeploymentAnnotation is the annotation key used to define the time period to pause a deployment after
	PauseDeploymentAnnotation string `json:"pauseDeploymentAnnotation"`
	// PauseDeploymentTimeAnnotation is the annotation key used to indicate when a deployment was paused by Reloader
	PauseDeploymentTimeAnnotation string `json:"pauseDeploymentTimeAnnotation"`

	// LogFormat specifies the log format to use (json, or empty string for default text format)
	LogFormat string `json:"logFormat"`
	// LogLevel specifies the log level to use (trace, debug, info, warning, error, fatal, panic)
	LogLevel string `json:"logLevel"`
	// IsArgoRollouts indicates whether support for Argo Rollouts is enabled
	IsArgoRollouts bool `json:"isArgoRollouts"`
	// ReloadStrategy specifies the strategy used to trigger resource reloads (env-vars or annotations)
	ReloadStrategy string `json:"reloadStrategy"`
	// ReloadOnCreate indicates whether to trigger reloads when ConfigMaps/Secrets are created
	ReloadOnCreate bool `json:"reloadOnCreate"`
	// ReloadOnDelete indicates whether to trigger reloads when ConfigMaps/Secrets are deleted
	ReloadOnDelete bool `json:"reloadOnDelete"`
	// SyncAfterRestart indicates whether to sync add events after Reloader restarts (only works when ReloadOnCreate is true)
	SyncAfterRestart bool `json:"syncAfterRestart"`
	// EnableHA indicates whether High Availability mode is enabled with leader election
	EnableHA bool `json:"enableHA"`
	// WebhookUrl is the URL to send webhook notifications to instead of performing reloads
	WebhookUrl string `json:"webhookUrl"`
	// ResourcesToIgnore is a list of resource types to ignore (e.g., "configmaps" or "secrets")
	ResourcesToIgnore []string `json:"resourcesToIgnore"`
	// NamespaceSelectors is a list of label selectors to filter namespaces to watch
	NamespaceSelectors []string `json:"namespaceSelectors"`
	// ResourceSelectors is a list of label selectors to filter ConfigMaps and Secrets to watch
	ResourceSelectors []string `json:"resourceSelectors"`
	// NamespacesToIgnore is a list of namespace names to ignore when watching for changes
	NamespacesToIgnore []string `json:"namespacesToIgnore"`
}

// Legacy options for backward compatibility
var (
	// Auto reload all resources when their corresponding configmaps/secrets are updated
	AutoReloadAll = false
	// ConfigmapUpdateOnChangeAnnotation is an annotation to detect changes in
	// configmaps specified by name
	ConfigmapUpdateOnChangeAnnotation = "configmap.reloader.stakater.com/reload"
	// SecretUpdateOnChangeAnnotation is an annotation to detect changes in
	// secrets specified by name
	SecretUpdateOnChangeAnnotation = "secret.reloader.stakater.com/reload"
	// ReloaderAutoAnnotation is an annotation to detect changes in secrets/configmaps
	ReloaderAutoAnnotation = "reloader.stakater.com/auto"
	// IgnoreResourceAnnotation is an annotation to ignore changes in secrets/configmaps
	IgnoreResourceAnnotation = "reloader.stakater.com/ignore"
	// ConfigmapReloaderAutoAnnotation is an annotation to detect changes in configmaps
	ConfigmapReloaderAutoAnnotation = "configmap.reloader.stakater.com/auto"
	// SecretReloaderAutoAnnotation is an annotation to detect changes in secrets
	SecretReloaderAutoAnnotation = "secret.reloader.stakater.com/auto"
	// ConfigmapReloaderAutoAnnotation is a comma separated list of configmaps that excludes detecting changes on cms
	ConfigmapExcludeReloaderAnnotation = "configmaps.exclude.reloader.stakater.com/reload"
	// SecretExcludeReloaderAnnotation is a comma separated list of secrets that excludes detecting changes on secrets
	SecretExcludeReloaderAnnotation = "secrets.exclude.reloader.stakater.com/reload"
	// AutoSearchAnnotation is an annotation to detect changes in
	// configmaps or triggers with the SearchMatchAnnotation
	AutoSearchAnnotation = "reloader.stakater.com/search"
	// SearchMatchAnnotation is an annotation to tag secrets to be found with
	// AutoSearchAnnotation
	SearchMatchAnnotation = "reloader.stakater.com/match"
	// RolloutStrategyAnnotation is an annotation to define rollout update strategy
	RolloutStrategyAnnotation = "reloader.stakater.com/rollout-strategy"
	// PauseDeploymentAnnotation is an annotation to define the time period to pause a deployment after
	// a configmap/secret change has been detected. Valid values are described here: https://pkg.go.dev/time#ParseDuration
	// only positive values are allowed
	PauseDeploymentAnnotation = "deployment.reloader.stakater.com/pause-period"
	// Annotation set by reloader to indicate that the deployment has been paused
	PauseDeploymentTimeAnnotation = "deployment.reloader.stakater.com/paused-at"
	// LogFormat is the log format to use (json, or empty string for default)
	LogFormat = ""
	// LogLevel is the log level to use (trace, debug, info, warning, error, fatal and panic)
	LogLevel = ""
	// IsArgoRollouts Adds support for argo rollouts
	IsArgoRollouts = "false"
	// ReloadStrategy Specify the update strategy
	ReloadStrategy = constants.EnvVarsReloadStrategy
	// ReloadOnCreate Adds support to watch create events
	ReloadOnCreate = "false"
	// ReloadOnDelete Adds support to watch delete events
	ReloadOnDelete   = "false"
	SyncAfterRestart = false
	// EnableHA adds support for running multiple replicas via leadership election
	EnableHA = false
	// Url to send a request to instead of triggering a reload
	WebhookUrl = ""

	ResourcesToIgnore = []string{}

	NamespacesToIgnore = []string{}

	NamespaceSelectors = []string{}

	ResourceSelectors = []string{}
)

var CommandLineOptions *ReloaderOptions

func InitializeReloaderOptions() {
	CommandLineOptions = &ReloaderOptions{
		AutoReloadAll:                      AutoReloadAll,
		ConfigmapUpdateOnChangeAnnotation:  ConfigmapUpdateOnChangeAnnotation,
		SecretUpdateOnChangeAnnotation:     SecretUpdateOnChangeAnnotation,
		ReloaderAutoAnnotation:             ReloaderAutoAnnotation,
		IgnoreResourceAnnotation:           IgnoreResourceAnnotation,
		ConfigmapReloaderAutoAnnotation:    ConfigmapReloaderAutoAnnotation,
		SecretReloaderAutoAnnotation:       SecretReloaderAutoAnnotation,
		ConfigmapExcludeReloaderAnnotation: ConfigmapExcludeReloaderAnnotation,
		SecretExcludeReloaderAnnotation:    SecretExcludeReloaderAnnotation,
		AutoSearchAnnotation:               AutoSearchAnnotation,
		SearchMatchAnnotation:              SearchMatchAnnotation,
		RolloutStrategyAnnotation:          RolloutStrategyAnnotation,
		PauseDeploymentAnnotation:          PauseDeploymentAnnotation,
		PauseDeploymentTimeAnnotation:      PauseDeploymentTimeAnnotation,
		LogFormat:                          LogFormat,
		LogLevel:                           LogLevel,
		ReloadStrategy:                     ReloadStrategy,
		SyncAfterRestart:                   SyncAfterRestart,
		EnableHA:                           EnableHA,
		WebhookUrl:                         WebhookUrl,
		ResourcesToIgnore:                  ResourcesToIgnore,
		NamespaceSelectors:                 NamespaceSelectors,
		ResourceSelectors:                  ResourceSelectors,
		NamespacesToIgnore:                 NamespacesToIgnore,
		IsArgoRollouts:                     parseBool(IsArgoRollouts),
		ReloadOnCreate:                     parseBool(ReloadOnCreate),
		ReloadOnDelete:                     parseBool(ReloadOnDelete),
	}
}

func parseBool(value string) bool {
	if value == "" {
		return false
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return false // Default to false if parsing fails
	}
	return result
}
