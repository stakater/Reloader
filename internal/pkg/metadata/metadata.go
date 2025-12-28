// Package metadata provides metadata ConfigMap creation for Reloader.
// The metadata ConfigMap contains build info, configuration options, and deployment info.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ConfigMapName is the name of the metadata ConfigMap.
	ConfigMapName = "reloader-meta-info"
	// ConfigMapLabelKey is the label key for the metadata ConfigMap.
	ConfigMapLabelKey = "reloader.stakater.com/meta-info"
	// ConfigMapLabelValue is the label value for the metadata ConfigMap.
	ConfigMapLabelValue = "reloader-oss"
	// FieldManager is the field manager name for server-side apply.
	FieldManager = "reloader"

	// Environment variables for deployment info.
	EnvReloaderNamespace      = "RELOADER_NAMESPACE"
	EnvReloaderDeploymentName = "RELOADER_DEPLOYMENT_NAME"
)

// Version, Commit, and BuildDate are set during the build process
// using the -X linker flag to inject these values into the binary.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// MetaInfo contains comprehensive metadata about the Reloader instance.
type MetaInfo struct {
	// BuildInfo contains information about the build version, commit, and compilation details.
	BuildInfo BuildInfo `json:"buildInfo"`
	// ReloaderOptions contains all the configuration options used by this Reloader instance.
	ReloaderOptions ReloaderOptions `json:"reloaderOptions"`
	// DeploymentInfo contains metadata about the Kubernetes deployment of this instance.
	DeploymentInfo DeploymentInfo `json:"deploymentInfo"`
}

// BuildInfo contains information about the build and version of the Reloader binary.
type BuildInfo struct {
	// GoVersion is the version of Go used to compile the binary.
	GoVersion string `json:"goVersion"`
	// ReleaseVersion is the version tag or branch of the Reloader release.
	ReleaseVersion string `json:"releaseVersion"`
	// CommitHash is the Git commit hash of the source code used to build this binary.
	CommitHash string `json:"commitHash"`
	// CommitTime is the timestamp of the Git commit used to build this binary.
	CommitTime time.Time `json:"commitTime"`
}

// DeploymentInfo contains metadata about the Reloader deployment.
type DeploymentInfo struct {
	// Name is the name of the Reloader deployment.
	Name string `json:"name"`
	// Namespace is the namespace where Reloader is deployed.
	Namespace string `json:"namespace"`
}

// ReloaderOptions contains the configuration options for Reloader.
// This is a subset of config.Config that's relevant for the metadata ConfigMap.
type ReloaderOptions struct {
	// AutoReloadAll enables automatic reloading of all resources.
	AutoReloadAll bool `json:"autoReloadAll"`
	// ReloadStrategy specifies the strategy used to trigger resource reloads.
	ReloadStrategy string `json:"reloadStrategy"`
	// IsArgoRollouts indicates whether support for Argo Rollouts is enabled.
	IsArgoRollouts bool `json:"isArgoRollouts"`
	// ReloadOnCreate indicates whether to trigger reloads when resources are created.
	ReloadOnCreate bool `json:"reloadOnCreate"`
	// ReloadOnDelete indicates whether to trigger reloads when resources are deleted.
	ReloadOnDelete bool `json:"reloadOnDelete"`
	// SyncAfterRestart indicates whether to sync add events after Reloader restarts.
	SyncAfterRestart bool `json:"syncAfterRestart"`
	// EnableHA indicates whether High Availability mode is enabled.
	EnableHA bool `json:"enableHA"`
	// WebhookURL is the URL to send webhook notifications to.
	WebhookURL string `json:"webhookUrl"`
	// LogFormat specifies the log format to use.
	LogFormat string `json:"logFormat"`
	// LogLevel specifies the log level to use.
	LogLevel string `json:"logLevel"`
	// ResourcesToIgnore is a list of resource types to ignore.
	ResourcesToIgnore []string `json:"resourcesToIgnore"`
	// WorkloadTypesToIgnore is a list of workload types to ignore.
	WorkloadTypesToIgnore []string `json:"workloadTypesToIgnore"`
	// NamespacesToIgnore is a list of namespaces to ignore.
	NamespacesToIgnore []string `json:"namespacesToIgnore"`
	// NamespaceSelectors is a list of namespace label selectors.
	NamespaceSelectors []string `json:"namespaceSelectors"`
	// ResourceSelectors is a list of resource label selectors.
	ResourceSelectors []string `json:"resourceSelectors"`

	// Annotations
	ConfigmapUpdateOnChangeAnnotation  string `json:"configmapUpdateOnChangeAnnotation"`
	SecretUpdateOnChangeAnnotation     string `json:"secretUpdateOnChangeAnnotation"`
	ReloaderAutoAnnotation             string `json:"reloaderAutoAnnotation"`
	ConfigmapReloaderAutoAnnotation    string `json:"configmapReloaderAutoAnnotation"`
	SecretReloaderAutoAnnotation       string `json:"secretReloaderAutoAnnotation"`
	IgnoreResourceAnnotation           string `json:"ignoreResourceAnnotation"`
	ConfigmapExcludeReloaderAnnotation string `json:"configmapExcludeReloaderAnnotation"`
	SecretExcludeReloaderAnnotation    string `json:"secretExcludeReloaderAnnotation"`
	AutoSearchAnnotation               string `json:"autoSearchAnnotation"`
	SearchMatchAnnotation              string `json:"searchMatchAnnotation"`
	RolloutStrategyAnnotation          string `json:"rolloutStrategyAnnotation"`
	PauseDeploymentAnnotation          string `json:"pauseDeploymentAnnotation"`
	PauseDeploymentTimeAnnotation      string `json:"pauseDeploymentTimeAnnotation"`
}

// NewBuildInfo creates a new BuildInfo with current build information.
func NewBuildInfo() BuildInfo {
	return BuildInfo{
		GoVersion:      runtime.Version(),
		ReleaseVersion: Version,
		CommitHash:     Commit,
		CommitTime:     parseUTCTime(BuildDate),
	}
}

// NewReloaderOptions creates ReloaderOptions from a Config.
func NewReloaderOptions(cfg *config.Config) ReloaderOptions {
	return ReloaderOptions{
		AutoReloadAll:                      cfg.AutoReloadAll,
		ReloadStrategy:                     string(cfg.ReloadStrategy),
		IsArgoRollouts:                     cfg.ArgoRolloutsEnabled,
		ReloadOnCreate:                     cfg.ReloadOnCreate,
		ReloadOnDelete:                     cfg.ReloadOnDelete,
		SyncAfterRestart:                   cfg.SyncAfterRestart,
		EnableHA:                           cfg.EnableHA,
		WebhookURL:                         cfg.WebhookURL,
		LogFormat:                          cfg.LogFormat,
		LogLevel:                           cfg.LogLevel,
		ResourcesToIgnore:                  cfg.IgnoredResources,
		WorkloadTypesToIgnore:              cfg.IgnoredWorkloads,
		NamespacesToIgnore:                 cfg.IgnoredNamespaces,
		NamespaceSelectors:                 cfg.NamespaceSelectorStrings,
		ResourceSelectors:                  cfg.ResourceSelectorStrings,
		ConfigmapUpdateOnChangeAnnotation:  cfg.Annotations.ConfigmapReload,
		SecretUpdateOnChangeAnnotation:     cfg.Annotations.SecretReload,
		ReloaderAutoAnnotation:             cfg.Annotations.Auto,
		ConfigmapReloaderAutoAnnotation:    cfg.Annotations.ConfigmapAuto,
		SecretReloaderAutoAnnotation:       cfg.Annotations.SecretAuto,
		IgnoreResourceAnnotation:           cfg.Annotations.Ignore,
		ConfigmapExcludeReloaderAnnotation: cfg.Annotations.ConfigmapExclude,
		SecretExcludeReloaderAnnotation:    cfg.Annotations.SecretExclude,
		AutoSearchAnnotation:               cfg.Annotations.Search,
		SearchMatchAnnotation:              cfg.Annotations.Match,
		RolloutStrategyAnnotation:          cfg.Annotations.RolloutStrategy,
		PauseDeploymentAnnotation:          cfg.Annotations.PausePeriod,
		PauseDeploymentTimeAnnotation:      cfg.Annotations.PausedAt,
	}
}

// NewMetaInfo creates a new MetaInfo from configuration.
func NewMetaInfo(cfg *config.Config) *MetaInfo {
	return &MetaInfo{
		BuildInfo:       NewBuildInfo(),
		ReloaderOptions: NewReloaderOptions(cfg),
		DeploymentInfo: DeploymentInfo{
			Name:      os.Getenv(EnvReloaderDeploymentName),
			Namespace: os.Getenv(EnvReloaderNamespace),
		},
	}
}

// ToConfigMap converts MetaInfo to a Kubernetes ConfigMap.
func (m *MetaInfo) ToConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: m.DeploymentInfo.Namespace,
			Labels: map[string]string{
				ConfigMapLabelKey: ConfigMapLabelValue,
			},
		},
		Data: map[string]string{
			"buildInfo":       toJSON(m.BuildInfo),
			"reloaderOptions": toJSON(m.ReloaderOptions),
			"deploymentInfo":  toJSON(m.DeploymentInfo),
		},
	}
}

// Publisher handles creating and updating the metadata ConfigMap.
type Publisher struct {
	client client.Client
	cfg    *config.Config
	log    logr.Logger
}

// NewPublisher creates a new Publisher.
func NewPublisher(c client.Client, cfg *config.Config, log logr.Logger) *Publisher {
	return &Publisher{
		client: c,
		cfg:    cfg,
		log:    log,
	}
}

// Publish creates or updates the metadata ConfigMap.
func (p *Publisher) Publish(ctx context.Context) error {
	namespace := os.Getenv(EnvReloaderNamespace)
	if namespace == "" {
		p.log.Info("RELOADER_NAMESPACE is not set, skipping meta info configmap creation")
		return nil
	}

	metaInfo := NewMetaInfo(p.cfg)
	configMap := metaInfo.ToConfigMap()

	existing := &corev1.ConfigMap{}
	err := p.client.Get(ctx, client.ObjectKey{
		Name:      ConfigMapName,
		Namespace: namespace,
	}, existing)

	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing meta info configmap: %w", err)
		}
		p.log.Info("Creating meta info configmap")
		if err := p.client.Create(ctx, configMap, client.FieldOwner(FieldManager)); err != nil {
			return fmt.Errorf("failed to create meta info configmap: %w", err)
		}
		p.log.Info("Meta info configmap created successfully")
		return nil
	}

	p.log.Info("Meta info configmap already exists, updating it")
	existing.Data = configMap.Data
	existing.Labels = configMap.Labels
	if err := p.client.Update(ctx, existing, client.FieldOwner(FieldManager)); err != nil {
		return fmt.Errorf("failed to update meta info configmap: %w", err)
	}
	p.log.Info("Meta info configmap updated successfully")
	return nil
}

// PublishMetaInfoConfigMap is a convenience function that creates a Publisher and calls Publish.
func PublishMetaInfoConfigMap(ctx context.Context, c client.Client, cfg *config.Config, log logr.Logger) error {
	publisher := NewPublisher(c, cfg, log)
	return publisher.Publish(ctx)
}

// CreateOrUpdate creates or updates the metadata ConfigMap using the provided client.
func CreateOrUpdate(c client.Client, cfg *config.Config, log logr.Logger) error {
	ctx := context.Background()
	return PublishMetaInfoConfigMap(ctx, c, cfg, log)
}

func toJSON(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(jsonData)
}

func parseUTCTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
