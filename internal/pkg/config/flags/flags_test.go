package flags

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/stakater/Reloader/pkg/config"
)

// resetViper resets the viper instance for testing.
func resetViper() {
	v = viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
}

func TestBindFlags(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	BindFlags(fs, cfg)

	expectedFlags := []string{
		"auto-reload-all",
		"reload-strategy",
		"is-Argo-Rollouts",
		"is-openshift",
		"enable-csi-integration",
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
		"configmap-exclude-annotation",
		"secret-exclude-annotation",
		"secretproviderclass-auto-annotation",
		"secretproviderclass-annotation",
		"secretproviderclass-exclude-annotation",
		"auto-search-annotation",
		"search-match-annotation",
		"ignore-annotation",
		"pause-deployment-annotation",
		"pause-deployment-time-annotation",
		"namespaces",
		"alert-on-reload",
		"alert-webhook-url",
		"alert-sink",
		"alert-proxy",
		"alert-additional-info",
		"alert-structured",
	}

	for _, flagName := range expectedFlags {
		if fs.Lookup(flagName) == nil {
			t.Errorf("Expected flag %q to be registered", flagName)
		}
	}
}

func TestBindFlags_DefaultValues(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	BindFlags(fs, cfg)

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if cfg.ReloadStrategy != config.ReloadStrategyEnvVars {
		t.Errorf("ReloadStrategy = %v, want %v", cfg.ReloadStrategy, config.ReloadStrategyEnvVars)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestBindFlags_CustomValues(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
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

	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if !cfg.AutoReloadAll {
		t.Error("AutoReloadAll should be true")
	}

	if cfg.ReloadStrategy != config.ReloadStrategyAnnotations {
		t.Errorf("ReloadStrategy = %v, want %v", cfg.ReloadStrategy, config.ReloadStrategyAnnotations)
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

func TestApplyFlags_SecretProviderClassAnnotations(t *testing.T) {
	// Defaults are preserved when the flags are not provided.
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}
	defaults := config.DefaultAnnotations()
	if cfg.Annotations.SecretProviderClassAuto != defaults.SecretProviderClassAuto {
		t.Errorf("SecretProviderClassAuto = %q, want default %q", cfg.Annotations.SecretProviderClassAuto, defaults.SecretProviderClassAuto)
	}
	if cfg.Annotations.SecretProviderClassReload != defaults.SecretProviderClassReload {
		t.Errorf("SecretProviderClassReload = %q, want default %q", cfg.Annotations.SecretProviderClassReload, defaults.SecretProviderClassReload)
	}
	if cfg.Annotations.SecretProviderClassExclude != defaults.SecretProviderClassExclude {
		t.Errorf("SecretProviderClassExclude = %q, want default %q", cfg.Annotations.SecretProviderClassExclude, defaults.SecretProviderClassExclude)
	}

	// Custom values are applied from the flags.
	resetViper()
	cfg = config.NewDefault()
	fs = pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)
	args := []string{
		"--secretproviderclass-auto-annotation=spc.example.com/auto",
		"--secretproviderclass-annotation=spc.example.com/reload",
		"--secretproviderclass-exclude-annotation=spc.example.com/exclude",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}
	if cfg.Annotations.SecretProviderClassAuto != "spc.example.com/auto" {
		t.Errorf("SecretProviderClassAuto = %q, want %q", cfg.Annotations.SecretProviderClassAuto, "spc.example.com/auto")
	}
	if cfg.Annotations.SecretProviderClassReload != "spc.example.com/reload" {
		t.Errorf("SecretProviderClassReload = %q, want %q", cfg.Annotations.SecretProviderClassReload, "spc.example.com/reload")
	}
	if cfg.Annotations.SecretProviderClassExclude != "spc.example.com/exclude" {
		t.Errorf("SecretProviderClassExclude = %q, want %q", cfg.Annotations.SecretProviderClassExclude, "spc.example.com/exclude")
	}
}

func TestApplyFlags_ExcludeAnnotations(t *testing.T) {
	// Defaults are preserved when the flags are not provided.
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}
	defaults := config.DefaultAnnotations()
	if cfg.Annotations.ConfigmapExclude != defaults.ConfigmapExclude {
		t.Errorf("ConfigmapExclude = %q, want default %q", cfg.Annotations.ConfigmapExclude, defaults.ConfigmapExclude)
	}
	if cfg.Annotations.SecretExclude != defaults.SecretExclude {
		t.Errorf("SecretExclude = %q, want default %q", cfg.Annotations.SecretExclude, defaults.SecretExclude)
	}

	// Custom values are applied from the flags.
	resetViper()
	cfg = config.NewDefault()
	fs = pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)
	args := []string{
		"--configmap-exclude-annotation=cm.example.com/exclude",
		"--secret-exclude-annotation=sec.example.com/exclude",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}
	if cfg.Annotations.ConfigmapExclude != "cm.example.com/exclude" {
		t.Errorf("ConfigmapExclude = %q, want %q", cfg.Annotations.ConfigmapExclude, "cm.example.com/exclude")
	}
	if cfg.Annotations.SecretExclude != "sec.example.com/exclude" {
		t.Errorf("SecretExclude = %q, want %q", cfg.Annotations.SecretExclude, "sec.example.com/exclude")
	}
}

func TestApplyFlags_IgnoreAnnotation(t *testing.T) {
	// Default is preserved when the flag is not provided.
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}
	if cfg.Annotations.Ignore != config.DefaultAnnotations().Ignore {
		t.Errorf("Ignore = %q, want default %q", cfg.Annotations.Ignore, config.DefaultAnnotations().Ignore)
	}

	// Custom value is applied from the flag.
	resetViper()
	cfg = config.NewDefault()
	fs = pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)
	if err := fs.Parse([]string{"--ignore-annotation=my.company.com/reloader-ignore"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}
	if cfg.Annotations.Ignore != "my.company.com/reloader-ignore" {
		t.Errorf("Ignore = %q, want %q", cfg.Annotations.Ignore, "my.company.com/reloader-ignore")
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
		t.Run(
			tt.name, func(t *testing.T) {
				resetViper()
				cfg := config.NewDefault()
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				BindFlags(fs, cfg)

				if err := fs.Parse(tt.args); err != nil {
					t.Fatalf("Parse() error = %v", err)
				}

				err := ApplyFlags(cfg, logr.Discard())
				if (err != nil) != tt.wantErr {
					t.Errorf("ApplyFlags() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if cfg.ArgoRolloutsEnabled != tt.want {
					t.Errorf("ArgoRolloutsEnabled = %v, want %v", cfg.ArgoRolloutsEnabled, tt.want)
				}
			},
		)
	}
}

func TestApplyFlags_CommaSeparatedLists(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
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

	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.IgnoredResources) != 2 {
		t.Errorf("IgnoredResources length = %d, want 2", len(cfg.IgnoredResources))
	}
	if cfg.IgnoredResources[0] != "configMaps" || cfg.IgnoredResources[1] != "secrets" {
		t.Errorf("IgnoredResources = %v", cfg.IgnoredResources)
	}

	if len(cfg.IgnoredWorkloads) != 2 {
		t.Errorf("IgnoredWorkloads length = %d, want 2", len(cfg.IgnoredWorkloads))
	}

	if len(cfg.IgnoredNamespaces) != 2 {
		t.Errorf("IgnoredNamespaces length = %d, want 2", len(cfg.IgnoredNamespaces))
	}
}

func TestApplyFlags_Selectors(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	args := []string{
		"--namespace-selector=env=production,team=platform",
		"--resource-label-selector=app=myapp",
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.NamespaceSelectors) != 1 {
		t.Errorf("NamespaceSelectors length = %d, want 1", len(cfg.NamespaceSelectors))
	}

	if len(cfg.ResourceSelectors) != 1 {
		t.Errorf("ResourceSelectors length = %d, want 1", len(cfg.ResourceSelectors))
	}

	if len(cfg.NamespaceSelectorStrings) != 2 {
		t.Errorf("NamespaceSelectorStrings length = %d, want 2", len(cfg.NamespaceSelectorStrings))
	}
}

func TestApplyFlags_InvalidSelector(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	args := []string{
		"--namespace-selector=env in (prod,staging", // missing closing paren
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	err := ApplyFlags(cfg, logr.Discard())
	if err == nil {
		t.Error("ApplyFlags() should return error for invalid selector")
	}
}

func TestApplyFlags_AlertingEnvVars(t *testing.T) {
	tests := []struct {
		name       string
		envVars    map[string]string
		wantURL    string
		wantSink   string
		wantEnable bool
	}{
		{
			name: "ALERT_WEBHOOK_URL enables alerting",
			envVars: map[string]string{
				"ALERT_WEBHOOK_URL": "https://hooks.example.com",
			},
			wantURL:    "https://hooks.example.com",
			wantEnable: true,
		},
		{
			name: "all alert env vars",
			envVars: map[string]string{
				"ALERT_WEBHOOK_URL":   "https://hooks.example.com",
				"ALERT_SINK":          "slack",
				"ALERT_WEBHOOK_PROXY": "http://proxy:8080",
			},
			wantURL:    "https://hooks.example.com",
			wantSink:   "slack",
			wantEnable: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				resetViper()

				for k, val := range tt.envVars {
					t.Setenv(k, val)
				}

				cfg := config.NewDefault()
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				BindFlags(fs, cfg)

				if err := fs.Parse([]string{}); err != nil {
					t.Fatalf("Parse() error = %v", err)
				}

				if err := ApplyFlags(cfg, logr.Discard()); err != nil {
					t.Fatalf("ApplyFlags() error = %v", err)
				}

				if cfg.Alerting.WebhookURL != tt.wantURL {
					t.Errorf("Alerting.WebhookURL = %q, want %q", cfg.Alerting.WebhookURL, tt.wantURL)
				}

				if tt.wantSink != "" && cfg.Alerting.Sink != tt.wantSink {
					t.Errorf("Alerting.Sink = %q, want %q", cfg.Alerting.Sink, tt.wantSink)
				}

				if cfg.Alerting.Enabled != tt.wantEnable {
					t.Errorf("Alerting.Enabled = %v, want %v", cfg.Alerting.Enabled, tt.wantEnable)
				}
			},
		)
	}
}

func TestApplyFlags_LegacyProxyEnvVar(t *testing.T) {
	resetViper()

	t.Setenv("ALERT_WEBHOOK_PROXY", "http://legacy-proxy:8080")

	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if cfg.Alerting.Proxy != "http://legacy-proxy:8080" {
		t.Errorf("Alerting.Proxy = %q, want %q", cfg.Alerting.Proxy, "http://legacy-proxy:8080")
	}
}

func TestApplyFlagsCSIIntegration(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)
	if err := fs.Parse([]string{"--enable-csi-integration=true"}); err != nil {
		t.Fatal(err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatal(err)
	}
	if !cfg.CSIIntegrationEnabled {
		t.Fatal("expected CSIIntegrationEnabled=true")
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
		t.Run(
			tt.input, func(t *testing.T) {
				got := parseBoolString(tt.input)
				if got != tt.want {
					t.Errorf("parseBoolString(%q) = %v, want %v", tt.input, got, tt.want)
				}
			},
		)
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
		t.Run(
			tt.name, func(t *testing.T) {
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
			},
		)
	}
}

func TestApplyFlags_NamespacesScoped(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{"--namespaces=team-a,team-b"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.WatchedNamespaces) != 2 {
		t.Fatalf("WatchedNamespaces length = %d, want 2", len(cfg.WatchedNamespaces))
	}
	if cfg.WatchedNamespaces[0] != "team-a" || cfg.WatchedNamespaces[1] != "team-b" {
		t.Errorf("WatchedNamespaces = %v", cfg.WatchedNamespaces)
	}
	if cfg.IsGlobalMode() {
		t.Errorf("explicit namespaces should not be global mode")
	}
}

func TestApplyFlags_NamespacesFromEnv(t *testing.T) {
	resetViper()
	t.Setenv("KUBERNETES_NAMESPACE", "single-ns")
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.WatchedNamespaces) != 1 || cfg.WatchedNamespaces[0] != "single-ns" {
		t.Errorf("WatchedNamespaces = %v, want [single-ns]", cfg.WatchedNamespaces)
	}
}

func TestApplyFlags_NamespacesGlobal(t *testing.T) {
	resetViper()
	t.Setenv("KUBERNETES_NAMESPACE", "")
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.WatchedNamespaces) != 0 {
		t.Errorf("WatchedNamespaces = %v, want empty (global)", cfg.WatchedNamespaces)
	}
	if !cfg.IsGlobalMode() {
		t.Errorf("no namespaces and no env should be global mode")
	}
}

func TestApplyFlags_NamespacesTrimsEmptyEntries(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{"--namespaces=team-a, ,team-b,"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.WatchedNamespaces) != 2 {
		t.Fatalf("WatchedNamespaces length = %d, want 2", len(cfg.WatchedNamespaces))
	}
	if cfg.WatchedNamespaces[0] != "team-a" || cfg.WatchedNamespaces[1] != "team-b" {
		t.Errorf("WatchedNamespaces = %v, want [team-a team-b]", cfg.WatchedNamespaces)
	}
	if cfg.IsGlobalMode() {
		t.Errorf("trimmed namespaces should not be global mode")
	}
}

func TestApplyFlags_NamespacesAllEmptyIsGlobal(t *testing.T) {
	resetViper()
	t.Setenv("KUBERNETES_NAMESPACE", "")
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{"--namespaces=, ,"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.WatchedNamespaces) != 0 {
		t.Errorf("WatchedNamespaces = %v, want empty (global)", cfg.WatchedNamespaces)
	}
	if !cfg.IsGlobalMode() {
		t.Errorf("all-empty namespaces should be global mode")
	}
}

// ApplyFlags must finalize a self-consistent config: in scoped mode it enforces
// namespace-scope semantics (clears selector/ignore lists) and logs a warning
// for each dropped setting.
func TestApplyFlags_ScopedClearsSelectorsAndIgnores(t *testing.T) {
	resetViper()
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{
		"--namespaces=team-a",
		"--namespace-selector=env=prod",
		"--namespaces-to-ignore=kube-system",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if len(cfg.NamespaceSelectors) != 0 || len(cfg.NamespaceSelectorStrings) != 0 {
		t.Errorf("scoped mode should clear namespace selectors, got %v", cfg.NamespaceSelectorStrings)
	}
	if len(cfg.IgnoredNamespaces) != 0 {
		t.Errorf("scoped mode should clear ignored namespaces, got %v", cfg.IgnoredNamespaces)
	}
}

func TestApplyFlags_GlobalKeepsSelectorsNoWarnings(t *testing.T) {
	resetViper()
	t.Setenv("KUBERNETES_NAMESPACE", "")
	cfg := config.NewDefault()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(fs, cfg)

	if err := fs.Parse([]string{
		"--namespace-selector=env=prod",
		"--namespaces-to-ignore=kube-system",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ApplyFlags(cfg, logr.Discard()); err != nil {
		t.Fatalf("ApplyFlags() error = %v", err)
	}

	if !cfg.IsGlobalMode() {
		t.Fatalf("no --namespaces should be global mode")
	}
	if len(cfg.NamespaceSelectors) != 1 || len(cfg.IgnoredNamespaces) != 1 {
		t.Errorf("global mode should keep selectors and ignored namespaces")
	}
}
