package reload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

const (
	// EnvVarPrefix is the prefix for environment variables added by Reloader.
	EnvVarPrefix = "STAKATER_"
	// ConfigmapEnvVarPostfix is the postfix for ConfigMap environment variables.
	ConfigmapEnvVarPostfix = "CONFIGMAP"
	// SecretEnvVarPostfix is the postfix for Secret environment variables.
	SecretEnvVarPostfix = "SECRET"
)

// Strategy defines how workload restarts are triggered.
type Strategy interface {
	// Apply applies the reload strategy to the pod spec.
	// Returns true if changes were made, false otherwise.
	Apply(input StrategyInput) (bool, error)

	// Name returns the strategy name for logging purposes.
	Name() string
}

// StrategyInput contains the information needed to apply a reload strategy.
type StrategyInput struct {
	// ResourceName is the name of the ConfigMap or Secret that changed.
	ResourceName string
	// ResourceType is the type of resource (configmap or secret).
	ResourceType ResourceType
	// Namespace is the namespace of the resource.
	Namespace string
	// Hash is the SHA hash of the resource content.
	Hash string
	// Container is the container to target for env var injection.
	// If nil, the first container is used.
	Container *corev1.Container
	// PodAnnotations is the pod template annotations map (for annotation strategy).
	PodAnnotations map[string]string
	// AutoReload indicates if this is an auto-reload (affects container selection).
	AutoReload bool
}

// ReloadSource contains metadata about what triggered a reload.
// This is stored in the annotation when using annotation strategy.
type ReloadSource struct {
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	Namespace  string    `json:"namespace"`
	Hash       string    `json:"hash"`
	Containers []string  `json:"containers"`
	ReloadedAt time.Time `json:"reloadedAt"`
}

// EnvVarStrategy triggers reloads by adding/updating environment variables.
// This is the default strategy and is GitOps-friendly.
type EnvVarStrategy struct{}

// NewEnvVarStrategy creates a new EnvVarStrategy.
func NewEnvVarStrategy() *EnvVarStrategy {
	return &EnvVarStrategy{}
}

// Name returns the strategy name.
func (s *EnvVarStrategy) Name() string {
	return string(config.ReloadStrategyEnvVars)
}

// Apply adds or updates an environment variable to trigger a restart.
func (s *EnvVarStrategy) Apply(input StrategyInput) (bool, error) {
	if input.Container == nil {
		return false, fmt.Errorf("container is required for env-var strategy")
	}

	envVarName := s.envVarName(input.ResourceName, input.ResourceType)

	// Check if env var already exists
	for i := range input.Container.Env {
		if input.Container.Env[i].Name == envVarName {
			if input.Container.Env[i].Value == input.Hash {
				// Already up to date
				return false, nil
			}
			// Update existing
			input.Container.Env[i].Value = input.Hash
			return true, nil
		}
	}

	// Add new env var
	input.Container.Env = append(input.Container.Env, corev1.EnvVar{
		Name:  envVarName,
		Value: input.Hash,
	})

	return true, nil
}

// envVarName generates the environment variable name for a resource.
func (s *EnvVarStrategy) envVarName(resourceName string, resourceType ResourceType) string {
	var postfix string
	switch resourceType {
	case ResourceTypeConfigMap:
		postfix = ConfigmapEnvVarPostfix
	case ResourceTypeSecret:
		postfix = SecretEnvVarPostfix
	}
	return EnvVarPrefix + convertToEnvVarName(resourceName) + "_" + postfix
}

// convertToEnvVarName converts a string to a valid environment variable name.
// Invalid characters are replaced with underscores, and the result is uppercased.
func convertToEnvVarName(text string) string {
	var buffer bytes.Buffer
	upper := strings.ToUpper(text)
	lastCharValid := false

	for i := 0; i < len(upper); i++ {
		ch := upper[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			buffer.WriteByte(ch)
			lastCharValid = true
		} else {
			if lastCharValid {
				buffer.WriteByte('_')
			}
			lastCharValid = false
		}
	}

	return buffer.String()
}

// AnnotationStrategy triggers reloads by adding/updating pod template annotations.
type AnnotationStrategy struct {
	cfg *config.Config
}

// NewAnnotationStrategy creates a new AnnotationStrategy.
func NewAnnotationStrategy(cfg *config.Config) *AnnotationStrategy {
	return &AnnotationStrategy{cfg: cfg}
}

// Name returns the strategy name.
func (s *AnnotationStrategy) Name() string {
	return string(config.ReloadStrategyAnnotations)
}

// Apply adds or updates a pod annotation to trigger a restart.
func (s *AnnotationStrategy) Apply(input StrategyInput) (bool, error) {
	if input.PodAnnotations == nil {
		return false, fmt.Errorf("pod annotations map is required for annotation strategy")
	}

	containerName := ""
	if input.Container != nil {
		containerName = input.Container.Name
	}

	// Create reload source metadata
	source := ReloadSource{
		Kind:       string(input.ResourceType),
		Name:       input.ResourceName,
		Namespace:  input.Namespace,
		Hash:       input.Hash,
		Containers: []string{containerName},
		ReloadedAt: time.Now().UTC(),
	}

	sourceJSON, err := json.Marshal(source)
	if err != nil {
		return false, fmt.Errorf("failed to marshal reload source: %w", err)
	}

	annotationKey := s.cfg.Annotations.LastReloadedFrom
	existingValue := input.PodAnnotations[annotationKey]

	if existingValue == string(sourceJSON) {
		// Already up to date
		return false, nil
	}

	input.PodAnnotations[annotationKey] = string(sourceJSON)
	return true, nil
}

// NewStrategy creates a Strategy based on the configuration.
func NewStrategy(cfg *config.Config) Strategy {
	switch cfg.ReloadStrategy {
	case config.ReloadStrategyAnnotations:
		return NewAnnotationStrategy(cfg)
	default:
		return NewEnvVarStrategy()
	}
}
