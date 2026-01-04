// Package metadata provides metadata ConfigMap creation for Reloader.
// The metadata ConfigMap contains build info, configuration options, and deployment info.
package metadata

import (
	"encoding/json"
	"os"
	"runtime"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConfigMapName is the name of the metadata ConfigMap.
	ConfigMapName = "reloader-meta-info"
	// ConfigMapLabelKey is the label key for the metadata ConfigMap.
	ConfigMapLabelKey = "reloader.stakater.com/meta-info"
	// ConfigMapLabelValue is the label value for the metadata ConfigMap.
	ConfigMapLabelValue = "reloader-oss"

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
	// Config contains all the configuration options used by this Reloader instance.
	Config *config.Config `json:"config"`
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

// NewBuildInfo creates a new BuildInfo with current build information.
func NewBuildInfo() BuildInfo {
	return BuildInfo{
		GoVersion:      runtime.Version(),
		ReleaseVersion: Version,
		CommitHash:     Commit,
		CommitTime:     parseUTCTime(BuildDate),
	}
}

// NewMetaInfo creates a new MetaInfo from configuration.
func NewMetaInfo(cfg *config.Config) *MetaInfo {
	return &MetaInfo{
		BuildInfo: NewBuildInfo(),
		Config:    cfg,
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
			"buildInfo":      toJSON(m.BuildInfo),
			"config":         toJSON(m.Config),
			"deploymentInfo": toJSON(m.DeploymentInfo),
		},
	}
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
