package options

var (
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
	AutoSearchAnnotation = "reloader.stakater.com/annotated"
	// SearchMatchAnnotation is an annotation to tag secrets to be found with
	// AutoSearchAnnotation
	SearchMatchAnnotation = "reloader.stakater.com/match"
	// LogFormat is the log format to use (json, or empty string for default)
	LogFormat = ""
)
