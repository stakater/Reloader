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
	// ConfigmapUpdateAutoSearchAnnotation is an annotation to detect changes in
	// configmaps searched by annotation
	ConfigmapUpdateAutoSearchAnnotation = "configmap.reloader.stakater.com/auto-by-annotation"
	// SecretUpdateAutoSearchAnnotation is an annotation to detect changes in
	// secrets searched by annotation
	SecretUpdateAutoSearchAnnotation = "secret.reloader.stakater.com/auto-by-annotation"
	// LogFormat is the log format to use (json, or empty string for default)
	LogFormat = ""
)
