package reload

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/workload"
	corev1 "k8s.io/api/core/v1"
)

// Service orchestrates the reload logic for ConfigMaps and Secrets.
type Service struct {
	cfg      *config.Config
	hasher   *Hasher
	matcher  *Matcher
	strategy Strategy
}

// NewService creates a new reload Service with the given configuration.
func NewService(cfg *config.Config) *Service {
	return &Service{
		cfg:      cfg,
		hasher:   NewHasher(),
		matcher:  NewMatcher(cfg),
		strategy: NewStrategy(cfg),
	}
}

// ConfigMapChange represents a change event for a ConfigMap.
type ConfigMapChange struct {
	ConfigMap *corev1.ConfigMap
	EventType EventType
}

// SecretChange represents a change event for a Secret.
type SecretChange struct {
	Secret    *corev1.Secret
	EventType EventType
}

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

// ReloadDecision contains the result of evaluating whether to reload a workload.
type ReloadDecision struct {
	// Workload is the workload accessor.
	Workload workload.WorkloadAccessor
	// ShouldReload indicates whether the workload should be reloaded.
	ShouldReload bool
	// AutoReload indicates if this is an auto-reload.
	AutoReload bool
	// Reason provides a human-readable explanation.
	Reason string
	// Hash is the computed hash of the resource content.
	Hash string
}

// FilterDecisions returns only decisions where ShouldReload is true.
func FilterDecisions(decisions []ReloadDecision) []ReloadDecision {
	var result []ReloadDecision
	for _, d := range decisions {
		if d.ShouldReload {
			result = append(result, d)
		}
	}
	return result
}

// ProcessConfigMap evaluates all workloads to determine which should be reloaded.
func (s *Service) ProcessConfigMap(change ConfigMapChange, workloads []workload.WorkloadAccessor) []ReloadDecision {
	if change.ConfigMap == nil {
		return nil
	}

	if !s.shouldProcessEvent(change.EventType) {
		return nil
	}

	hash := s.hasher.HashConfigMap(change.ConfigMap)
	if change.EventType == EventTypeDelete {
		hash = s.hasher.EmptyHash()
	}

	return s.processResource(
		change.ConfigMap.Name,
		change.ConfigMap.Namespace,
		change.ConfigMap.Annotations,
		ResourceTypeConfigMap,
		hash,
		workloads,
	)
}

// ProcessSecret evaluates all workloads to determine which should be reloaded.
func (s *Service) ProcessSecret(change SecretChange, workloads []workload.WorkloadAccessor) []ReloadDecision {
	if change.Secret == nil {
		return nil
	}

	if !s.shouldProcessEvent(change.EventType) {
		return nil
	}

	hash := s.hasher.HashSecret(change.Secret)
	if change.EventType == EventTypeDelete {
		hash = s.hasher.EmptyHash()
	}

	return s.processResource(
		change.Secret.Name,
		change.Secret.Namespace,
		change.Secret.Annotations,
		ResourceTypeSecret,
		hash,
		workloads,
	)
}

func (s *Service) processResource(
	resourceName string,
	resourceNamespace string,
	resourceAnnotations map[string]string,
	resourceType ResourceType,
	hash string,
	workloads []workload.WorkloadAccessor,
) []ReloadDecision {
	var decisions []ReloadDecision

	for _, wl := range workloads {
		if wl.GetNamespace() != resourceNamespace {
			continue
		}

		if s.cfg.IsWorkloadIgnored(string(wl.Kind())) {
			continue
		}

		var usesResource bool
		switch resourceType {
		case ResourceTypeConfigMap:
			usesResource = wl.UsesConfigMap(resourceName)
		case ResourceTypeSecret:
			usesResource = wl.UsesSecret(resourceName)
		}

		input := MatchInput{
			ResourceName:        resourceName,
			ResourceNamespace:   resourceNamespace,
			ResourceType:        resourceType,
			ResourceAnnotations: resourceAnnotations,
			WorkloadAnnotations: wl.GetAnnotations(),
			PodAnnotations:      wl.GetPodTemplateAnnotations(),
		}

		matchResult := s.matcher.ShouldReload(input)

		shouldReload := matchResult.ShouldReload
		if matchResult.AutoReload && !usesResource {
			shouldReload = false
		}

		decisions = append(decisions, ReloadDecision{
			Workload:     wl,
			ShouldReload: shouldReload,
			AutoReload:   matchResult.AutoReload,
			Reason:       matchResult.Reason,
			Hash:         hash,
		})
	}

	return decisions
}

func (s *Service) shouldProcessEvent(eventType EventType) bool {
	switch eventType {
	case EventTypeCreate:
		return s.cfg.ReloadOnCreate
	case EventTypeDelete:
		return s.cfg.ReloadOnDelete
	case EventTypeUpdate:
		return true
	default:
		return false
	}
}

// ApplyReload applies the reload strategy to a workload.
func (s *Service) ApplyReload(
	ctx context.Context,
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	container := s.findTargetContainer(wl, resourceName, resourceType, autoReload)

	input := StrategyInput{
		ResourceName:   resourceName,
		ResourceType:   resourceType,
		Namespace:      namespace,
		Hash:           hash,
		Container:      container,
		PodAnnotations: wl.GetPodTemplateAnnotations(),
		AutoReload:     autoReload,
	}

	updated, err := s.strategy.Apply(input)
	if err != nil {
		return false, err
	}

	if updated {
		s.setAttributionAnnotation(wl, resourceName, resourceType, namespace, hash, container)
	}

	return updated, nil
}

func (s *Service) setAttributionAnnotation(
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType ResourceType,
	namespace string,
	hash string,
	container *corev1.Container,
) {
	containerName := ""
	if container != nil {
		containerName = container.Name
	}

	source := ReloadSource{
		Kind:       string(resourceType),
		Name:       resourceName,
		Namespace:  namespace,
		Hash:       hash,
		Containers: []string{containerName},
		ReloadedAt: time.Now().UTC(),
	}

	sourceJSON, err := json.Marshal(source)
	if err != nil {
		return
	}

	wl.SetPodTemplateAnnotation(s.cfg.Annotations.LastReloadedFrom, string(sourceJSON))
}

func (s *Service) findTargetContainer(
	wl workload.WorkloadAccessor,
	resourceName string,
	resourceType ResourceType,
	autoReload bool,
) *corev1.Container {
	containers := wl.GetContainers()
	if len(containers) == 0 {
		return nil
	}

	if !autoReload {
		return &containers[0]
	}

	volumes := wl.GetVolumes()
	initContainers := wl.GetInitContainers()

	volumeName := s.findVolumeUsingResource(volumes, resourceName, resourceType)
	if volumeName != "" {
		container := s.findContainerWithVolumeMount(containers, volumeName)
		if container != nil {
			return container
		}
		container = s.findContainerWithVolumeMount(initContainers, volumeName)
		if container != nil {
			return &containers[0]
		}
	}

	container := s.findContainerWithEnvRef(containers, resourceName, resourceType)
	if container != nil {
		return container
	}

	container = s.findContainerWithEnvRef(initContainers, resourceName, resourceType)
	if container != nil {
		return &containers[0]
	}

	return &containers[0]
}

func (s *Service) findVolumeUsingResource(volumes []corev1.Volume, resourceName string, resourceType ResourceType) string {
	for _, vol := range volumes {
		switch resourceType {
		case ResourceTypeConfigMap:
			if vol.ConfigMap != nil && vol.ConfigMap.Name == resourceName {
				return vol.Name
			}
			if vol.Projected != nil {
				for _, src := range vol.Projected.Sources {
					if src.ConfigMap != nil && src.ConfigMap.Name == resourceName {
						return vol.Name
					}
				}
			}
		case ResourceTypeSecret:
			if vol.Secret != nil && vol.Secret.SecretName == resourceName {
				return vol.Name
			}
			if vol.Projected != nil {
				for _, src := range vol.Projected.Sources {
					if src.Secret != nil && src.Secret.Name == resourceName {
						return vol.Name
					}
				}
			}
		}
	}
	return ""
}

func (s *Service) findContainerWithVolumeMount(containers []corev1.Container, volumeName string) *corev1.Container {
	for i := range containers {
		for _, mount := range containers[i].VolumeMounts {
			if mount.Name == volumeName {
				return &containers[i]
			}
		}
	}
	return nil
}

func (s *Service) findContainerWithEnvRef(containers []corev1.Container, resourceName string, resourceType ResourceType) *corev1.Container {
	for i := range containers {
		for _, env := range containers[i].Env {
			if env.ValueFrom == nil {
				continue
			}
			switch resourceType {
			case ResourceTypeConfigMap:
				if env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == resourceName {
					return &containers[i]
				}
			case ResourceTypeSecret:
				if env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == resourceName {
					return &containers[i]
				}
			}
		}

		for _, envFrom := range containers[i].EnvFrom {
			switch resourceType {
			case ResourceTypeConfigMap:
				if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == resourceName {
					return &containers[i]
				}
			case ResourceTypeSecret:
				if envFrom.SecretRef != nil && envFrom.SecretRef.Name == resourceName {
					return &containers[i]
				}
			}
		}
	}
	return nil
}

// Hasher returns the hasher used by this service.
func (s *Service) Hasher() *Hasher {
	return s.hasher
}
