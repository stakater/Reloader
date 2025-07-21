package metainfo

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/stakater/Reloader/internal/pkg/options"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Version, Commit, BuildDate, and IsDirty are set during the build process
// using the -X linker flag to inject these values into the binary.
// They provide metadata about the build version, commit hash, build date, and whether there are
// uncommitted changes in the source code at the time of build.
// This information is useful for debugging and tracking the specific build of the Reloader binary.
var Version = "dev"
var Commit = "unknown"
var BuildDate = "unknown"
var IsDirty = "false"

const (
	MetaInfoConfigmapName       = "reloader-meta-info"
	MetaInfoConfigmapLabelKey   = "reloader.stakater.com/meta-info"
	MetaInfoConfigmapLabelValue = "reloader-oss"
)

// ReloaderOptions contains all configurable options for the Reloader controller.
// These options control how Reloader behaves when watching for changes in ConfigMaps and Secrets.
type ReloaderOptions struct {
	// AutoReloadAll enables automatic reloading of all resources when their corresponding ConfigMaps/Secrets are updated
	AutoReloadAll bool `json:"autoReloadAll"`
	// ConfigmapUpdateOnChangeAnnotation is the annotation key used to detect changes in ConfigMaps specified by name
	ConfigmapUpdateOnChangeAnnotation string `json:"configmapUpdateOnChangeAnnotation"`
	// SecretUpdateOnChangeAnnotation is the annotation key used to detect changes in Secrets specified by name
	SecretUpdateOnChangeAnnotation string `json:"secretUpdateOnChangeAnnotation"`
	// ReloaderAutoAnnotation is the annotation key used to detect changes in any referenced ConfigMaps or Secrets
	ReloaderAutoAnnotation string `json:"reloaderAutoAnnotation"`
	// IgnoreResourceAnnotation is the annotation key used to ignore resources from being watched
	IgnoreResourceAnnotation string `json:"ignoreResourceAnnotation"`
	// ConfigmapReloaderAutoAnnotation is the annotation key used to detect changes in ConfigMaps only
	ConfigmapReloaderAutoAnnotation string `json:"configmapReloaderAutoAnnotation"`
	// SecretReloaderAutoAnnotation is the annotation key used to detect changes in Secrets only
	SecretReloaderAutoAnnotation string `json:"secretReloaderAutoAnnotation"`
	// ConfigmapExcludeReloaderAnnotation is the annotation key containing comma-separated list of ConfigMaps to exclude from watching
	ConfigmapExcludeReloaderAnnotation string `json:"configmapExcludeReloaderAnnotation"`
	// SecretExcludeReloaderAnnotation is the annotation key containing comma-separated list of Secrets to exclude from watching
	SecretExcludeReloaderAnnotation string `json:"secretExcludeReloaderAnnotation"`
	// AutoSearchAnnotation is the annotation key used to detect changes in ConfigMaps/Secrets tagged with SearchMatchAnnotation
	AutoSearchAnnotation string `json:"autoSearchAnnotation"`
	// SearchMatchAnnotation is the annotation key used to tag ConfigMaps/Secrets to be found by AutoSearchAnnotation
	SearchMatchAnnotation string `json:"searchMatchAnnotation"`
	// RolloutStrategyAnnotation is the annotation key used to define the rollout update strategy for workloads
	RolloutStrategyAnnotation string `json:"rolloutStrategyAnnotation"`
	// PauseDeploymentAnnotation is the annotation key used to define the time period to pause a deployment after
	PauseDeploymentAnnotation string `json:"pauseDeploymentAnnotation"`
	// PauseDeploymentTimeAnnotation is the annotation key used to indicate when a deployment was paused by Reloader
	PauseDeploymentTimeAnnotation string `json:"pauseDeploymentTimeAnnotation"`

	// LogFormat specifies the log format to use (json, or empty string for default text format)
	LogFormat string `json:"logFormat"`
	// LogLevel specifies the log level to use (trace, debug, info, warning, error, fatal, panic)
	LogLevel string `json:"logLevel"`
	// IsArgoRollouts indicates whether support for Argo Rollouts is enabled
	IsArgoRollouts bool `json:"isArgoRollouts"`
	// ReloadStrategy specifies the strategy used to trigger resource reloads (env-vars or annotations)
	ReloadStrategy string `json:"reloadStrategy"`
	// ReloadOnCreate indicates whether to trigger reloads when ConfigMaps/Secrets are created
	ReloadOnCreate bool `json:"reloadOnCreate"`
	// ReloadOnDelete indicates whether to trigger reloads when ConfigMaps/Secrets are deleted
	ReloadOnDelete bool `json:"reloadOnDelete"`
	// SyncAfterRestart indicates whether to sync add events after Reloader restarts (only works when ReloadOnCreate is true)
	SyncAfterRestart bool `json:"syncAfterRestart"`
	// EnableHA indicates whether High Availability mode is enabled with leader election
	EnableHA bool `json:"enableHA"`
	// WebhookUrl is the URL to send webhook notifications to instead of performing reloads
	WebhookUrl string `json:"webhookUrl"`
	// ResourcesToIgnore is a list of resource types to ignore (e.g., "configmaps" or "secrets")
	ResourcesToIgnore []string `json:"resourcesToIgnore"`
	// NamespaceSelectors is a list of label selectors to filter namespaces to watch
	NamespaceSelectors []string `json:"namespaceSelectors"`
	// ResourceSelectors is a list of label selectors to filter ConfigMaps and Secrets to watch
	ResourceSelectors []string `json:"resourceSelectors"`
	// NamespacesToIgnore is a list of namespace names to ignore when watching for changes
	NamespacesToIgnore []string `json:"namespacesToIgnore"`
}

// MetaInfo contains comprehensive metadata about the Reloader instance.
// This includes build information, configuration options, and deployment details.
type MetaInfo struct {
	// BuildInfo contains information about the build version, commit, and compilation details
	BuildInfo BuildInfo `json:"buildInfo"`
	// ReloaderOptions contains all the configuration options and flags used by this Reloader instance
	ReloaderOptions ReloaderOptions `json:"reloaderOptions"`
	// DeploymentInfo contains metadata about the Kubernetes deployment of this Reloader instance
	DeploymentInfo metav1.ObjectMeta `json:"deploymentInfo"`
}

func GetReloaderOptions() *ReloaderOptions {
	return &ReloaderOptions{
		AutoReloadAll:                      options.AutoReloadAll,
		ConfigmapUpdateOnChangeAnnotation:  options.ConfigmapUpdateOnChangeAnnotation,
		SecretUpdateOnChangeAnnotation:     options.SecretUpdateOnChangeAnnotation,
		ReloaderAutoAnnotation:             options.ReloaderAutoAnnotation,
		IgnoreResourceAnnotation:           options.IgnoreResourceAnnotation,
		ConfigmapReloaderAutoAnnotation:    options.ConfigmapReloaderAutoAnnotation,
		SecretReloaderAutoAnnotation:       options.SecretReloaderAutoAnnotation,
		ConfigmapExcludeReloaderAnnotation: options.ConfigmapExcludeReloaderAnnotation,
		SecretExcludeReloaderAnnotation:    options.SecretExcludeReloaderAnnotation,
		AutoSearchAnnotation:               options.AutoSearchAnnotation,
		SearchMatchAnnotation:              options.SearchMatchAnnotation,
		RolloutStrategyAnnotation:          options.RolloutStrategyAnnotation,
		PauseDeploymentAnnotation:          options.PauseDeploymentAnnotation,
		PauseDeploymentTimeAnnotation:      options.PauseDeploymentTimeAnnotation,
		LogFormat:                          options.LogFormat,
		LogLevel:                           options.LogLevel,
		IsArgoRollouts:                     parseBool(options.IsArgoRollouts),
		ReloadStrategy:                     options.ReloadStrategy,
		ReloadOnCreate:                     parseBool(options.ReloadOnCreate),
		ReloadOnDelete:                     parseBool(options.ReloadOnDelete),
		SyncAfterRestart:                   options.SyncAfterRestart,
		EnableHA:                           options.EnableHA,
		WebhookUrl:                         options.WebhookUrl,
		ResourcesToIgnore:                  options.ResourcesToIgnore,
		NamespaceSelectors:                 options.NamespaceSelectors,
		ResourceSelectors:                  options.ResourceSelectors,
		NamespacesToIgnore:                 options.NamespacesToIgnore,
	}
}

// BuildInfo contains information about the build and version of the Reloader binary.
// This includes Go version, release version, commit details, and build timestamp.
type BuildInfo struct {
	// GoVersion is the version of Go used to compile the binary
	GoVersion string `json:"goVersion"`
	// ReleaseVersion is the version tag or branch of the Reloader release
	ReleaseVersion string `json:"releaseVersion"`
	// CommitHash is the Git commit hash of the source code used to build this binary
	CommitHash string `json:"commitHash"`
	// IsDirty indicates whether the working directory had uncommitted changes when built
	IsDirty bool `json:"isDirty"`
	// CommitTime is the timestamp of the Git commit used to build this binary
	CommitTime time.Time `json:"commitTime"`
}

func NewBuildInfo() *BuildInfo {
	metaInfo := &BuildInfo{
		GoVersion:      runtime.Version(),
		ReleaseVersion: Version,
		CommitHash:     Commit,
		IsDirty:        parseBool(IsDirty),
		CommitTime:     ParseUTCTime(BuildDate),
	}

	return metaInfo
}

func (m *MetaInfo) ToConfigMap() *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetaInfoConfigmapName,
			Namespace: m.DeploymentInfo.Namespace,
			Labels: map[string]string{
				MetaInfoConfigmapLabelKey: MetaInfoConfigmapLabelValue,
			},
		},
		Data: map[string]string{
			"buildInfo":       toJson(m.BuildInfo),
			"reloaderOptions": toJson(m.ReloaderOptions),
			"deploymentInfo":  toJson(m.DeploymentInfo),
		},
	}
}

func NewMetaInfo(configmap *v1.ConfigMap) (*MetaInfo, error) {
	var buildInfo BuildInfo
	if val, ok := configmap.Data["buildInfo"]; ok {
		err := json.Unmarshal([]byte(val), &buildInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal buildInfo: %w", err)
		}
	}

	var reloaderOptions ReloaderOptions
	if val, ok := configmap.Data["reloaderOptions"]; ok {
		err := json.Unmarshal([]byte(val), &reloaderOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal reloaderOptions: %w", err)
		}
	}

	var deploymentInfo metav1.ObjectMeta
	if val, ok := configmap.Data["deploymentInfo"]; ok {
		err := json.Unmarshal([]byte(val), &deploymentInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal deploymentInfo: %w", err)
		}
	}

	return &MetaInfo{
		BuildInfo:       buildInfo,
		ReloaderOptions: reloaderOptions,
		DeploymentInfo:  deploymentInfo,
	}, nil
}

func toJson(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(jsonData)
}

func parseBool(value string) bool {
	if value == "" {
		return false
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return false // Default to false if parsing fails
	}
	return result
}

func ParseUTCTime(value string) time.Time {
	if value == "" {
		return time.Time{} // Return zero time if value is empty
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{} // Return zero time if parsing fails
	}
	return t
}
