package common

import (
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	v1 "k8s.io/api/core/v1"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// Config contains rolling upgrade configuration parameters
type Config struct {
	Namespace           string
	ResourceName        string
	ResourceAnnotations map[string]string
	Annotation          string
	TypedAutoAnnotation string
	SHAValue            string
	Type                string
	Labels              map[string]string
}

// GetConfigmapConfig provides utility config for configmap
func GetConfigmapConfig(configmap *v1.ConfigMap) Config {
	return Config{
		Namespace:           configmap.Namespace,
		ResourceName:        configmap.Name,
		ResourceAnnotations: configmap.Annotations,
		Annotation:          options.ConfigmapUpdateOnChangeAnnotation,
		TypedAutoAnnotation: options.ConfigmapReloaderAutoAnnotation,
		SHAValue:            util.GetSHAfromConfigmap(configmap),
		Type:                constants.ConfigmapEnvVarPostfix,
		Labels:              configmap.Labels,
	}
}

// GetSecretConfig provides utility config for secret
func GetSecretConfig(secret *v1.Secret) Config {
	return Config{
		Namespace:           secret.Namespace,
		ResourceName:        secret.Name,
		ResourceAnnotations: secret.Annotations,
		Annotation:          options.SecretUpdateOnChangeAnnotation,
		TypedAutoAnnotation: options.SecretReloaderAutoAnnotation,
		SHAValue:            util.GetSHAfromSecret(secret.Data),
		Type:                constants.SecretEnvVarPostfix,
		Labels:              secret.Labels,
	}
}

func GetSecretProviderClassPodStatusConfig(podStatus *csiv1.SecretProviderClassPodStatus) Config {
	// As csi injects SecretProviderClass, we will create config for it instead of SecretProviderClassPodStatus
	// ResourceAnnotations will be retrieved during PerformAction call
	return Config{
		Namespace:           podStatus.Namespace,
		ResourceName:        podStatus.Status.SecretProviderClassName,
		Annotation:          options.SecretProviderClassUpdateOnChangeAnnotation,
		TypedAutoAnnotation: options.SecretProviderClassReloaderAutoAnnotation,
		SHAValue:            util.GetSHAfromSecretProviderClassPodStatus(podStatus.Status),
		Type:                constants.SecretProviderClassEnvVarPostfix,
	}
}
