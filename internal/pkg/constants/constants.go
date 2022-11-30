package constants

const (
	// ConfigmapEnvVarPostfix is a postfix for configmap envVar
	ConfigmapEnvVarPostfix = "CONFIGMAP"
	// SecretEnvVarPostfix is a postfix for secret envVar
	SecretEnvVarPostfix = "SECRET"
	// EnvVarPrefix is a Prefix for environment variable
	EnvVarPrefix = "STAKATER_"

	// ReloaderAnnotationPrefix is a Prefix for all reloader annotations
	ReloaderAnnotationPrefix = "reloader.stakater.com"
	// LastReloadedFromAnnotation is an annotation used to describe the last resource that triggered a reload
	LastReloadedFromAnnotation = "last-reloaded-from"

	// 	ReloadStrategyFlag The reload strategy flag name
	ReloadStrategyFlag = "reload-strategy"
	// EnvVarsReloadStrategy instructs Reloader to add container environment variables to facilitate a restart
	EnvVarsReloadStrategy = "env-vars"
	// AnnotationsReloadStrategy instructs Reloader to add pod template annotations to facilitate a restart
	AnnotationsReloadStrategy = "annotations"
)

// Leadership election related consts
const (
	LockName        string = "stakater-reloader-lock"
	PodNameEnv      string = "POD_NAME"
	PodNamespaceEnv string = "POD_NAMESPACE"
)
