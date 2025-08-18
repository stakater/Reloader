package common

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type Map map[string]string

type ReloadCheckResult struct {
	ShouldReload bool
	AutoReload   bool
}

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
	// WorkloadTypesToIgnore is a list of workload types to ignore (e.g., "jobs" or "cronjobs")
	WorkloadTypesToIgnore []string `json:"workloadTypesToIgnore"`
	// NamespaceSelectors is a list of label selectors to filter namespaces to watch
	NamespaceSelectors []string `json:"namespaceSelectors"`
	// ResourceSelectors is a list of label selectors to filter ConfigMaps and Secrets to watch
	ResourceSelectors []string `json:"resourceSelectors"`
	// NamespacesToIgnore is a list of namespace names to ignore when watching for changes
	NamespacesToIgnore []string `json:"namespacesToIgnore"`
	// EnablePProf enables pprof for profiling
	EnablePProf bool `json:"enablePProf"`
	// PProfAddr is the address to start pprof server on
	PProfAddr string `json:"pprofAddr"`
}

var CommandLineOptions *ReloaderOptions

func PublishMetaInfoConfigmap(clientset kubernetes.Interface) {
	namespace := os.Getenv("RELOADER_NAMESPACE")
	if namespace == "" {
		logrus.Warn("RELOADER_NAMESPACE is not set, skipping meta info configmap creation")
		return
	}

	metaInfo := &MetaInfo{
		BuildInfo:       *NewBuildInfo(),
		ReloaderOptions: *GetCommandLineOptions(),
		DeploymentInfo: metav1.ObjectMeta{
			Name:      os.Getenv("RELOADER_DEPLOYMENT_NAME"),
			Namespace: namespace,
		},
	}

	configMap := metaInfo.ToConfigMap()

	if _, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMap.Name, metav1.GetOptions{}); err == nil {
		logrus.Info("Meta info configmap already exists, updating it")
		_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.Background(), configMap, metav1.UpdateOptions{})
		if err != nil {
			logrus.Warn("Failed to update existing meta info configmap: ", err)
		}
		return
	}

	_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		logrus.Warn("Failed to create meta info configmap: ", err)
	}
}

func GetNamespaceLabelSelector(slice []string) (string, error) {
	for i, kv := range slice {
		// Legacy support for ":" as a delimiter and "*" for wildcard.
		if strings.Contains(kv, ":") {
			split := strings.Split(kv, ":")
			if split[1] == "*" {
				slice[i] = split[0]
			} else {
				slice[i] = split[0] + "=" + split[1]
			}
		}
		// Convert wildcard to valid apimachinery operator
		if strings.Contains(kv, "=") {
			split := strings.Split(kv, "=")
			if split[1] == "*" {
				slice[i] = split[0]
			}
		}
	}

	namespaceLabelSelector := strings.Join(slice[:], ",")
	_, err := labels.Parse(namespaceLabelSelector)
	if err != nil {
		logrus.Fatal(err)
	}

	return namespaceLabelSelector, nil
}

func GetResourceLabelSelector(slice []string) (string, error) {
	for i, kv := range slice {
		// Legacy support for ":" as a delimiter and "*" for wildcard.
		if strings.Contains(kv, ":") {
			split := strings.Split(kv, ":")
			if split[1] == "*" {
				slice[i] = split[0]
			} else {
				slice[i] = split[0] + "=" + split[1]
			}
		}
		// Convert wildcard to valid apimachinery operator
		if strings.Contains(kv, "=") {
			split := strings.Split(kv, "=")
			if split[1] == "*" {
				slice[i] = split[0]
			}
		}
	}

	resourceLabelSelector := strings.Join(slice[:], ",")
	_, err := labels.Parse(resourceLabelSelector)
	if err != nil {
		logrus.Fatal(err)
	}

	return resourceLabelSelector, nil
}

// ShouldReload checks if a resource should be reloaded based on its annotations and the provided options.
func ShouldReload(config Config, resourceType string, annotations Map, podAnnotations Map, options *ReloaderOptions) ReloadCheckResult {

	// Check if this workload type should be ignored
	if len(options.WorkloadTypesToIgnore) > 0 {
		ignoredWorkloadTypes, err := util.GetIgnoredWorkloadTypesList()
		if err != nil {
			logrus.Errorf("Failed to parse ignored workload types: %v", err)
		} else {
			// Map Kubernetes resource types to CLI-friendly names for comparison
			var resourceToCheck string
			switch resourceType {
			case "Job":
				resourceToCheck = "jobs"
			case "CronJob":
				resourceToCheck = "cronjobs"
			default:
				resourceToCheck = resourceType // For other types, use as-is
			}

			// Check if current resource type should be ignored
			if ignoredWorkloadTypes.Contains(resourceToCheck) {
				return ReloadCheckResult{
					ShouldReload: false,
				}
			}
		}
	}

	ignoreResourceAnnotatonValue := config.ResourceAnnotations[options.IgnoreResourceAnnotation]
	if ignoreResourceAnnotatonValue == "true" {
		return ReloadCheckResult{
			ShouldReload: false,
		}
	}

	annotationValue, found := annotations[config.Annotation]
	searchAnnotationValue, foundSearchAnn := annotations[options.AutoSearchAnnotation]
	reloaderEnabledValue, foundAuto := annotations[options.ReloaderAutoAnnotation]
	typedAutoAnnotationEnabledValue, foundTypedAuto := annotations[config.TypedAutoAnnotation]
	excludeConfigmapAnnotationValue, foundExcludeConfigmap := annotations[options.ConfigmapExcludeReloaderAnnotation]
	excludeSecretAnnotationValue, foundExcludeSecret := annotations[options.SecretExcludeReloaderAnnotation]

	if !found && !foundAuto && !foundTypedAuto && !foundSearchAnn {
		annotations = podAnnotations
		annotationValue = annotations[config.Annotation]
		searchAnnotationValue = annotations[options.AutoSearchAnnotation]
		reloaderEnabledValue = annotations[options.ReloaderAutoAnnotation]
		typedAutoAnnotationEnabledValue = annotations[config.TypedAutoAnnotation]
	}

	isResourceExcluded := false

	switch config.Type {
	case constants.ConfigmapEnvVarPostfix:
		if foundExcludeConfigmap {
			isResourceExcluded = checkIfResourceIsExcluded(config.ResourceName, excludeConfigmapAnnotationValue)
		}
	case constants.SecretEnvVarPostfix:
		if foundExcludeSecret {
			isResourceExcluded = checkIfResourceIsExcluded(config.ResourceName, excludeSecretAnnotationValue)
		}
	}

	if isResourceExcluded {
		return ReloadCheckResult{
			ShouldReload: false,
		}
	}

	reloaderEnabled, _ := strconv.ParseBool(reloaderEnabledValue)
	typedAutoAnnotationEnabled, _ := strconv.ParseBool(typedAutoAnnotationEnabledValue)
	if reloaderEnabled || typedAutoAnnotationEnabled || reloaderEnabledValue == "" && typedAutoAnnotationEnabledValue == "" && options.AutoReloadAll {
		return ReloadCheckResult{
			ShouldReload: true,
			AutoReload:   true,
		}
	}

	values := strings.Split(annotationValue, ",")
	for _, value := range values {
		value = strings.TrimSpace(value)
		re := regexp.MustCompile("^" + value + "$")
		if re.Match([]byte(config.ResourceName)) {
			return ReloadCheckResult{
				ShouldReload: true,
				AutoReload:   false,
			}
		}
	}

	if searchAnnotationValue == "true" {
		matchAnnotationValue := config.ResourceAnnotations[options.SearchMatchAnnotation]
		if matchAnnotationValue == "true" {
			return ReloadCheckResult{
				ShouldReload: true,
				AutoReload:   true,
			}
		}
	}

	return ReloadCheckResult{
		ShouldReload: false,
	}
}

func checkIfResourceIsExcluded(resourceName, excludedResources string) bool {
	if excludedResources == "" {
		return false
	}

	excludedResourcesList := strings.Split(excludedResources, ",")
	for _, excludedResource := range excludedResourcesList {
		if strings.TrimSpace(excludedResource) == resourceName {
			return true
		}
	}

	return false
}

func init() {
	GetCommandLineOptions()
}

func GetCommandLineOptions() *ReloaderOptions {
	if CommandLineOptions == nil {
		CommandLineOptions = &ReloaderOptions{}
	}

	CommandLineOptions.AutoReloadAll = options.AutoReloadAll
	CommandLineOptions.ConfigmapUpdateOnChangeAnnotation = options.ConfigmapUpdateOnChangeAnnotation
	CommandLineOptions.SecretUpdateOnChangeAnnotation = options.SecretUpdateOnChangeAnnotation
	CommandLineOptions.ReloaderAutoAnnotation = options.ReloaderAutoAnnotation
	CommandLineOptions.IgnoreResourceAnnotation = options.IgnoreResourceAnnotation
	CommandLineOptions.ConfigmapReloaderAutoAnnotation = options.ConfigmapReloaderAutoAnnotation
	CommandLineOptions.SecretReloaderAutoAnnotation = options.SecretReloaderAutoAnnotation
	CommandLineOptions.ConfigmapExcludeReloaderAnnotation = options.ConfigmapExcludeReloaderAnnotation
	CommandLineOptions.SecretExcludeReloaderAnnotation = options.SecretExcludeReloaderAnnotation
	CommandLineOptions.AutoSearchAnnotation = options.AutoSearchAnnotation
	CommandLineOptions.SearchMatchAnnotation = options.SearchMatchAnnotation
	CommandLineOptions.RolloutStrategyAnnotation = options.RolloutStrategyAnnotation
	CommandLineOptions.PauseDeploymentAnnotation = options.PauseDeploymentAnnotation
	CommandLineOptions.PauseDeploymentTimeAnnotation = options.PauseDeploymentTimeAnnotation
	CommandLineOptions.LogFormat = options.LogFormat
	CommandLineOptions.LogLevel = options.LogLevel
	CommandLineOptions.ReloadStrategy = options.ReloadStrategy
	CommandLineOptions.SyncAfterRestart = options.SyncAfterRestart
	CommandLineOptions.EnableHA = options.EnableHA
	CommandLineOptions.WebhookUrl = options.WebhookUrl
	CommandLineOptions.ResourcesToIgnore = options.ResourcesToIgnore
	CommandLineOptions.WorkloadTypesToIgnore = options.WorkloadTypesToIgnore
	CommandLineOptions.NamespaceSelectors = options.NamespaceSelectors
	CommandLineOptions.ResourceSelectors = options.ResourceSelectors
	CommandLineOptions.NamespacesToIgnore = options.NamespacesToIgnore
	CommandLineOptions.IsArgoRollouts = parseBool(options.IsArgoRollouts)
	CommandLineOptions.ReloadOnCreate = parseBool(options.ReloadOnCreate)
	CommandLineOptions.ReloadOnDelete = parseBool(options.ReloadOnDelete)
	CommandLineOptions.EnablePProf = options.EnablePProf
	CommandLineOptions.PProfAddr = options.PProfAddr

	return CommandLineOptions
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
