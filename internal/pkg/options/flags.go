package options

var (
	// ConfigmapUpdateOnChangeAnnotation is an annotation to detect changes in configmaps
	ConfigmapUpdateOnChangeAnnotation = "configmap.reloader.stakater.com/reload"
	// SecretUpdateOnChangeAnnotation is an annotation to detect changes in secrets
	SecretUpdateOnChangeAnnotation = "secret.reloader.stakater.com/reload"
	// ReloaderAutoAnnotation is an annotation to detect changes in secrets
	ReloaderAutoAnnotation = "reloader.stakater.com/auto"
	// LogFormat is the log format to use (json, or empty string for default)
	LogFormat = ""
)
