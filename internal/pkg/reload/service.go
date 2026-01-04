package reload

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/workload"
	corev1 "k8s.io/api/core/v1"
)

// Service orchestrates the reload logic for ConfigMaps and Secrets.
type Service struct {
	cfg      *config.Config
	log      logr.Logger
	hasher   *Hasher
	matcher  *Matcher
	strategy Strategy
}

// NewService creates a new reload Service with the given configuration.
func NewService(cfg *config.Config, log logr.Logger) *Service {
	return &Service{
		cfg:      cfg,
		log:      log,
		hasher:   NewHasher(),
		matcher:  NewMatcher(cfg),
		strategy: NewStrategy(cfg),
	}
}

// Process evaluates all workloads to determine which should be reloaded.
func (s *Service) Process(change ResourceChange, workloads []workload.Workload) []ReloadDecision {
	if change.IsNil() {
		return nil
	}

	if !s.shouldProcessEvent(change.GetEventType()) {
		return nil
	}

	hash := change.ComputeHash(s.hasher)
	if change.GetEventType() == EventTypeDelete {
		hash = s.hasher.EmptyHash()
	}

	return s.processResource(
		change.GetName(),
		change.GetNamespace(),
		change.GetAnnotations(),
		change.GetResourceType(),
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
	workloads []workload.Workload,
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

		decisions = append(
			decisions, ReloadDecision{
				Workload:     wl,
				ShouldReload: shouldReload,
				AutoReload:   matchResult.AutoReload,
				Reason:       matchResult.Reason,
				Hash:         hash,
			},
		)
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
	wl workload.Workload,
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
		// Attribution annotation is informational; log errors but don't fail reloads
		if err := s.setAttributionAnnotation(wl, resourceName, resourceType, namespace, hash, container); err != nil {
			s.log.V(1).Info("failed to set attribution annotation", "error", err, "workload", wl.GetName())
		}
	}

	return updated, nil
}

func (s *Service) setAttributionAnnotation(
	wl workload.Workload,
	resourceName string,
	resourceType ResourceType,
	namespace string,
	hash string,
	container *corev1.Container,
) error {
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
		return fmt.Errorf("failed to marshal reload source: %w", err)
	}

	wl.SetPodTemplateAnnotation(s.cfg.Annotations.LastReloadedFrom, string(sourceJSON))
	return nil
}

func (s *Service) findTargetContainer(
	wl workload.Workload,
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
