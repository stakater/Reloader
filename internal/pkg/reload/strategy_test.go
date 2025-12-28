package reload

import (
	"encoding/json"
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

func TestEnvVarStrategy_Apply(t *testing.T) {
	strategy := NewEnvVarStrategy()

	t.Run("adds new env var", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test-container",
			Env:  []corev1.EnvVar{},
		}

		input := StrategyInput{
			ResourceName: "my-config",
			ResourceType: ResourceTypeConfigMap,
			Namespace:    "default",
			Hash:         "abc123",
			Container:    container,
		}

		changed, err := strategy.Apply(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected changed=true for new env var")
		}

		// Verify env var was added
		found := false
		for _, env := range container.Env {
			if env.Name == "STAKATER_MY_CONFIG_CONFIGMAP" && env.Value == "abc123" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected env var STAKATER_MY_CONFIG_CONFIGMAP=abc123, got %+v", container.Env)
		}
	})

	t.Run("updates existing env var", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test-container",
			Env: []corev1.EnvVar{
				{Name: "STAKATER_MY_CONFIG_CONFIGMAP", Value: "old-hash"},
			},
		}

		input := StrategyInput{
			ResourceName: "my-config",
			ResourceType: ResourceTypeConfigMap,
			Namespace:    "default",
			Hash:         "new-hash",
			Container:    container,
		}

		changed, err := strategy.Apply(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected changed=true for updated env var")
		}

		// Verify env var was updated
		if container.Env[0].Value != "new-hash" {
			t.Errorf("expected env var value=new-hash, got %s", container.Env[0].Value)
		}
	})

	t.Run("no change when hash is same", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test-container",
			Env: []corev1.EnvVar{
				{Name: "STAKATER_MY_CONFIG_CONFIGMAP", Value: "same-hash"},
			},
		}

		input := StrategyInput{
			ResourceName: "my-config",
			ResourceType: ResourceTypeConfigMap,
			Namespace:    "default",
			Hash:         "same-hash",
			Container:    container,
		}

		changed, err := strategy.Apply(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if changed {
			t.Error("expected changed=false when hash is unchanged")
		}
	})

	t.Run("error when container is nil", func(t *testing.T) {
		input := StrategyInput{
			ResourceName: "my-config",
			ResourceType: ResourceTypeConfigMap,
			Namespace:    "default",
			Hash:         "abc123",
			Container:    nil,
		}

		_, err := strategy.Apply(input)
		if err == nil {
			t.Error("expected error for nil container")
		}
	})

	t.Run("secret env var has correct postfix", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test-container",
			Env:  []corev1.EnvVar{},
		}

		input := StrategyInput{
			ResourceName: "my-secret",
			ResourceType: ResourceTypeSecret,
			Namespace:    "default",
			Hash:         "abc123",
			Container:    container,
		}

		changed, err := strategy.Apply(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected changed=true")
		}

		// Verify env var name has SECRET postfix
		found := false
		for _, env := range container.Env {
			if env.Name == "STAKATER_MY_SECRET_SECRET" && env.Value == "abc123" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected env var STAKATER_MY_SECRET_SECRET=abc123, got %+v", container.Env)
		}
	})
}

func TestEnvVarStrategy_EnvVarName(t *testing.T) {
	strategy := NewEnvVarStrategy()

	tests := []struct {
		resourceName string
		resourceType ResourceType
		expected     string
	}{
		{"my-config", ResourceTypeConfigMap, "STAKATER_MY_CONFIG_CONFIGMAP"},
		{"my-secret", ResourceTypeSecret, "STAKATER_MY_SECRET_SECRET"},
		{"app-config-v2", ResourceTypeConfigMap, "STAKATER_APP_CONFIG_V2_CONFIGMAP"},
		{"my.dotted.config", ResourceTypeConfigMap, "STAKATER_MY_DOTTED_CONFIG_CONFIGMAP"},
		{"MyMixedCase", ResourceTypeConfigMap, "STAKATER_MYMIXEDCASE_CONFIGMAP"},
		{"config-with-123-numbers", ResourceTypeConfigMap, "STAKATER_CONFIG_WITH_123_NUMBERS_CONFIGMAP"},
	}

	for _, tt := range tests {
		t.Run(tt.resourceName, func(t *testing.T) {
			got := strategy.envVarName(tt.resourceName, tt.resourceType)
			if got != tt.expected {
				t.Errorf("envVarName(%q, %q) = %q, want %q",
					tt.resourceName, tt.resourceType, got, tt.expected)
			}
		})
	}
}

func TestConvertToEnvVarName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-config", "MY_CONFIG"},
		{"my.config", "MY_CONFIG"},
		{"my_config", "MY_CONFIG"},
		{"MY-CONFIG", "MY_CONFIG"},
		{"config123", "CONFIG123"},
		{"123config", "123CONFIG"},
		{"my--config", "MY_CONFIG"},
		{"my..config", "MY_CONFIG"},
		{"", ""},
		{"-leading-dash", "LEADING_DASH"},
		{"trailing-dash-", "TRAILING_DASH_"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := convertToEnvVarName(tt.input)
			if got != tt.expected {
				t.Errorf("convertToEnvVarName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAnnotationStrategy_Apply(t *testing.T) {
	cfg := config.NewDefault()
	strategy := NewAnnotationStrategy(cfg)

	t.Run("adds new annotation", func(t *testing.T) {
		annotations := make(map[string]string)
		container := &corev1.Container{Name: "test-container"}

		input := StrategyInput{
			ResourceName:   "my-config",
			ResourceType:   ResourceTypeConfigMap,
			Namespace:      "default",
			Hash:           "abc123",
			Container:      container,
			PodAnnotations: annotations,
		}

		changed, err := strategy.Apply(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected changed=true for new annotation")
		}

		// Verify annotation was added
		annotationValue := annotations[cfg.Annotations.LastReloadedFrom]
		if annotationValue == "" {
			t.Error("expected annotation to be set")
		}

		// Verify annotation content
		var source ReloadSource
		if err := json.Unmarshal([]byte(annotationValue), &source); err != nil {
			t.Fatalf("failed to unmarshal annotation: %v", err)
		}
		if source.Kind != string(ResourceTypeConfigMap) {
			t.Errorf("expected kind=%s, got %s", ResourceTypeConfigMap, source.Kind)
		}
		if source.Name != "my-config" {
			t.Errorf("expected name=my-config, got %s", source.Name)
		}
		if source.Hash != "abc123" {
			t.Errorf("expected hash=abc123, got %s", source.Hash)
		}
	})

	t.Run("error when annotations map is nil", func(t *testing.T) {
		input := StrategyInput{
			ResourceName:   "my-config",
			ResourceType:   ResourceTypeConfigMap,
			Namespace:      "default",
			Hash:           "abc123",
			PodAnnotations: nil,
		}

		_, err := strategy.Apply(input)
		if err == nil {
			t.Error("expected error for nil annotations map")
		}
	})
}

func TestNewStrategy(t *testing.T) {
	t.Run("default strategy is env-vars", func(t *testing.T) {
		cfg := config.NewDefault()
		strategy := NewStrategy(cfg)

		if strategy.Name() != string(config.ReloadStrategyEnvVars) {
			t.Errorf("expected env-vars strategy, got %s", strategy.Name())
		}
	})

	t.Run("annotations strategy when configured", func(t *testing.T) {
		cfg := config.NewDefault()
		cfg.ReloadStrategy = config.ReloadStrategyAnnotations
		strategy := NewStrategy(cfg)

		if strategy.Name() != string(config.ReloadStrategyAnnotations) {
			t.Errorf("expected annotations strategy, got %s", strategy.Name())
		}
	})
}
