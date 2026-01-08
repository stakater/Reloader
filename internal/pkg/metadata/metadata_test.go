package metadata

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stakater/Reloader/internal/pkg/config"
)

// testLogger returns a no-op logger for testing.
func testLogger() logr.Logger {
	return logr.Discard()
}

func TestNewBuildInfo(t *testing.T) {
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

func TestNewMetaInfo(t *testing.T) {
	t.Setenv(EnvReloaderNamespace, "test-ns")
	t.Setenv(EnvReloaderDeploymentName, "test-deploy")

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

	metaInfo := NewMetaInfo(cfg)

	if !metaInfo.Config.AutoReloadAll {
		t.Error("AutoReloadAll should be true")
	}
	if metaInfo.Config.ReloadStrategy != config.ReloadStrategyAnnotations {
		t.Errorf("ReloadStrategy = %s, want annotations", metaInfo.Config.ReloadStrategy)
	}
	if !metaInfo.Config.ArgoRolloutsEnabled {
		t.Error("ArgoRolloutsEnabled should be true")
	}
	if !metaInfo.Config.ReloadOnCreate {
		t.Error("ReloadOnCreate should be true")
	}
	if !metaInfo.Config.ReloadOnDelete {
		t.Error("ReloadOnDelete should be true")
	}
	if !metaInfo.Config.EnableHA {
		t.Error("EnableHA should be true")
	}
	if metaInfo.Config.WebhookURL != "https://example.com/webhook" {
		t.Errorf("WebhookURL = %s, want https://example.com/webhook", metaInfo.Config.WebhookURL)
	}

	if metaInfo.DeploymentInfo.Namespace != "test-ns" {
		t.Errorf("DeploymentInfo.Namespace = %s, want test-ns", metaInfo.DeploymentInfo.Namespace)
	}
	if metaInfo.DeploymentInfo.Name != "test-deploy" {
		t.Errorf("DeploymentInfo.Name = %s, want test-deploy", metaInfo.DeploymentInfo.Name)
	}
}

func TestMetaInfo_ToConfigMap(t *testing.T) {
	t.Setenv(EnvReloaderNamespace, "reloader-ns")
	t.Setenv(EnvReloaderDeploymentName, "reloader-deploy")

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

	if _, ok := cm.Data["buildInfo"]; !ok {
		t.Error("buildInfo data key missing")
	}
	if _, ok := cm.Data["config"]; !ok {
		t.Error("config data key missing")
	}
	if _, ok := cm.Data["deploymentInfo"]; !ok {
		t.Error("deploymentInfo data key missing")
	}

	// Verify buildInfo is valid JSON
	var buildInfo BuildInfo
	if err := json.Unmarshal([]byte(cm.Data["buildInfo"]), &buildInfo); err != nil {
		t.Errorf("buildInfo is not valid JSON: %v", err)
	}

	var parsedConfig config.Config
	if err := json.Unmarshal([]byte(cm.Data["config"]), &parsedConfig); err != nil {
		t.Errorf("config is not valid JSON: %v", err)
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
	t.Setenv(EnvReloaderNamespace, "")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := config.NewDefault()
	publisher := NewPublisher(fakeClient, cfg, testLogger())

	err := publisher.Publish(context.Background())
	if err != nil {
		t.Errorf("Publish() with no namespace should not error, got: %v", err)
	}
}

func TestPublisher_Publish_CreateNew(t *testing.T) {
	t.Setenv(EnvReloaderNamespace, "test-ns")
	t.Setenv(EnvReloaderDeploymentName, "test-deploy")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := config.NewDefault()
	publisher := NewPublisher(fakeClient, cfg, testLogger())

	ctx := context.Background()
	err := publisher.Publish(ctx)
	if err != nil {
		t.Errorf("Publish() error = %v", err)
	}

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
	t.Setenv(EnvReloaderNamespace, "test-ns")
	t.Setenv(EnvReloaderDeploymentName, "test-deploy")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

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
	publisher := NewPublisher(fakeClient, cfg, testLogger())

	ctx := context.Background()
	err := publisher.Publish(ctx)
	if err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	cm := &corev1.ConfigMap{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: ConfigMapName, Namespace: "test-ns"}, cm)
	if err != nil {
		t.Errorf("Failed to get updated ConfigMap: %v", err)
	}

	if _, ok := cm.Data["buildInfo"]; !ok {
		t.Error("buildInfo data key missing after update")
	}
	if _, ok := cm.Data["config"]; !ok {
		t.Error("config data key missing after update")
	}
	if _, ok := cm.Data["deploymentInfo"]; !ok {
		t.Error("deploymentInfo data key missing after update")
	}

	if cm.Labels[ConfigMapLabelKey] != ConfigMapLabelValue {
		t.Errorf("Label not updated: %s", cm.Labels[ConfigMapLabelKey])
	}
}

func TestPublishMetaInfoConfigMap(t *testing.T) {
	t.Setenv(EnvReloaderNamespace, "test-ns")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := config.NewDefault()
	ctx := context.Background()

	err := PublishMetaInfoConfigMap(ctx, fakeClient, cfg, testLogger())
	if err != nil {
		t.Errorf("PublishMetaInfoConfigMap() error = %v", err)
	}

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
		t.Run(
			tt.name, func(t *testing.T) {
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
			},
		)
	}
}
