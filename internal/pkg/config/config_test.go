package config

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"
)

func TestNewDefault(t *testing.T) {
	cfg := NewDefault()

	if cfg == nil {
		t.Fatal("NewDefault() returned nil")
	}

	if cfg.ReloadStrategy != ReloadStrategyEnvVars {
		t.Errorf("ReloadStrategy = %v, want %v", cfg.ReloadStrategy, ReloadStrategyEnvVars)
	}

	if cfg.ArgoRolloutStrategy != ArgoRolloutStrategyRollout {
		t.Errorf("ArgoRolloutStrategy = %v, want %v", cfg.ArgoRolloutStrategy, ArgoRolloutStrategyRollout)
	}

	if cfg.AutoReloadAll {
		t.Error("AutoReloadAll should be false by default")
	}

	if cfg.ArgoRolloutsEnabled {
		t.Error("ArgoRolloutsEnabled should be false by default")
	}

	if cfg.ReloadOnCreate {
		t.Error("ReloadOnCreate should be false by default")
	}

	if cfg.ReloadOnDelete {
		t.Error("ReloadOnDelete should be false by default")
	}

	if cfg.EnableHA {
		t.Error("EnableHA should be false by default")
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}

	if cfg.MetricsAddr != ":9090" {
		t.Errorf("MetricsAddr = %q, want %q", cfg.MetricsAddr, ":9090")
	}

	if cfg.HealthAddr != ":8080" {
		t.Errorf("HealthAddr = %q, want %q", cfg.HealthAddr, ":8080")
	}

	if cfg.PProfAddr != ":6060" {
		t.Errorf("PProfAddr = %q, want %q", cfg.PProfAddr, ":6060")
	}
}

func TestDefaultAnnotations(t *testing.T) {
	ann := DefaultAnnotations()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Prefix", ann.Prefix, "reloader.stakater.com"},
		{"Auto", ann.Auto, "reloader.stakater.com/auto"},
		{"ConfigmapAuto", ann.ConfigmapAuto, "configmap.reloader.stakater.com/auto"},
		{"SecretAuto", ann.SecretAuto, "secret.reloader.stakater.com/auto"},
		{"ConfigmapReload", ann.ConfigmapReload, "configmap.reloader.stakater.com/reload"},
		{"SecretReload", ann.SecretReload, "secret.reloader.stakater.com/reload"},
		{"ConfigmapExclude", ann.ConfigmapExclude, "configmaps.exclude.reloader.stakater.com/reload"},
		{"SecretExclude", ann.SecretExclude, "secrets.exclude.reloader.stakater.com/reload"},
		{"Ignore", ann.Ignore, "reloader.stakater.com/ignore"},
		{"Search", ann.Search, "reloader.stakater.com/search"},
		{"Match", ann.Match, "reloader.stakater.com/match"},
		{"RolloutStrategy", ann.RolloutStrategy, "reloader.stakater.com/rollout-strategy"},
		{"PausePeriod", ann.PausePeriod, "deployment.reloader.stakater.com/pause-period"},
		{"PausedAt", ann.PausedAt, "deployment.reloader.stakater.com/paused-at"},
		{"LastReloadedFrom", ann.LastReloadedFrom, "reloader.stakater.com/last-reloaded-from"},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if tt.got != tt.want {
					t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
				}
			},
		)
	}
}

func TestDefaultLeaderElection(t *testing.T) {
	cfg := NewDefault()

	if cfg.LeaderElection.LockName != "reloader-leader-election" {
		t.Errorf("LockName = %q, want %q", cfg.LeaderElection.LockName, "reloader-leader-election")
	}

	if cfg.LeaderElection.LeaseDuration != 15*time.Second {
		t.Errorf("LeaseDuration = %v, want %v", cfg.LeaderElection.LeaseDuration, 15*time.Second)
	}

	if cfg.LeaderElection.RenewDeadline != 10*time.Second {
		t.Errorf("RenewDeadline = %v, want %v", cfg.LeaderElection.RenewDeadline, 10*time.Second)
	}

	if cfg.LeaderElection.RetryPeriod != 2*time.Second {
		t.Errorf("RetryPeriod = %v, want %v", cfg.LeaderElection.RetryPeriod, 2*time.Second)
	}

	if !cfg.LeaderElection.ReleaseOnCancel {
		t.Error("ReleaseOnCancel should be true by default")
	}
}

func TestConfig_IsResourceIgnored(t *testing.T) {
	cfg := NewDefault()
	cfg.IgnoredResources = []string{"configmaps", "secrets"}

	tests := []struct {
		name     string
		resource string
		want     bool
	}{
		{"exact match lowercase", "configmaps", true},
		{"exact match uppercase", "CONFIGMAPS", true},
		{"exact match mixed case", "ConfigMaps", true},
		{"not ignored", "deployments", false},
		{"partial match (not ignored)", "config", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := cfg.IsResourceIgnored(tt.resource)
				if got != tt.want {
					t.Errorf("IsResourceIgnored(%q) = %v, want %v", tt.resource, got, tt.want)
				}
			},
		)
	}
}

func TestConfig_IsWorkloadIgnored(t *testing.T) {
	cfg := NewDefault()
	cfg.IgnoredWorkloads = []string{"jobs", "cronjobs"}

	tests := []struct {
		name     string
		workload string
		want     bool
	}{
		{"exact match", "jobs", true},
		{"case insensitive", "JOBS", true},
		{"not ignored", "deployments", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := cfg.IsWorkloadIgnored(tt.workload)
				if got != tt.want {
					t.Errorf("IsWorkloadIgnored(%q) = %v, want %v", tt.workload, got, tt.want)
				}
			},
		)
	}
}

func TestDefaultAnnotationsSecretProviderClass(t *testing.T) {
	a := DefaultAnnotations()
	if a.SecretProviderClassAuto != "secretproviderclass.reloader.stakater.com/auto" {
		t.Fatalf("SecretProviderClassAuto = %q", a.SecretProviderClassAuto)
	}
	if a.SecretProviderClassReload != "secretproviderclass.reloader.stakater.com/reload" {
		t.Fatalf("SecretProviderClassReload = %q", a.SecretProviderClassReload)
	}
	if a.SecretProviderClassExclude != "secretproviderclasses.exclude.reloader.stakater.com/reload" {
		t.Fatalf("SecretProviderClassExclude = %q", a.SecretProviderClassExclude)
	}
}

func TestNewDefaultCSIDisabled(t *testing.T) {
	if NewDefault().CSIIntegrationEnabled {
		t.Fatal("CSIIntegrationEnabled should default to false")
	}
}

func TestConfig_IsNamespaceIgnored(t *testing.T) {
	cfg := NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system", "kube-public"}

	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{"exact match", "kube-system", true},
		{"case sensitive no match", "Kube-System", false},
		{"not ignored", "default", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := cfg.IsNamespaceIgnored(tt.namespace)
				if got != tt.want {
					t.Errorf("IsNamespaceIgnored(%q) = %v, want %v", tt.namespace, got, tt.want)
				}
			},
		)
	}
}

func TestIsGlobalMode(t *testing.T) {
	c := &Config{WatchedNamespaces: nil}
	if !c.IsGlobalMode() {
		t.Errorf("empty WatchedNamespaces should be global mode")
	}
	c.WatchedNamespaces = []string{"team-a"}
	if c.IsGlobalMode() {
		t.Errorf("non-empty WatchedNamespaces should not be global mode")
	}
}

func TestApplyNamespaceScope_GlobalKeepsSettings(t *testing.T) {
	c := &Config{
		WatchedNamespaces:  nil,
		IgnoredNamespaces:  []string{"kube-system"},
		NamespaceSelectors: []labels.Selector{labels.Everything()},
	}
	warnings := c.ApplyNamespaceScope()
	if len(warnings) != 0 {
		t.Errorf("global mode should produce no warnings, got %v", warnings)
	}
	if len(c.IgnoredNamespaces) != 1 || len(c.NamespaceSelectors) != 1 {
		t.Errorf("global mode should keep selectors and ignored namespaces")
	}
}

func TestApplyNamespaceScope_ScopedClearsSettings(t *testing.T) {
	c := &Config{
		WatchedNamespaces:        []string{"team-a"},
		IgnoredNamespaces:        []string{"kube-system"},
		NamespaceSelectors:       []labels.Selector{labels.Everything()},
		NamespaceSelectorStrings: []string{"env=prod"},
	}
	warnings := c.ApplyNamespaceScope()
	if len(warnings) != 2 {
		t.Errorf("scoped mode should warn about both dropped settings, got %v", warnings)
	}
	if len(c.IgnoredNamespaces) != 0 || len(c.NamespaceSelectors) != 0 || len(c.NamespaceSelectorStrings) != 0 {
		t.Errorf("scoped mode should clear selectors and ignored namespaces")
	}
}
