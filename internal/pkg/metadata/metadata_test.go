package metadata

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewBuildInfo(t *testing.T) {
	// Set build variables for testing
	oldVersion := Version
	oldCommit := Commit
	oldBuildDate := BuildDate
	defer func() {
		Version = oldVersion
		Commit = oldCommit
		BuildDate = oldBuildDate
	}()

	Version = "1.0.0"
	Commit = "abc123"
	BuildDate = "2024-01-01T12:00:00Z"

	info := NewBuildInfo()

	if info.ReleaseVersion != "1.0.0" {
		t.Errorf("ReleaseVersion = %s, want 1.0.0", info.ReleaseVersion)
	}
	if info.CommitHash != "abc123" {
		t.Errorf("CommitHash = %s, want abc123", info.CommitHash)
	}
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if info.CommitTime.IsZero() {
		t.Error("CommitTime should not be zero")
	}
}

func TestNewReloaderOptions(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true
	cfg.ReloadStrategy = config.ReloadStrategyAnnotations
	cfg.ArgoRolloutsEnabled = true
	cfg.ReloadOnCreate = true
	cfg.ReloadOnDelete = true
	cfg.EnableHA = true
	cfg.WebhookURL = "https://example.com/webhook"
	cfg.LogFormat = "json"
	cfg.LogLevel = "debug"
	cfg.IgnoredResources = []string{"configmaps"}
	cfg.IgnoredWorkloads = []string{"jobs"}
	cfg.IgnoredNamespaces = []string{"kube-system"}

	opts := NewReloaderOptions(cfg)

	if !opts.AutoReloadAll {
		t.Error("AutoReloadAll should be true")
	}
	if opts.ReloadStrategy != "annotations" {
		t.Errorf("ReloadStrategy = %s, want annotations", opts.ReloadStrategy)
	}
	if !opts.IsArgoRollouts {
		t.Error("IsArgoRollouts should be true")
	}
	if !opts.ReloadOnCreate {
		t.Error("ReloadOnCreate should be true")
	}
	if !opts.ReloadOnDelete {
		t.Error("ReloadOnDelete should be true")
	}
	if !opts.EnableHA {
		t.Error("EnableHA should be true")
	}
	if opts.WebhookURL != "https://example.com/webhook" {
		t.Errorf("WebhookURL = %s, want https://example.com/webhook", opts.WebhookURL)
	}
	if opts.LogFormat != "json" {
		t.Errorf("LogFormat = %s, want json", opts.LogFormat)
	}
	if opts.LogLevel != "debug" {
		t.Errorf("LogLevel = %s, want debug", opts.LogLevel)
	}
	if len(opts.ResourcesToIgnore) != 1 || opts.ResourcesToIgnore[0] != "configmaps" {
		t.Errorf("ResourcesToIgnore = %v, want [configmaps]", opts.ResourcesToIgnore)
	}
	if len(opts.WorkloadTypesToIgnore) != 1 || opts.WorkloadTypesToIgnore[0] != "jobs" {
		t.Errorf("WorkloadTypesToIgnore = %v, want [jobs]", opts.WorkloadTypesToIgnore)
	}
	if len(opts.NamespacesToIgnore) != 1 || opts.NamespacesToIgnore[0] != "kube-system" {
		t.Errorf("NamespacesToIgnore = %v, want [kube-system]", opts.NamespacesToIgnore)
	}

	// Check annotations
	if opts.ReloaderAutoAnnotation != "reloader.stakater.com/auto" {
		t.Errorf("ReloaderAutoAnnotation = %s, want reloader.stakater.com/auto", opts.ReloaderAutoAnnotation)
	}
}

func TestMetaInfo_ToConfigMap(t *testing.T) {
	// Set environment variables
	os.Setenv(EnvReloaderNamespace, "reloader-ns")
	os.Setenv(EnvReloaderDeploymentName, "reloader-deploy")
	defer func() {
		os.Unsetenv(EnvReloaderNamespace)
		os.Unsetenv(EnvReloaderDeploymentName)
	}()

	cfg := config.NewDefault()
	metaInfo := NewMetaInfo(cfg)
	cm := metaInfo.ToConfigMap()

	if cm.Name != ConfigMapName {
		t.Errorf("Name = %s, want %s", cm.Name, ConfigMapName)
	}
	if cm.Namespace != "reloader-ns" {
		t.Errorf("Namespace = %s, want reloader-ns", cm.Namespace)
	}
	if cm.Labels[ConfigMapLabelKey] != ConfigMapLabelValue {
		t.Errorf("Label = %s, want %s", cm.Labels[ConfigMapLabelKey], ConfigMapLabelValue)
	}

	// Check data fields exist
	if _, ok := cm.Data["buildInfo"]; !ok {
		t.Error("buildInfo data key missing")
	}
	if _, ok := cm.Data["reloaderOptions"]; !ok {
		t.Error("reloaderOptions data key missing")
	}
	if _, ok := cm.Data["deploymentInfo"]; !ok {
		t.Error("deploymentInfo data key missing")
	}

	// Verify buildInfo is valid JSON
	var buildInfo BuildInfo
	if err := json.Unmarshal([]byte(cm.Data["buildInfo"]), &buildInfo); err != nil {
		t.Errorf("buildInfo is not valid JSON: %v", err)
	}

	// Verify deploymentInfo contains expected values
	var deployInfo DeploymentInfo
	if err := json.Unmarshal([]byte(cm.Data["deploymentInfo"]), &deployInfo); err != nil {
		t.Errorf("deploymentInfo is not valid JSON: %v", err)
	}
	if deployInfo.Namespace != "reloader-ns" {
		t.Errorf("DeploymentInfo.Namespace = %s, want reloader-ns", deployInfo.Namespace)
	}
	if deployInfo.Name != "reloader-deploy" {
		t.Errorf("DeploymentInfo.Name = %s, want reloader-deploy", deployInfo.Name)
	}
}

func TestPublisher_Publish_NoNamespace(t *testing.T) {
	// Ensure RELOADER_NAMESPACE is not set
	os.Unsetenv(EnvReloaderNamespace)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := config.NewDefault()
	publisher := NewPublisher(fakeClient, cfg)

	err := publisher.Publish(context.Background())
	if err != nil {
		t.Errorf("Publish() with no namespace should not error, got: %v", err)
	}
}

func TestPublisher_Publish_CreateNew(t *testing.T) {
	// Set environment variables
	os.Setenv(EnvReloaderNamespace, "test-ns")
	os.Setenv(EnvReloaderDeploymentName, "test-deploy")
	defer func() {
		os.Unsetenv(EnvReloaderNamespace)
		os.Unsetenv(EnvReloaderDeploymentName)
	}()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := config.NewDefault()
	publisher := NewPublisher(fakeClient, cfg)

	ctx := context.Background()
	err := publisher.Publish(ctx)
	if err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	// Verify ConfigMap was created
	cm := &corev1.ConfigMap{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: ConfigMapName, Namespace: "test-ns"}, cm)
	if err != nil {
		t.Errorf("Failed to get created ConfigMap: %v", err)
	}
	if cm.Name != ConfigMapName {
		t.Errorf("ConfigMap.Name = %s, want %s", cm.Name, ConfigMapName)
	}
}

func TestPublisher_Publish_UpdateExisting(t *testing.T) {
	// Set environment variables
	os.Setenv(EnvReloaderNamespace, "test-ns")
	os.Setenv(EnvReloaderDeploymentName, "test-deploy")
	defer func() {
		os.Unsetenv(EnvReloaderNamespace)
		os.Unsetenv(EnvReloaderDeploymentName)
	}()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create existing ConfigMap with old data
	existingCM := &corev1.ConfigMap{}
	existingCM.Name = ConfigMapName
	existingCM.Namespace = "test-ns"
	existingCM.Data = map[string]string{
		"buildInfo": `{"goVersion":"old"}`,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingCM).
		Build()

	cfg := config.NewDefault()
	publisher := NewPublisher(fakeClient, cfg)

	ctx := context.Background()
	err := publisher.Publish(ctx)
	if err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	// Verify ConfigMap was updated
	cm := &corev1.ConfigMap{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: ConfigMapName, Namespace: "test-ns"}, cm)
	if err != nil {
		t.Errorf("Failed to get updated ConfigMap: %v", err)
	}

	// Check that all data keys are present
	if _, ok := cm.Data["buildInfo"]; !ok {
		t.Error("buildInfo data key missing after update")
	}
	if _, ok := cm.Data["reloaderOptions"]; !ok {
		t.Error("reloaderOptions data key missing after update")
	}
	if _, ok := cm.Data["deploymentInfo"]; !ok {
		t.Error("deploymentInfo data key missing after update")
	}

	// Verify labels were added
	if cm.Labels[ConfigMapLabelKey] != ConfigMapLabelValue {
		t.Errorf("Label not updated: %s", cm.Labels[ConfigMapLabelKey])
	}
}

func TestPublishMetaInfoConfigMap(t *testing.T) {
	// Set environment variables
	os.Setenv(EnvReloaderNamespace, "test-ns")
	defer os.Unsetenv(EnvReloaderNamespace)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := config.NewDefault()
	ctx := context.Background()

	err := PublishMetaInfoConfigMap(ctx, fakeClient, cfg)
	if err != nil {
		t.Errorf("PublishMetaInfoConfigMap() error = %v", err)
	}

	// Verify ConfigMap was created
	cm := &corev1.ConfigMap{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: ConfigMapName, Namespace: "test-ns"}, cm)
	if err != nil {
		t.Errorf("Failed to get created ConfigMap: %v", err)
	}
}

func TestParseUTCTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid RFC3339 time",
			input:   "2024-01-01T12:00:00Z",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true, // returns zero time
		},
		{
			name:    "invalid format",
			input:   "not-a-time",
			wantErr: true, // returns zero time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseUTCTime(tt.input)
			if tt.wantErr {
				if !result.IsZero() {
					t.Errorf("parseUTCTime(%s) should return zero time", tt.input)
				}
			} else {
				if result.IsZero() {
					t.Errorf("parseUTCTime(%s) should not return zero time", tt.input)
				}
			}
		})
	}
}
