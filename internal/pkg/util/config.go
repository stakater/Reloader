package util

import (
	"k8s.io/api/core/v1"
	"github.com/stakater/Reloader/internal/pkg/constants"
)

//Config contains rolling upgrade configuration parameters
type Config struct {
	Namespace    string
	ResourceName string
	Annotation   string
	SHAValue     string
}

func GetConfigmapConfig(configmap *v1.ConfigMap) Config {
	return Config{
		Namespace:    configmap.Namespace,
		ResourceName: configmap.Name,
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
		SHAValue:     GetSHAfromConfigmap(configmap.Data),
	}
}

func GetSecretConfig(secret *v1.Secret) Config {
	return Config{
		Namespace:    secret.Namespace,
		ResourceName: secret.Name,
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
		SHAValue:     GetSHAfromSecret(secret.Data),
	}
}