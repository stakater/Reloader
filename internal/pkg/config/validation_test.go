package config

import (
	"strings"
	"testing"
)

func TestConfig_Validate_ReloadStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy ReloadStrategy
		wantErr  bool
		wantVal  ReloadStrategy
	}{
		{"valid env-vars", ReloadStrategyEnvVars, false, ReloadStrategyEnvVars},
		{"valid annotations", ReloadStrategyAnnotations, false, ReloadStrategyAnnotations},
		{"empty defaults to env-vars", "", false, ReloadStrategyEnvVars},
		{"invalid strategy", "invalid", true, ""},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := NewDefault()
				cfg.ReloadStrategy = tt.strategy

				err := cfg.Validate()

				if tt.wantErr {
					if err == nil {
						t.Error("Validate() should return error for invalid strategy")
					}
					return
				}

				if err != nil {
					t.Errorf("Validate() error = %v", err)
					return
				}

				if cfg.ReloadStrategy != tt.wantVal {
					t.Errorf("ReloadStrategy = %v, want %v", cfg.ReloadStrategy, tt.wantVal)
				}
			},
		)
	}
}

func TestConfig_Validate_ArgoRolloutStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy ArgoRolloutStrategy
		wantErr  bool
		wantVal  ArgoRolloutStrategy
	}{
		{"valid restart", ArgoRolloutStrategyRestart, false, ArgoRolloutStrategyRestart},
		{"valid rollout", ArgoRolloutStrategyRollout, false, ArgoRolloutStrategyRollout},
		{"empty defaults to rollout", "", false, ArgoRolloutStrategyRollout},
		{"invalid strategy", "invalid", true, ""},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := NewDefault()
				cfg.ArgoRolloutStrategy = tt.strategy

				err := cfg.Validate()

				if tt.wantErr {
					if err == nil {
						t.Error("Validate() should return error for invalid strategy")
					}
					return
				}

				if err != nil {
					t.Errorf("Validate() error = %v", err)
					return
				}

				if cfg.ArgoRolloutStrategy != tt.wantVal {
					t.Errorf("ArgoRolloutStrategy = %v, want %v", cfg.ArgoRolloutStrategy, tt.wantVal)
				}
			},
		)
	}
}

func TestConfig_Validate_LogLevel(t *testing.T) {
	validLevels := []string{"trace", "debug", "info", "warn", "warning", "error", "fatal", "panic", ""}
	for _, level := range validLevels {
		t.Run(
			"valid_"+level, func(t *testing.T) {
				cfg := NewDefault()
				cfg.LogLevel = level
				if err := cfg.Validate(); err != nil {
					t.Errorf("Validate() error for level %q: %v", level, err)
				}
			},
		)
	}

	t.Run(
		"invalid level", func(t *testing.T) {
			cfg := NewDefault()
			cfg.LogLevel = "invalid"
			err := cfg.Validate()
			if err == nil {
				t.Error("Validate() should return error for invalid log level")
			}
		},
	)
}

func TestConfig_Validate_LogFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"json format", "json", false},
		{"empty format", "", false},
		{"invalid format", "xml", true},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := NewDefault()
				cfg.LogFormat = tt.format
				err := cfg.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			},
		)
	}
}

func TestConfig_Validate_NormalizesIgnoredResources(t *testing.T) {
	cfg := NewDefault()
	cfg.IgnoredResources = []string{"ConfigMaps", "SECRETS", "  spaces  "}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	expected := []string{"configmaps", "secrets", "spaces"}
	if len(cfg.IgnoredResources) != len(expected) {
		t.Fatalf("IgnoredResources length = %d, want %d", len(cfg.IgnoredResources), len(expected))
	}

	for i, got := range cfg.IgnoredResources {
		if got != expected[i] {
			t.Errorf("IgnoredResources[%d] = %q, want %q", i, got, expected[i])
		}
	}
}

func TestConfig_Validate_NormalizesIgnoredWorkloads(t *testing.T) {
	cfg := NewDefault()
	cfg.IgnoredWorkloads = []string{"Jobs", "CRONJOBS", ""}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	expected := []string{"jobs", "cronjobs"}
	if len(cfg.IgnoredWorkloads) != len(expected) {
		t.Fatalf("IgnoredWorkloads length = %d, want %d", len(cfg.IgnoredWorkloads), len(expected))
	}

	for i, got := range cfg.IgnoredWorkloads {
		if got != expected[i] {
			t.Errorf("IgnoredWorkloads[%d] = %q, want %q", i, got, expected[i])
		}
	}
}

func TestConfig_Validate_InvalidIgnoredWorkload(t *testing.T) {
	cfg := NewDefault()
	cfg.IgnoredWorkloads = []string{"deployment", "invalidtype"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should return error for invalid workload type")
	}

	if !strings.Contains(err.Error(), "invalidtype") {
		t.Errorf("Error should mention invalid workload type, got: %v", err)
	}
}

func TestConfig_Validate_MultipleErrors(t *testing.T) {
	cfg := NewDefault()
	cfg.ReloadStrategy = "invalid"
	cfg.ArgoRolloutStrategy = "invalid"
	cfg.LogLevel = "invalid"
	cfg.LogFormat = "invalid"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should return error for multiple invalid values")
	}

	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors, got %T", err)
	}

	if len(errs) != 4 {
		t.Errorf("Expected 4 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "TestField",
		Message: "test message",
	}

	expected := "config.TestField: test message"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestValidationErrors_Error(t *testing.T) {
	t.Run(
		"empty", func(t *testing.T) {
			var errs ValidationErrors
			if errs.Error() != "" {
				t.Errorf("Empty errors should return empty string, got %q", errs.Error())
			}
		},
	)

	t.Run(
		"single error", func(t *testing.T) {
			errs := ValidationErrors{
				{Field: "Field1", Message: "error1"},
			}
			if !strings.Contains(errs.Error(), "Field1") {
				t.Errorf("Error() should contain field name, got %q", errs.Error())
			}
		},
	)

	t.Run(
		"multiple errors", func(t *testing.T) {
			errs := ValidationErrors{
				{Field: "Field1", Message: "error1"},
				{Field: "Field2", Message: "error2"},
			}
			errStr := errs.Error()
			if !strings.Contains(errStr, "multiple configuration errors") {
				t.Errorf("Error() should mention multiple errors, got %q", errStr)
			}
			if !strings.Contains(errStr, "Field1") || !strings.Contains(errStr, "Field2") {
				t.Errorf("Error() should contain all field names, got %q", errStr)
			}
		},
	)
}

func TestParseSelectors(t *testing.T) {
	tests := []struct {
		name      string
		selectors []string
		wantLen   int
		wantErr   bool
	}{
		{"nil input", nil, 0, false},
		{"empty input", []string{}, 0, false},
		{"single valid selector", []string{"env=production"}, 1, false},
		{"multiple valid selectors", []string{"env=production", "team=platform"}, 2, false},
		{"selector with whitespace", []string{"  env=production  "}, 1, false},
		{"empty string in list", []string{"env=production", "", "team=platform"}, 2, false},
		{"invalid selector syntax", []string{"env in (prod,staging"}, 0, true}, // missing closing paren
		{"set-based selector", []string{"env in (prod,staging)"}, 1, false},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				selectors, err := ParseSelectors(tt.selectors)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseSelectors() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && len(selectors) != tt.wantLen {
					t.Errorf("ParseSelectors() returned %d selectors, want %d", len(selectors), tt.wantLen)
				}
			},
		)
	}
}

func TestNormalizeToLower(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil input", nil, nil},
		{"empty input", []string{}, []string{}},
		{"lowercase", []string{"abc"}, []string{"abc"}},
		{"uppercase", []string{"ABC"}, []string{"abc"}},
		{"mixed case", []string{"AbC"}, []string{"abc"}},
		{"with whitespace", []string{"  abc  "}, []string{"abc"}},
		{"removes empty", []string{"abc", "", "def"}, []string{"abc", "def"}},
		{"only whitespace", []string{"   "}, []string{}},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := normalizeToLower(tt.input)
				if tt.want == nil && got != nil {
					t.Errorf("normalizeToLower() = %v, want nil", got)
					return
				}
				if len(got) != len(tt.want) {
					t.Errorf("normalizeToLower() length = %d, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("normalizeToLower()[%d] = %q, want %q", i, got[i], tt.want[i])
					}
				}
			},
		)
	}
}
