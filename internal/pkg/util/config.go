package util

import (
	"github.com/stakater/Reloader/internal/pkg/constants"
	"k8s.io/api/core/v1"
)

//Config contains rolling upgrade configuration parameters
type Config struct {
	Namespace    string
	ResourceName string
	Annotation   string
	SHAValue     string
	Type         string
}

// GetConfigmapConfig provides utility config for configmap
func GetConfigmapConfig(configmap *v1.ConfigMap) Config {
	return Config{
		Namespace:    configmap.Namespace,
		ResourceName: configmap.Name,
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
		SHAValue:     GetSHAfromConfigmap(configmap.Data),
		Type:         constants.ConfigmapEnvVarPostfix,
	}
}

// GetSecretConfig provides utility config for secret
func GetSecretConfig(secret *v1.Secret) Config {
	return Config{
		Namespace:    secret.Namespace,
		ResourceName: secret.Name,
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
		SHAValue:     GetSHAfromSecret(secret.Data),
		Type:         constants.SecretEnvVarPostfix,
	}
}
