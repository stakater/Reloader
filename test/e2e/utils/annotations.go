package utils

// Annotation key constants used by Reloader.
// These follow the pattern: {scope}.reloader.stakater.com/{action}
// where scope can be empty (all resources), "configmap", "secret", "deployment", etc.
const (
	// ============================================================
	// Core reload annotations
	// ============================================================

	// AnnotationLastReloadedFrom is set by Reloader on workloads to track the last resource
	// that triggered a reload. Format: "{namespace}/{resource-type}/{resource-name}"
	AnnotationLastReloadedFrom = "reloader.stakater.com/last-reloaded-from"

	// AnnotationConfigMapReload triggers reload when specified ConfigMap(s) change.
	// Value: comma-separated list of ConfigMap names, e.g., "config1,config2"
	AnnotationConfigMapReload = "configmap.reloader.stakater.com/reload"

	// AnnotationSecretReload triggers reload when specified Secret(s) change.
	// Value: comma-separated list of Secret names, e.g., "secret1,secret2"
	AnnotationSecretReload = "secret.reloader.stakater.com/reload"

	// ============================================================
	// Auto-reload annotations
	// ============================================================

	// AnnotationAuto enables auto-reload for all referenced ConfigMaps and Secrets.
	// Value: "true" or "false"
	AnnotationAuto = "reloader.stakater.com/auto"

	// AnnotationConfigMapAuto enables auto-reload for all referenced ConfigMaps only.
	// Value: "true" or "false"
	AnnotationConfigMapAuto = "configmap.reloader.stakater.com/auto"

	// AnnotationSecretAuto enables auto-reload for all referenced Secrets only.
	// Value: "true" or "false"
	AnnotationSecretAuto = "secret.reloader.stakater.com/auto"

	// ============================================================
	// Exclude annotations (used with auto=true to exclude specific resources)
	// ============================================================

	// AnnotationConfigMapExclude excludes specified ConfigMaps from auto-reload.
	// Value: comma-separated list of ConfigMap names
	AnnotationConfigMapExclude = "configmaps.exclude.reloader.stakater.com/reload"

	// AnnotationSecretExclude excludes specified Secrets from auto-reload.
	// Value: comma-separated list of Secret names
	AnnotationSecretExclude = "secrets.exclude.reloader.stakater.com/reload"

	// ============================================================
	// Search annotations (for regex matching)
	// ============================================================

	// AnnotationSearch enables regex search mode for ConfigMap/Secret names.
	// Value: "true"
	// Used with reload annotation where value is a regex pattern.
	AnnotationSearch = "reloader.stakater.com/search"

	// AnnotationMatch is an alias for AnnotationSearch.
	// Value: "true"
	AnnotationMatch = "reloader.stakater.com/match"

	// ============================================================
	// Resource-level annotations (placed on ConfigMap/Secret)
	// ============================================================

	// AnnotationIgnore prevents Reloader from triggering reloads for this resource.
	// Place this on a ConfigMap or Secret to exclude it from reload triggers.
	// Value: "true"
	AnnotationIgnore = "reloader.stakater.com/ignore"

	// ============================================================
	// Pause/period annotations
	// ============================================================

	// AnnotationDeploymentPausePeriod sets a pause period before triggering reload.
	// Value: duration string, e.g., "10s", "1m"
	AnnotationDeploymentPausePeriod = "deployment.reloader.stakater.com/pause-period"

	// AnnotationDeploymentPausedAt is set by Reloader when a workload is paused.
	// Value: RFC3339 timestamp
	AnnotationDeploymentPausedAt = "deployment.reloader.stakater.com/paused-at"

	// ============================================================
	// Argo Rollouts specific annotations
	// ============================================================

	// AnnotationRolloutStrategy specifies the strategy for Argo Rollouts.
	// Value: "restart" (sets spec.restartAt)
	AnnotationRolloutStrategy = "reloader.stakater.com/rollout-strategy"
)

// Annotation values.
const (
	// AnnotationValueTrue is the string "true" for annotation values.
	AnnotationValueTrue = "true"

	// AnnotationValueFalse is the string "false" for annotation values.
	AnnotationValueFalse = "false"

	// AnnotationValueRestart is the "restart" strategy value for Argo Rollouts.
	AnnotationValueRestart = "restart"
)

// BuildConfigMapReloadAnnotation creates an annotation map for ConfigMap reload.
func BuildConfigMapReloadAnnotation(configMapNames ...string) map[string]string {
	return map[string]string{
		AnnotationConfigMapReload: joinNames(configMapNames),
	}
}

// BuildSecretReloadAnnotation creates an annotation map for Secret reload.
func BuildSecretReloadAnnotation(secretNames ...string) map[string]string {
	return map[string]string{
		AnnotationSecretReload: joinNames(secretNames),
	}
}

// BuildAutoTrueAnnotation creates an annotation map with auto=true.
func BuildAutoTrueAnnotation() map[string]string {
	return map[string]string{
		AnnotationAuto: AnnotationValueTrue,
	}
}

// BuildAutoFalseAnnotation creates an annotation map with auto=false.
func BuildAutoFalseAnnotation() map[string]string {
	return map[string]string{
		AnnotationAuto: AnnotationValueFalse,
	}
}

// BuildConfigMapAutoAnnotation creates an annotation map with configmap auto=true.
func BuildConfigMapAutoAnnotation() map[string]string {
	return map[string]string{
		AnnotationConfigMapAuto: AnnotationValueTrue,
	}
}

// BuildSecretAutoAnnotation creates an annotation map with secret auto=true.
func BuildSecretAutoAnnotation() map[string]string {
	return map[string]string{
		AnnotationSecretAuto: AnnotationValueTrue,
	}
}

// BuildSearchAnnotation creates an annotation map to enable search mode.
func BuildSearchAnnotation() map[string]string {
	return map[string]string{
		AnnotationSearch: AnnotationValueTrue,
	}
}

// BuildMatchAnnotation creates an annotation map to enable match mode.
func BuildMatchAnnotation() map[string]string {
	return map[string]string{
		AnnotationMatch: AnnotationValueTrue,
	}
}

// BuildIgnoreAnnotation creates an annotation map to ignore a resource.
func BuildIgnoreAnnotation() map[string]string {
	return map[string]string{
		AnnotationIgnore: AnnotationValueTrue,
	}
}

// BuildRolloutRestartStrategyAnnotation creates an annotation for Argo Rollout restart strategy.
func BuildRolloutRestartStrategyAnnotation() map[string]string {
	return map[string]string{
		AnnotationRolloutStrategy: AnnotationValueRestart,
	}
}

// BuildConfigMapExcludeAnnotation creates an annotation to exclude ConfigMaps from auto-reload.
func BuildConfigMapExcludeAnnotation(configMapNames ...string) map[string]string {
	return map[string]string{
		AnnotationConfigMapExclude: joinNames(configMapNames),
	}
}

// BuildSecretExcludeAnnotation creates an annotation to exclude Secrets from auto-reload.
func BuildSecretExcludeAnnotation(secretNames ...string) map[string]string {
	return map[string]string{
		AnnotationSecretExclude: joinNames(secretNames),
	}
}

// BuildPausePeriodAnnotation creates an annotation for deployment pause period.
func BuildPausePeriodAnnotation(duration string) map[string]string {
	return map[string]string{
		AnnotationDeploymentPausePeriod: duration,
	}
}

// joinNames joins names with comma separator.
func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	result := names[0]
	for i := 1; i < len(names); i++ {
		result += "," + names[i]
	}
	return result
}
