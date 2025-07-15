package util

import (
	"runtime/debug"

	"github.com/stakater/Reloader/internal/pkg/options"
)

type ReloaderOptions struct {
	AutoReloadAll                      bool   `json:"autoReloadAll"`
	ConfigmapUpdateOnChangeAnnotation  string `json:"configmapUpdateOnChangeAnnotation"`
	SecretUpdateOnChangeAnnotation     string `json:"secretUpdateOnChangeAnnotation"`
	ReloaderAutoAnnotation             string `json:"reloaderAutoAnnotation"`
	IgnoreResourceAnnotation           string `json:"ignoreResourceAnnotation"`
	ConfigmapReloaderAutoAnnotation    string `json:"configmapReloaderAutoAnnotation"`
	SecretReloaderAutoAnnotation       string `json:"secretReloaderAutoAnnotation"`
	ConfigmapExcludeReloaderAnnotation string `json:"configmapExcludeReloaderAnnotation"`
	SecretExcludeReloaderAnnotation    string `json:"secretExcludeReloaderAnnotation"`
	AutoSearchAnnotation               string `json:"autoSearchAnnotation"`
	SearchMatchAnnotation              string `json:"searchMatchAnnotation"`
	RolloutStrategyAnnotation          string `json:"rolloutStrategyAnnotation"`
	LogFormat                          string `json:"logFormat"`
	LogLevel                           string `json:"logLevel"`
	IsArgoRollouts                     string `json:"isArgoRollouts"`
	ReloadStrategy                     string `json:"reloadStrategy"`
	ReloadOnCreate                     string `json:"reloadOnCreate"`
	ReloadOnDelete                     string `json:"reloadOnDelete"`
	SyncAfterRestart                   bool   `json:"syncAfterRestart"`
	EnableHA                           bool   `json:"enableHA"`
	WebhookUrl                         string `json:"webhookUrl"`
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
		IsArgoRollouts:                     options.IsArgoRollouts,
		ReloadStrategy:                     options.ReloadStrategy,
		ReloadOnCreate:                     options.ReloadOnCreate,
		ReloadOnDelete:                     options.ReloadOnDelete,
		SyncAfterRestart:                   options.SyncAfterRestart,
		EnableHA:                           options.EnableHA,
		WebhookUrl:                         options.WebhookUrl,
	}
}

type BuildInfo struct {
	GoVersion   string `json:"goversion"`
	Version     string `json:"version"`
	Checksum    string `json:"checksum"`
	VCSRevision string `json:"vcs.revision,omitempty"`
	VCSModified string `json:"vcs.modified,omitempty"`
	VCSTime     string `json:"vcs.time,omitempty"`
}

func parseBuildInfo(info *debug.BuildInfo) *BuildInfo {
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
		GoVersion:   info.GoVersion,
		Version:     info.Main.Version,
		Checksum:    info.Main.Sum,
		VCSRevision: infoMap["vcs.revision"],
		VCSModified: infoMap["vcs.modified"],
		VCSTime:     infoMap["vcs.time"],
	}

	return metaInfo
}
