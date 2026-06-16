package reload

import (
	corev1 "k8s.io/api/core/v1"
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
