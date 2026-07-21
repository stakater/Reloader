package reload

import (
	corev1 "k8s.io/api/core/v1"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// EventType represents the type of change event.
type EventType string

const (
	// EventTypeCreate indicates a resource was created.
	EventTypeCreate EventType = "create"
	// EventTypeUpdate indicates a resource was updated.
	EventTypeUpdate EventType = "update"
	// EventTypeDelete indicates a resource was deleted.
	EventTypeDelete EventType = "delete"
)

// ResourceChange represents a change event for a ConfigMap or Secret.
type ResourceChange interface {
	IsNil() bool
	GetEventType() EventType
	GetName() string
	GetNamespace() string
	GetAnnotations() map[string]string
	GetResourceType() ResourceType
	ComputeHash(hasher *Hasher) string
}

// ConfigMapChange represents a change event for a ConfigMap.
type ConfigMapChange struct {
	ConfigMap *corev1.ConfigMap
	EventType EventType
}

func (c ConfigMapChange) IsNil() bool                       { return c.ConfigMap == nil }
func (c ConfigMapChange) GetEventType() EventType           { return c.EventType }
func (c ConfigMapChange) GetName() string                   { return c.ConfigMap.Name }
func (c ConfigMapChange) GetNamespace() string              { return c.ConfigMap.Namespace }
func (c ConfigMapChange) GetAnnotations() map[string]string { return c.ConfigMap.Annotations }
func (c ConfigMapChange) GetResourceType() ResourceType     { return ResourceTypeConfigMap }
func (c ConfigMapChange) ComputeHash(h *Hasher) string      { return h.HashConfigMap(c.ConfigMap) }

// SecretChange represents a change event for a Secret.
type SecretChange struct {
	Secret    *corev1.Secret
	EventType EventType
}

func (c SecretChange) IsNil() bool                       { return c.Secret == nil }
func (c SecretChange) GetEventType() EventType           { return c.EventType }
func (c SecretChange) GetName() string                   { return c.Secret.Name }
func (c SecretChange) GetNamespace() string              { return c.Secret.Namespace }
func (c SecretChange) GetAnnotations() map[string]string { return c.Secret.Annotations }
func (c SecretChange) GetResourceType() ResourceType     { return ResourceTypeSecret }
func (c SecretChange) ComputeHash(h *Hasher) string      { return h.HashSecret(c.Secret) }

// SecretProviderClassChange represents a change event derived from a
// SecretProviderClassPodStatus update. Name/Annotations refer to the resolved
// SecretProviderClass; Status carries the SPCPS status used for hashing.
type SecretProviderClassChange struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Status      csiv1.SecretProviderClassPodStatusStatus
	EventType   EventType
}

func (c SecretProviderClassChange) IsNil() bool             { return c.Name == "" }
func (c SecretProviderClassChange) GetEventType() EventType { return c.EventType }
func (c SecretProviderClassChange) GetName() string         { return c.Name }
func (c SecretProviderClassChange) GetNamespace() string    { return c.Namespace }
func (c SecretProviderClassChange) GetAnnotations() map[string]string {
	return c.Annotations
}
func (c SecretProviderClassChange) GetResourceType() ResourceType {
	return ResourceTypeSecretProviderClass
}
func (c SecretProviderClassChange) ComputeHash(h *Hasher) string {
	return h.HashSecretProviderClass(c.Status)
}
