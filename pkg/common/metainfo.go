package common

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Version, Commit, and BuildDate are set during the build process
// using the -X linker flag to inject these values into the binary.
// They provide metadata about the build version, commit hash, build date, and whether there are
// uncommitted changes in the source code at the time of build.
// This information is useful for debugging and tracking the specific build of the Reloader binary.
var Version = "dev"
var Commit = "unknown"
var BuildDate = "unknown"
var Edition = "oss"

const (
	MetaInfoConfigmapName       = "reloader-meta-info"
	MetaInfoConfigmapLabelKey   = "reloader.stakater.com/meta-info"
	MetaInfoConfigmapLabelValue = "reloader"
)

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

// BuildInfo contains information about the build and version of the Reloader binary.
// This includes Go version, release version, commit details, and build timestamp.
type BuildInfo struct {
	// GoVersion is the version of Go used to compile the binary
	GoVersion string `json:"goVersion"`
	// ReleaseVersion is the version tag or branch of the Reloader release
	ReleaseVersion string `json:"releaseVersion"`
	// CommitHash is the Git commit hash of the source code used to build this binary
	CommitHash string `json:"commitHash"`
	// CommitTime is the timestamp of the Git commit used to build this binary
	CommitTime time.Time `json:"commitTime"`

	// Edition indicates the edition of Reloader (e.g., OSS, Enterprise)
	Edition string `json:"edition"`
}

func NewBuildInfo() *BuildInfo {
	metaInfo := &BuildInfo{
		GoVersion:      runtime.Version(),
		ReleaseVersion: Version,
		CommitHash:     Commit,
		CommitTime:     ParseUTCTime(BuildDate),
		Edition:        Edition,
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
