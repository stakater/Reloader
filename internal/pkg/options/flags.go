package options

import "github.com/stakater/Reloader/internal/pkg/constants"

var (
	// Auto reload all resources when their corresponding configmaps/secrets are updated
	AutoReloadAll = false
	// ConfigmapUpdateOnChangeAnnotation is an annotation to detect changes in
	// configmaps specified by name
	ConfigmapUpdateOnChangeAnnotation = "configmap.reloader.stakater.com/reload"
	// SecretUpdateOnChangeAnnotation is an annotation to detect changes in
	// secrets specified by name
	SecretUpdateOnChangeAnnotation = "secret.reloader.stakater.com/reload"
	// ReloaderAutoAnnotation is an annotation to detect changes in secrets
	ReloaderAutoAnnotation = "reloader.stakater.com/auto"
	// AutoSearchAnnotation is an annotation to detect changes in
	// configmaps or triggers with the SearchMatchAnnotation
	AutoSearchAnnotation = "reloader.stakater.com/search"
	// SearchMatchAnnotation is an annotation to tag secrets to be found with
	// AutoSearchAnnotation
	SearchMatchAnnotation = "reloader.stakater.com/match"
	// LogFormat is the log format to use (json, or empty string for default)
	LogFormat = ""
	// IsArgoRollouts Adds support for argo rollouts
	IsArgoRollouts = "false"
	// ReloadStrategy Specify the update strategy
	ReloadStrategy = constants.EnvVarsReloadStrategy
	// ReloadOnCreate Adds support to watch create events
	ReloadOnCreate   = "false"
	SyncAfterRestart = false
	// EnableHA adds support for running multiple replicas via leadership election
	EnableHA = false
)
