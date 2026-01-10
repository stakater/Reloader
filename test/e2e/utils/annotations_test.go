package utils

import (
	"testing"
)

func TestBuildConfigMapReloadAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		configMaps []string
		expected   map[string]string
	}{
		{
			name:       "single ConfigMap",
			configMaps: []string{"my-config"},
			expected: map[string]string{
				AnnotationConfigMapReload: "my-config",
			},
		},
		{
			name:       "multiple ConfigMaps",
			configMaps: []string{"config1", "config2", "config3"},
			expected: map[string]string{
				AnnotationConfigMapReload: "config1,config2,config3",
			},
		},
		{
			name:       "empty list",
			configMaps: []string{},
			expected: map[string]string{
				AnnotationConfigMapReload: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildConfigMapReloadAnnotation(tt.configMaps...)
			if len(result) != len(tt.expected) {
				t.Errorf("BuildConfigMapReloadAnnotation() returned %d entries, want %d", len(result), len(tt.expected))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("BuildConfigMapReloadAnnotation()[%q] = %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

func TestBuildSecretReloadAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		secrets  []string
		expected map[string]string
	}{
		{
			name:    "single Secret",
			secrets: []string{"my-secret"},
			expected: map[string]string{
				AnnotationSecretReload: "my-secret",
			},
		},
		{
			name:    "multiple Secrets",
			secrets: []string{"secret1", "secret2"},
			expected: map[string]string{
				AnnotationSecretReload: "secret1,secret2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildSecretReloadAnnotation(tt.secrets...)
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("BuildSecretReloadAnnotation()[%q] = %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

func TestBuildAutoAnnotations(t *testing.T) {
	t.Run("BuildAutoTrueAnnotation", func(t *testing.T) {
		result := BuildAutoTrueAnnotation()
		if result[AnnotationAuto] != AnnotationValueTrue {
			t.Errorf("BuildAutoTrueAnnotation()[%q] = %q, want %q",
				AnnotationAuto, result[AnnotationAuto], AnnotationValueTrue)
		}
	})

	t.Run("BuildAutoFalseAnnotation", func(t *testing.T) {
		result := BuildAutoFalseAnnotation()
		if result[AnnotationAuto] != AnnotationValueFalse {
			t.Errorf("BuildAutoFalseAnnotation()[%q] = %q, want %q",
				AnnotationAuto, result[AnnotationAuto], AnnotationValueFalse)
		}
	})

	t.Run("BuildConfigMapAutoAnnotation", func(t *testing.T) {
		result := BuildConfigMapAutoAnnotation()
		if result[AnnotationConfigMapAuto] != AnnotationValueTrue {
			t.Errorf("BuildConfigMapAutoAnnotation()[%q] = %q, want %q",
				AnnotationConfigMapAuto, result[AnnotationConfigMapAuto], AnnotationValueTrue)
		}
	})

	t.Run("BuildSecretAutoAnnotation", func(t *testing.T) {
		result := BuildSecretAutoAnnotation()
		if result[AnnotationSecretAuto] != AnnotationValueTrue {
			t.Errorf("BuildSecretAutoAnnotation()[%q] = %q, want %q",
				AnnotationSecretAuto, result[AnnotationSecretAuto], AnnotationValueTrue)
		}
	})
}

func TestBuildSearchMatchAnnotations(t *testing.T) {
	t.Run("BuildSearchAnnotation", func(t *testing.T) {
		result := BuildSearchAnnotation()
		if result[AnnotationSearch] != AnnotationValueTrue {
			t.Errorf("BuildSearchAnnotation()[%q] = %q, want %q",
				AnnotationSearch, result[AnnotationSearch], AnnotationValueTrue)
		}
	})

	t.Run("BuildMatchAnnotation", func(t *testing.T) {
		result := BuildMatchAnnotation()
		if result[AnnotationMatch] != AnnotationValueTrue {
			t.Errorf("BuildMatchAnnotation()[%q] = %q, want %q",
				AnnotationMatch, result[AnnotationMatch], AnnotationValueTrue)
		}
	})
}

func TestBuildIgnoreAnnotation(t *testing.T) {
	result := BuildIgnoreAnnotation()
	if result[AnnotationIgnore] != AnnotationValueTrue {
		t.Errorf("BuildIgnoreAnnotation()[%q] = %q, want %q",
			AnnotationIgnore, result[AnnotationIgnore], AnnotationValueTrue)
	}
}

func TestBuildRolloutRestartStrategyAnnotation(t *testing.T) {
	result := BuildRolloutRestartStrategyAnnotation()
	if result[AnnotationRolloutStrategy] != AnnotationValueRestart {
		t.Errorf("BuildRolloutRestartStrategyAnnotation()[%q] = %q, want %q",
			AnnotationRolloutStrategy, result[AnnotationRolloutStrategy], AnnotationValueRestart)
	}
}

func TestBuildExcludeAnnotations(t *testing.T) {
	t.Run("BuildConfigMapExcludeAnnotation single", func(t *testing.T) {
		result := BuildConfigMapExcludeAnnotation("excluded-cm")
		if result[AnnotationConfigMapExclude] != "excluded-cm" {
			t.Errorf("BuildConfigMapExcludeAnnotation()[%q] = %q, want %q",
				AnnotationConfigMapExclude, result[AnnotationConfigMapExclude], "excluded-cm")
		}
	})

	t.Run("BuildConfigMapExcludeAnnotation multiple", func(t *testing.T) {
		result := BuildConfigMapExcludeAnnotation("cm1", "cm2", "cm3")
		expected := "cm1,cm2,cm3"
		if result[AnnotationConfigMapExclude] != expected {
			t.Errorf("BuildConfigMapExcludeAnnotation()[%q] = %q, want %q",
				AnnotationConfigMapExclude, result[AnnotationConfigMapExclude], expected)
		}
	})

	t.Run("BuildSecretExcludeAnnotation single", func(t *testing.T) {
		result := BuildSecretExcludeAnnotation("excluded-secret")
		if result[AnnotationSecretExclude] != "excluded-secret" {
			t.Errorf("BuildSecretExcludeAnnotation()[%q] = %q, want %q",
				AnnotationSecretExclude, result[AnnotationSecretExclude], "excluded-secret")
		}
	})

	t.Run("BuildSecretExcludeAnnotation multiple", func(t *testing.T) {
		result := BuildSecretExcludeAnnotation("s1", "s2")
		expected := "s1,s2"
		if result[AnnotationSecretExclude] != expected {
			t.Errorf("BuildSecretExcludeAnnotation()[%q] = %q, want %q",
				AnnotationSecretExclude, result[AnnotationSecretExclude], expected)
		}
	})
}

func TestBuildPausePeriodAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		expected string
	}{
		{
			name:     "10 seconds",
			duration: "10s",
			expected: "10s",
		},
		{
			name:     "1 minute",
			duration: "1m",
			expected: "1m",
		},
		{
			name:     "30 minutes",
			duration: "30m",
			expected: "30m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPausePeriodAnnotation(tt.duration)
			if result[AnnotationDeploymentPausePeriod] != tt.expected {
				t.Errorf("BuildPausePeriodAnnotation(%q)[%q] = %q, want %q",
					tt.duration, AnnotationDeploymentPausePeriod,
					result[AnnotationDeploymentPausePeriod], tt.expected)
			}
		})
	}
}

func TestJoinNames(t *testing.T) {
	tests := []struct {
		name     string
		names    []string
		expected string
	}{
		{
			name:     "empty slice",
			names:    []string{},
			expected: "",
		},
		{
			name:     "single name",
			names:    []string{"one"},
			expected: "one",
		},
		{
			name:     "two names",
			names:    []string{"one", "two"},
			expected: "one,two",
		},
		{
			name:     "three names",
			names:    []string{"alpha", "beta", "gamma"},
			expected: "alpha,beta,gamma",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinNames(tt.names)
			if result != tt.expected {
				t.Errorf("joinNames(%v) = %q, want %q", tt.names, result, tt.expected)
			}
		})
	}
}

func TestAnnotationConstants(t *testing.T) {
	// Verify annotation constants have expected values
	// This ensures we don't accidentally change the annotation keys
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"AnnotationLastReloadedFrom", AnnotationLastReloadedFrom, "reloader.stakater.com/last-reloaded-from"},
		{"AnnotationConfigMapReload", AnnotationConfigMapReload, "configmap.reloader.stakater.com/reload"},
		{"AnnotationSecretReload", AnnotationSecretReload, "secret.reloader.stakater.com/reload"},
		{"AnnotationAuto", AnnotationAuto, "reloader.stakater.com/auto"},
		{"AnnotationConfigMapAuto", AnnotationConfigMapAuto, "configmap.reloader.stakater.com/auto"},
		{"AnnotationSecretAuto", AnnotationSecretAuto, "secret.reloader.stakater.com/auto"},
		{"AnnotationConfigMapExclude", AnnotationConfigMapExclude, "configmaps.exclude.reloader.stakater.com/reload"},
		{"AnnotationSecretExclude", AnnotationSecretExclude, "secrets.exclude.reloader.stakater.com/reload"},
		{"AnnotationSearch", AnnotationSearch, "reloader.stakater.com/search"},
		{"AnnotationMatch", AnnotationMatch, "reloader.stakater.com/match"},
		{"AnnotationIgnore", AnnotationIgnore, "reloader.stakater.com/ignore"},
		{"AnnotationDeploymentPausePeriod", AnnotationDeploymentPausePeriod, "deployment.reloader.stakater.com/pause-period"},
		{"AnnotationDeploymentPausedAt", AnnotationDeploymentPausedAt, "deployment.reloader.stakater.com/paused-at"},
		{"AnnotationRolloutStrategy", AnnotationRolloutStrategy, "reloader.stakater.com/rollout-strategy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestAnnotationValues(t *testing.T) {
	// Verify annotation value constants
	if AnnotationValueTrue != "true" {
		t.Errorf("AnnotationValueTrue = %q, want \"true\"", AnnotationValueTrue)
	}
	if AnnotationValueFalse != "false" {
		t.Errorf("AnnotationValueFalse = %q, want \"false\"", AnnotationValueFalse)
	}
	if AnnotationValueRestart != "restart" {
		t.Errorf("AnnotationValueRestart = %q, want \"restart\"", AnnotationValueRestart)
	}
}
