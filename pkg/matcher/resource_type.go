package matcher

// ResourceType represents the type of Kubernetes resource.
type ResourceType string

const (
	// ResourceTypeConfigMap represents a ConfigMap resource.
	ResourceTypeConfigMap ResourceType = "configmap"
	// ResourceTypeSecret represents a Secret resource.
	ResourceTypeSecret ResourceType = "secret"
	// ResourceTypeSecretProviderClass represents a CSI SecretProviderClass resource.
	ResourceTypeSecretProviderClass ResourceType = "secretproviderclass"
)

// Kind returns the capitalized Kubernetes Kind (e.g., "ConfigMap", "Secret").
func (r ResourceType) Kind() string {
	switch r {
	case ResourceTypeConfigMap:
		return "ConfigMap"
	case ResourceTypeSecret:
		return "Secret"
	case ResourceTypeSecretProviderClass:
		return "SecretProviderClass"
	default:
		return string(r)
	}
}
