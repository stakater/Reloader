package reload

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
)

func TestMatcher_ShouldReload(t *testing.T) {
	defaultCfg := config.NewDefault()
	matcher := NewMatcher(defaultCfg)

	tests := []struct {
		name           string
		input          MatchInput
		wantReload     bool
		wantAutoReload bool
		description    string
	}{
		// Ignore annotation tests
		{
			name: "ignore annotation on resource skips reload",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: map[string]string{"reloader.stakater.com/ignore": "true"},
				WorkloadAnnotations: map[string]string{"reloader.stakater.com/auto": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "Resources with ignore annotation should never trigger reload",
		},
		{
			name: "ignore annotation false allows reload",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: map[string]string{"reloader.stakater.com/ignore": "false"},
				WorkloadAnnotations: map[string]string{"reloader.stakater.com/auto": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     true,
			wantAutoReload: true,
			description:    "Resources with ignore=false should allow reload",
		},

		// Exclude annotation tests
		{
			name: "exclude annotation skips reload",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"reloader.stakater.com/auto":                      "true",
					"configmaps.exclude.reloader.stakater.com/reload": "my-config",
				},
				PodAnnotations: nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "Excluded ConfigMaps should not trigger reload",
		},
		{
			name: "exclude annotation with multiple values",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"reloader.stakater.com/auto":                      "true",
					"configmaps.exclude.reloader.stakater.com/reload": "other-config,my-config,another-config",
				},
				PodAnnotations: nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "ConfigMaps in comma-separated exclude list should not trigger reload",
		},

		// BUG FIX: Explicit annotation checked BEFORE auto
		{
			name: "explicit reload annotation with auto enabled - should reload",
			input: MatchInput{
				ResourceName:        "external-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"reloader.stakater.com/auto":             "true",
					"configmap.reloader.stakater.com/reload": "external-config",
				},
				PodAnnotations: nil,
			},
			wantReload:     true,
			wantAutoReload: false, // Explicit, not auto
			description:    "BUG FIX: Explicit reload annotation should work even when auto is enabled",
		},
		{
			name: "explicit reload annotation matches pattern - should reload",
			input: MatchInput{
				ResourceName:        "app-config-v2",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"configmap.reloader.stakater.com/reload": "app-config-.*",
				},
				PodAnnotations: nil,
			},
			wantReload:     true,
			wantAutoReload: false,
			description:    "Regex pattern in reload annotation should match",
		},
		{
			name: "explicit reload annotation does not match - should not reload",
			input: MatchInput{
				ResourceName:        "other-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"configmap.reloader.stakater.com/reload": "app-config",
				},
				PodAnnotations: nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "ConfigMaps not in reload list should not trigger reload",
		},

		// Auto annotation tests
		{
			name: "auto annotation on workload triggers reload",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{"reloader.stakater.com/auto": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     true,
			wantAutoReload: true,
			description:    "Auto annotation on workload should trigger reload",
		},
		{
			name: "auto annotation on pod template triggers reload",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: nil,
				PodAnnotations:      map[string]string{"reloader.stakater.com/auto": "true"},
			},
			wantReload:     true,
			wantAutoReload: true,
			description:    "Auto annotation on pod template should trigger reload",
		},
		{
			name: "configmap-specific auto annotation",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{"configmap.reloader.stakater.com/auto": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     true,
			wantAutoReload: true,
			description:    "ConfigMap-specific auto annotation should trigger reload",
		},
		{
			name: "secret-specific auto annotation for secret",
			input: MatchInput{
				ResourceName:        "my-secret",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeSecret,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{"secret.reloader.stakater.com/auto": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     true,
			wantAutoReload: true,
			description:    "Secret-specific auto annotation should trigger reload for secrets",
		},
		{
			name: "configmap-specific auto annotation does not match secret",
			input: MatchInput{
				ResourceName:        "my-secret",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeSecret,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{"configmap.reloader.stakater.com/auto": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "ConfigMap-specific auto annotation should not match secrets",
		},

		// Search/Match annotation tests
		{
			name: "search annotation with matching resource",
			input: MatchInput{
				ResourceName:        "app-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: map[string]string{"reloader.stakater.com/match": "true"},
				WorkloadAnnotations: map[string]string{"reloader.stakater.com/search": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     true,
			wantAutoReload: true, // Search mode is an auto-discovery mechanism
			description:    "Search annotation with matching resource should trigger reload",
		},
		{
			name: "search annotation without matching resource",
			input: MatchInput{
				ResourceName:        "app-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{"reloader.stakater.com/search": "true"},
				PodAnnotations:      nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "Search annotation without matching resource should not trigger reload",
		},

		// No annotations - should not reload
		{
			name: "no annotations does not trigger reload",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: nil,
				PodAnnotations:      nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "Without any annotations, should not trigger reload",
		},

		// Secret tests
		{
			name: "secret reload annotation",
			input: MatchInput{
				ResourceName:        "my-secret",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeSecret,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"secret.reloader.stakater.com/reload": "my-secret",
				},
				PodAnnotations: nil,
			},
			wantReload:     true,
			wantAutoReload: false,
			description:    "Secret reload annotation should trigger reload",
		},
		{
			name: "secret exclude annotation",
			input: MatchInput{
				ResourceName:        "my-secret",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeSecret,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"reloader.stakater.com/auto":                   "true",
					"secrets.exclude.reloader.stakater.com/reload": "my-secret",
				},
				PodAnnotations: nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "Secret exclude annotation should prevent reload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.ShouldReload(tt.input)

			if result.ShouldReload != tt.wantReload {
				t.Errorf("ShouldReload = %v, want %v (%s)", result.ShouldReload, tt.wantReload, tt.description)
			}

			if result.AutoReload != tt.wantAutoReload {
				t.Errorf("AutoReload = %v, want %v (%s)", result.AutoReload, tt.wantAutoReload, tt.description)
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}

func TestMatcher_ShouldReload_AutoReloadAll(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true
	matcher := NewMatcher(cfg)

	tests := []struct {
		name           string
		input          MatchInput
		wantReload     bool
		wantAutoReload bool
		description    string
	}{
		{
			name: "auto-reload-all triggers reload",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: nil,
				PodAnnotations:      nil,
			},
			wantReload:     true,
			wantAutoReload: true,
			description:    "With auto-reload-all enabled, all ConfigMaps should trigger reload",
		},
		{
			name: "auto-reload-all respects ignore annotation",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: map[string]string{"reloader.stakater.com/ignore": "true"},
				WorkloadAnnotations: nil,
				PodAnnotations:      nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "Even with auto-reload-all, ignore annotation should be respected",
		},
		{
			name: "auto-reload-all respects exclude annotation",
			input: MatchInput{
				ResourceName:        "my-config",
				ResourceNamespace:   "default",
				ResourceType:        ResourceTypeConfigMap,
				ResourceAnnotations: nil,
				WorkloadAnnotations: map[string]string{
					"configmaps.exclude.reloader.stakater.com/reload": "my-config",
				},
				PodAnnotations: nil,
			},
			wantReload:     false,
			wantAutoReload: false,
			description:    "Even with auto-reload-all, exclude annotation should be respected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.ShouldReload(tt.input)

			if result.ShouldReload != tt.wantReload {
				t.Errorf("ShouldReload = %v, want %v (%s)", result.ShouldReload, tt.wantReload, tt.description)
			}

			if result.AutoReload != tt.wantAutoReload {
				t.Errorf("AutoReload = %v, want %v (%s)", result.AutoReload, tt.wantAutoReload, tt.description)
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}

// TestMatcher_BugFix_AutoDoesNotIgnoreExplicit tests the fix for the bug where
// having reloader.stakater.com/auto: "true" would cause explicit reload annotations
// to be ignored due to an early return.
func TestMatcher_BugFix_AutoDoesNotIgnoreExplicit(t *testing.T) {
	cfg := config.NewDefault()
	matcher := NewMatcher(cfg)

	// This is the exact scenario from the bug report:
	// Workload has:
	//   reloader.stakater.com/auto: "true" (watches all referenced CMs)
	//   configmap.reloader.stakater.com/reload: "external-config" (ALSO watches this one)
	// Container references: app-config
	//
	// When "external-config" changes:
	// - Expected: Reload (explicitly listed)
	// - Bug behavior: No reload (auto annotation causes early return)

	input := MatchInput{
		ResourceName:        "external-config", // Not referenced by workload
		ResourceNamespace:   "default",
		ResourceType:        ResourceTypeConfigMap,
		ResourceAnnotations: nil,
		WorkloadAnnotations: map[string]string{
			"reloader.stakater.com/auto":             "true",            // Enables auto-reload
			"configmap.reloader.stakater.com/reload": "external-config", // Explicit list
		},
		PodAnnotations: nil,
	}

	result := matcher.ShouldReload(input)

	if !result.ShouldReload {
		t.Errorf("BUG: Explicit reload annotation ignored when auto is enabled")
		t.Errorf("Expected ShouldReload=true for explicitly listed ConfigMap, got false")
	}

	// Should be marked as non-auto since it matched the explicit list
	if result.AutoReload {
		t.Errorf("Expected AutoReload=false for explicit match, got true")
	}

	t.Log("✓ Bug fixed: Explicit reload annotation works even when auto is enabled")
}

// TestMatcher_PrecedenceOrder verifies the correct order of precedence:
// 1. Ignore annotation → skip
// 2. Exclude annotation → skip
// 3. Explicit reload annotation → reload (BUG FIX: before auto!)
// 4. Search/Match → reload
// 5. Auto annotation → reload
// 6. Auto-reload-all → reload
func TestMatcher_PrecedenceOrder(t *testing.T) {
	cfg := config.NewDefault()
	matcher := NewMatcher(cfg)

	t.Run("explicit takes precedence over auto", func(t *testing.T) {
		input := MatchInput{
			ResourceName:      "my-config",
			ResourceNamespace: "default",
			ResourceType:      ResourceTypeConfigMap,
			WorkloadAnnotations: map[string]string{
				"reloader.stakater.com/auto":             "true",
				"configmap.reloader.stakater.com/reload": "my-config",
			},
		}
		result := matcher.ShouldReload(input)
		if result.AutoReload {
			t.Error("Expected explicit match (AutoReload=false), got auto match")
		}
		if !result.ShouldReload {
			t.Error("Expected ShouldReload=true")
		}
	})

	t.Run("ignore takes precedence over explicit", func(t *testing.T) {
		input := MatchInput{
			ResourceName:        "my-config",
			ResourceNamespace:   "default",
			ResourceType:        ResourceTypeConfigMap,
			ResourceAnnotations: map[string]string{"reloader.stakater.com/ignore": "true"},
			WorkloadAnnotations: map[string]string{
				"configmap.reloader.stakater.com/reload": "my-config",
			},
		}
		result := matcher.ShouldReload(input)
		if result.ShouldReload {
			t.Error("Expected ignore to take precedence, but got ShouldReload=true")
		}
	})

	t.Run("exclude takes precedence over explicit", func(t *testing.T) {
		input := MatchInput{
			ResourceName:      "my-config",
			ResourceNamespace: "default",
			ResourceType:      ResourceTypeConfigMap,
			WorkloadAnnotations: map[string]string{
				"configmap.reloader.stakater.com/reload":          "my-config",
				"configmaps.exclude.reloader.stakater.com/reload": "my-config",
			},
		}
		result := matcher.ShouldReload(input)
		if result.ShouldReload {
			t.Error("Expected exclude to take precedence, but got ShouldReload=true")
		}
	})
}
