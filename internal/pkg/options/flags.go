package options

import "github.com/stakater/Reloader/internal/pkg/constants"

type ArgoRolloutStrategy int

const (
	// RestartStrategy is the annotation value for restart strategy for rollouts
	RestartStrategy ArgoRolloutStrategy = iota
	// RolloutStrategy is the annotation value for rollout strategy for rollouts
	RolloutStrategy
)

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
	// ResourcesToIgnore is a list of resources to ignore when watching for changes
	ResourcesToIgnore = []string{}
	// NamespacesToIgnore is a list of namespace names to ignore when watching for changes
	NamespacesToIgnore = []string{}
	// NamespaceSelectors is a list of namespace selectors to watch for changes
	NamespaceSelectors = []string{}
	// ResourceSelectors is a list of resource selectors to watch for changes
	ResourceSelectors = []string{}
	// EnablePProf enables pprof for profiling
	EnablePProf = false
	// PProfAddr is the address to start pprof server on
	// Default is localhost:6060
	PProfAddr = "localhost:6060"
)

func ToArgoRolloutStrategy(s string) ArgoRolloutStrategy {
	switch s {
	case "restart":
		return RestartStrategy
	case "rollout":
		fallthrough
	default:
		return RolloutStrategy
	}
}
