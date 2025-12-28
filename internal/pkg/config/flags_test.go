package config

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestBindFlags(t *testing.T) {
	cfg := NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	BindFlags(fs, cfg)

	// Verify flags are registered
	expectedFlags := []string{
		"auto-reload-all",
		"reload-strategy",
		"is-Argo-Rollouts",
		"reload-on-create",
		"reload-on-delete",
		"sync-after-restart",
		"enable-ha",
		"leader-election-id",
		"leader-election-namespace",
		"leader-election-lease-duration",
		"leader-election-renew-deadline",
		"leader-election-retry-period",
		"leader-election-release-on-cancel",
		"webhook-url",
		"resources-to-ignore",
		"ignored-workload-types",
		"namespaces-to-ignore",
		"namespace-selector",
		"resource-label-selector",
		"log-format",
		"log-level",
		"metrics-addr",
		"health-addr",
		"enable-pprof",
		"pprof-addr",
		"auto-annotation",
		"configmap-auto-annotation",
		"secret-auto-annotation",
		"configmap-annotation",
		"secret-annotation",
		"auto-search-annotation",
		"search-match-annotation",
		"pause-deployment-annotation",
		"pause-deployment-time-annotation",
		"watch-namespace",
	}

	for _, flagName := range expectedFlags {
		if fs.Lookup(flagName) == nil {
			t.Errorf("Expected flag %q to be registered", flagName)
		}
	}
}

func TestBindFlags_DefaultValues(t *testing.T) {
	cfg := NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	BindFlags(fs, cfg)

	// Parse empty args to use defaults
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Check default values are preserved
	if cfg.ReloadStrategy != ReloadStrategyEnvVars {
		t.Errorf("ReloadStrategy = %v, want %v", cfg.ReloadStrategy, ReloadStrategyEnvVars)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestBindFlags_CustomValues(t *testing.T) {
	cfg := NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	BindFlags(fs, cfg)

	args := []string{
		"--auto-reload-all=true",
		"--reload-strategy=annotations",
		"--log-level=debug",
		"--log-format=json",
		"--webhook-url=https://example.com/hook",
		"--enable-ha=true",
		"--enable-pprof=true",
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if !cfg.AutoReloadAll {
		t.Error("AutoReloadAll should be true")
	}

	if cfg.ReloadStrategy != ReloadStrategyAnnotations {
		t.Errorf("ReloadStrategy = %v, want %v", cfg.ReloadStrategy, ReloadStrategyAnnotations)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}

	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}

	if cfg.WebhookURL != "https://example.com/hook" {
		t.Errorf("WebhookURL = %q, want %q", cfg.WebhookURL, "https://example.com/hook")
	}

	if !cfg.EnableHA {
		t.Error("EnableHA should be true")
	}

	if !cfg.EnablePProf {
		t.Error("EnablePProf should be true")
	}
}

func TestApplyFlags_BooleanStrings(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    bool
		wantErr bool
	}{
		{"true lowercase", []string{"--is-Argo-Rollouts=true"}, true, false},
		{"TRUE uppercase", []string{"--is-Argo-Rollouts=TRUE"}, true, false},
		{"1", []string{"--is-Argo-Rollouts=1"}, true, false},
		{"yes", []string{"--is-Argo-Rollouts=yes"}, true, false},
		{"false", []string{"--is-Argo-Rollouts=false"}, false, false},
		{"no", []string{"--is-Argo-Rollouts=no"}, false, false},
		{"0", []string{"--is-Argo-Rollouts=0"}, false, false},
		{"empty", []string{}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag values
			fv = flagValues{}

			cfg := NewDefault()
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			BindFlags(fs, cfg)

			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			err := ApplyFlags(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if cfg.ArgoRolloutsEnabled != tt.want {
				t.Errorf("ArgoRolloutsEnabled = %v, want %v", cfg.ArgoRolloutsEnabled, tt.want)
			}
		})
	}
}

func TestApplyFlags_CommaSeparatedLists(t *testing.T) {
	// Reset flag values
	fv = flagValues{}

	cfg := NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	args := []string{
		"--resources-to-ignore=configMaps,secrets",
		"--ignored-workload-types=jobs,cronjobs",
		"--namespaces-to-ignore=kube-system,kube-public",
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := ApplyFlags(cfg); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	// Check ignored resources
	if len(cfg.IgnoredResources) != 2 {
		t.Errorf("IgnoredResources length = %d, want 2", len(cfg.IgnoredResources))
	}
	if cfg.IgnoredResources[0] != "configMaps" || cfg.IgnoredResources[1] != "secrets" {
		t.Errorf("IgnoredResources = %v", cfg.IgnoredResources)
	}

	// Check ignored workloads
	if len(cfg.IgnoredWorkloads) != 2 {
		t.Errorf("IgnoredWorkloads length = %d, want 2", len(cfg.IgnoredWorkloads))
	}

	// Check ignored namespaces
	if len(cfg.IgnoredNamespaces) != 2 {
		t.Errorf("IgnoredNamespaces length = %d, want 2", len(cfg.IgnoredNamespaces))
	}
}

func TestApplyFlags_Selectors(t *testing.T) {
	// Reset flag values
	fv = flagValues{}

	cfg := NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	args := []string{
		"--namespace-selector=env=production,team=platform",
		"--resource-label-selector=app=myapp",
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := ApplyFlags(cfg); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.NamespaceSelectors) != 2 {
		t.Errorf("NamespaceSelectors length = %d, want 2", len(cfg.NamespaceSelectors))
	}

	if len(cfg.ResourceSelectors) != 1 {
		t.Errorf("ResourceSelectors length = %d, want 1", len(cfg.ResourceSelectors))
	}

	// Check string versions are preserved
	if len(cfg.NamespaceSelectorStrings) != 2 {
		t.Errorf("NamespaceSelectorStrings length = %d, want 2", len(cfg.NamespaceSelectorStrings))
	}
}

func TestApplyFlags_InvalidSelector(t *testing.T) {
	// Reset flag values
	fv = flagValues{}

	cfg := NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	args := []string{
		"--namespace-selector=env in (prod,staging", // missing closing paren
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	err := ApplyFlags(cfg)
	if err == nil {
		t.Error("ApplyFlags() should return error for invalid selector")
	}
}

func TestParseBoolString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"  true  ", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseBoolString(tt.input)
			if got != tt.want {
				t.Errorf("parseBoolString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"single value", "abc", []string{"abc"}},
		{"multiple values", "a,b,c", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", []string{"a", "b", "c"}},
		{"empty elements", "a,,b", []string{"a", "b"}},
		{"only commas", ",,,", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitAndTrim(%q) length = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitAndTrim(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
