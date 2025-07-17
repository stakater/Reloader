package metainfo

import (
	"encoding/json"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/stakater/Reloader/internal/pkg/options"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetaInfoConfigmapName       = "reloader-meta-info"
	MetaInfoConfigmapLabel      = "reloader.stakater.com/meta-info"
	MetaInfoConfigmapLabelValue = "reloader-oss"
)

type ReloaderOptions struct {
	AutoReloadAll                      bool     `json:"autoReloadAll"`
	ConfigmapUpdateOnChangeAnnotation  string   `json:"configmapUpdateOnChangeAnnotation"`
	SecretUpdateOnChangeAnnotation     string   `json:"secretUpdateOnChangeAnnotation"`
	ReloaderAutoAnnotation             string   `json:"reloaderAutoAnnotation"`
	IgnoreResourceAnnotation           string   `json:"ignoreResourceAnnotation"`
	ConfigmapReloaderAutoAnnotation    string   `json:"configmapReloaderAutoAnnotation"`
	SecretReloaderAutoAnnotation       string   `json:"secretReloaderAutoAnnotation"`
	ConfigmapExcludeReloaderAnnotation string   `json:"configmapExcludeReloaderAnnotation"`
	SecretExcludeReloaderAnnotation    string   `json:"secretExcludeReloaderAnnotation"`
	AutoSearchAnnotation               string   `json:"autoSearchAnnotation"`
	SearchMatchAnnotation              string   `json:"searchMatchAnnotation"`
	RolloutStrategyAnnotation          string   `json:"rolloutStrategyAnnotation"`
	LogFormat                          string   `json:"logFormat"`
	LogLevel                           string   `json:"logLevel"`
	IsArgoRollouts                     bool     `json:"isArgoRollouts"`
	ReloadStrategy                     string   `json:"reloadStrategy"`
	ReloadOnCreate                     bool     `json:"reloadOnCreate"`
	ReloadOnDelete                     bool     `json:"reloadOnDelete"`
	SyncAfterRestart                   bool     `json:"syncAfterRestart"`
	EnableHA                           bool     `json:"enableHA"`
	WebhookUrl                         string   `json:"webhookUrl"`
	ResourcesToIgnore                  []string `json:"resourcesToIgnore"`
	NamespaceSelectors                 []string `json:"namespaceSelectors"`
	ResourceSelectors                  []string `json:"resourceSelectors"`
	NamespacesToIgnore                 []string `json:"namespacesToIgnore"`
}

type MetaInfo struct {
	BuildInfo       BuildInfo         `json:"buildInfo"`
	ReloaderOptions ReloaderOptions   `json:"reloaderOptions"`
	DeploymentInfo  metav1.ObjectMeta `json:"deploymentInfo"`
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

type BuildInfo struct {
	GoVersion  string    `json:"goversion"`
	Version    string    `json:"version"`
	Checksum   string    `json:"checksum"`
	CommitHash string    `json:"commitHash"`
	IsDirty    bool      `json:"isDirty"`
	CommitTime time.Time `json:"commitTime"`
}

func NewBuildInfo(info *debug.BuildInfo) *BuildInfo {
	infoMap := make(map[string]string)
	infoMap["goversion"] = info.GoVersion
	infoMap["version"] = info.Main.Version
	infoMap["checksum"] = info.Main.Sum

	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" || setting.Key == "vcs.time" || setting.Key == "vcs.modified" {
			infoMap[setting.Key] = setting.Value
		}
	}

	metaInfo := &BuildInfo{
		GoVersion:  info.GoVersion,
		Version:    info.Main.Version,
		Checksum:   info.Main.Sum,
		CommitHash: infoMap["vcs.revision"],
		IsDirty:    parseBool(infoMap["vcs.modified"]),
		CommitTime: parseTime(infoMap["vcs.time"]),
	}

	return metaInfo
}

func (m *MetaInfo) ToConfigMap() *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetaInfoConfigmapName,
			Namespace: m.DeploymentInfo.Namespace,
			Labels: map[string]string{
				MetaInfoConfigmapLabel: MetaInfoConfigmapLabelValue,
			},
		},
		Data: map[string]string{
			"buildInfo":       toJson(m.BuildInfo),
			"reloaderOptions": toJson(m.ReloaderOptions),
			"deploymentInfo":  toJson(m.DeploymentInfo),
		},
	}
}

func NewMetaInfo(configmap *v1.ConfigMap) *MetaInfo {
	var buildInfo BuildInfo
	if val, ok := configmap.Data["buildInfo"]; ok {
		_ = json.Unmarshal([]byte(val), &buildInfo)
	}

	var reloaderOptions ReloaderOptions
	if val, ok := configmap.Data["reloaderOptions"]; ok {
		_ = json.Unmarshal([]byte(val), &reloaderOptions)
	}

	var deploymentInfo metav1.ObjectMeta
	if val, ok := configmap.Data["deploymentInfo"]; ok {
		_ = json.Unmarshal([]byte(val), &deploymentInfo)
	}

	return &MetaInfo{
		BuildInfo:       buildInfo,
		ReloaderOptions: reloaderOptions,
		DeploymentInfo:  deploymentInfo,
	}
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

func ParseTime(value string) time.Time {
	if value == "" {
		return time.Time{} // Return zero time if value is empty
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{} // Return zero time if parsing fails
	}
	return t
}
