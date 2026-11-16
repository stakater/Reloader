package constants

const (
	// DefaultHttpListenAddr is the default listening address for global http server
	DefaultHttpListenAddr = ":9090"

	// ConfigmapEnvVarPostfix is a postfix for configmap envVar
	ConfigmapEnvVarPostfix = "CONFIGMAP"
	// SecretEnvVarPostfix is a postfix for secret envVar
	SecretEnvVarPostfix = "SECRET"
	// SecretProviderClassEnvVarPostfix is a postfix for secretproviderclasspodstatus envVar
	SecretProviderClassEnvVarPostfix = "SECRETPROVIDERCLASS"
	// EnvVarPrefix is a Prefix for environment variable
	EnvVarPrefix = "STAKATER_"

	// ReloaderAnnotationPrefix is a Prefix for all reloader annotations
	ReloaderAnnotationPrefix = "reloader.stakater.com"
	// LastReloadedFromAnnotation is an annotation used to describe the last resource that triggered a reload
	LastReloadedFromAnnotation = "last-reloaded-from"
	// RolloutRestartStateAnnotation is an annotation set on Argo Rollouts using the restart strategy. It records the
	// SHA of every resource that triggered a restart so missed or replayed events can be reconciled after Reloader restarts
	RolloutRestartStateAnnotation = ReloaderAnnotationPrefix + "/rollout-restart-state"

	// 	ReloadStrategyFlag The reload strategy flag name
	ReloadStrategyFlag = "reload-strategy"
	// EnvVarsReloadStrategy instructs Reloader to add container environment variables to facilitate a restart
	EnvVarsReloadStrategy = "env-vars"
	// AnnotationsReloadStrategy instructs Reloader to add pod template annotations to facilitate a restart
	AnnotationsReloadStrategy = "annotations"
	// SecretProviderClassController enables support for SecretProviderClassPodStatus resources
	SecretProviderClassController = "secretproviderclasspodstatuses"
)

// Leadership election related consts
const (
	LockName        string = "stakater-reloader-lock"
	PodNameEnv      string = "POD_NAME"
	PodNamespaceEnv string = "POD_NAMESPACE"
)
