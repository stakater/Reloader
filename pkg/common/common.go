package common

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
)

type Map map[string]string

type ReloadCheckResult struct {
	ShouldReload bool
	AutoReload   bool
	Errors       []error
}

// ReloaderOptions contains all configurable options for the Reloader controller.
// These options control how Reloader behaves when watching for changes in ConfigMaps and Secrets.
type ReloaderOptions struct {
	EnableRestartWindows bool `json:"enableRestartWindows"`
	// AutoReloadAll enables automatic reloading of all resources when their corresponding ConfigMaps/Secrets are updated
	AutoReloadAll bool `json:"autoReloadAll"`
	// ConfigmapUpdateOnChangeAnnotation is the annotation key used to detect changes in ConfigMaps specified by name
	ConfigmapUpdateOnChangeAnnotation string `json:"configmapUpdateOnChangeAnnotation"`
	// SecretUpdateOnChangeAnnotation is the annotation key used to detect changes in Secrets specified by name
	SecretUpdateOnChangeAnnotation string `json:"secretUpdateOnChangeAnnotation"`
	// SecretProviderClassUpdateOnChangeAnnotation is the annotation key used to detect changes in SecretProviderClasses specified by name
	SecretProviderClassUpdateOnChangeAnnotation string `json:"secretProviderClassUpdateOnChangeAnnotation"`
	// ReloaderAutoAnnotation is the annotation key used to detect changes in any referenced ConfigMaps or Secrets
	ReloaderAutoAnnotation string `json:"reloaderAutoAnnotation"`
	// IgnoreResourceAnnotation is the annotation key used to ignore resources from being watched
	IgnoreResourceAnnotation string `json:"ignoreResourceAnnotation"`
	// ConfigmapReloaderAutoAnnotation is the annotation key used to detect changes in ConfigMaps only
	ConfigmapReloaderAutoAnnotation string `json:"configmapReloaderAutoAnnotation"`
	// SecretReloaderAutoAnnotation is the annotation key used to detect changes in Secrets only
	SecretReloaderAutoAnnotation string `json:"secretReloaderAutoAnnotation"`
	// SecretProviderClassReloaderAutoAnnotation is the annotation key used to detect changes in SecretProviderClasses only
	SecretProviderClassReloaderAutoAnnotation string `json:"secretProviderClassReloaderAutoAnnotation"`
	// ConfigmapExcludeReloaderAnnotation is the annotation key containing comma-separated list of ConfigMaps to exclude from watching
	ConfigmapExcludeReloaderAnnotation string `json:"configmapExcludeReloaderAnnotation"`
	// SecretExcludeReloaderAnnotation is the annotation key containing comma-separated list of Secrets to exclude from watching
	SecretExcludeReloaderAnnotation string `json:"secretExcludeReloaderAnnotation"`
	// SecretProviderClassExcludeReloaderAnnotation is the annotation key containing comma-separated list of SecretProviderClasses to exclude from watching
	SecretProviderClassExcludeReloaderAnnotation string `json:"secretProviderClassExcludeReloaderAnnotation"`
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
	// LeaderElectionLeaseDuration is the duration non-leader candidates wait before force acquiring leadership, formatted as a Go duration string
	LeaderElectionLeaseDuration string `json:"leaderElectionLeaseDuration"`
	// LeaderElectionRenewDeadline is the duration the acting leader retries refreshing leadership before giving up, formatted as a Go duration string
	LeaderElectionRenewDeadline string `json:"leaderElectionRenewDeadline"`
	// LeaderElectionRetryPeriod is the duration clients wait between attempting acquisition and renewal of leadership, formatted as a Go duration string
	LeaderElectionRetryPeriod string `json:"leaderElectionRetryPeriod"`
	// EnableCSIIntegration indicates whether CSI integration is enabled to watch SecretProviderClassPodStatus
	EnableCSIIntegration bool `json:"enableCSIIntegration"`
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

// CommandLineOptions is retained for compatibility with external users of this
// package. Reloader refreshes it once after Cobra parses the process flags.
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
func ShouldReload(config Config, resourceType string, annotations Map, podAnnotations Map, reloaderOpts *ReloaderOptions) ReloadCheckResult {

	// Check if this workload type should be ignored.
	// Use reloaderOpts.WorkloadTypesToIgnore directly instead of re-reading the
	// global via util.GetIgnoredWorkloadTypesList(), so that invalid entries simply
	// skip the ignore check (allowing reload) rather than silently blocking it.
	if len(reloaderOpts.WorkloadTypesToIgnore) > 0 {
		validIgnored := util.List{}
		valid := true
		for _, v := range reloaderOpts.WorkloadTypesToIgnore {
			if v != "jobs" && v != "cronjobs" {
				logrus.Errorf("Failed to parse ignored workload types: 'ignored-workload-types' accepts 'jobs', 'cronjobs', or both, not '%s'", v)
				valid = false
				break
			}
			validIgnored = append(validIgnored, v)
		}
		if valid {
			// Map Kubernetes resource types to CLI-friendly names for comparison
			var resourceToCheck string
			switch resourceType {
			case "Job":
				resourceToCheck = "jobs"
			case "CronJob":
				resourceToCheck = "cronjobs"
			default:
				resourceToCheck = resourceType
			}
			if validIgnored.Contains(resourceToCheck) {
				return ReloadCheckResult{ShouldReload: false}
			}
		}
	}

	ignoreResourceAnnotatonValue := config.ResourceAnnotations[reloaderOpts.IgnoreResourceAnnotation]
	if ignoreResourceAnnotatonValue == "true" {
		return ReloadCheckResult{
			ShouldReload: false,
		}
	}

	annotationValue, found := annotations[config.Annotation]
	searchAnnotationValue, foundSearchAnn := annotations[reloaderOpts.AutoSearchAnnotation]
	reloaderEnabledValue, foundAuto := annotations[reloaderOpts.ReloaderAutoAnnotation]
	typedAutoAnnotationEnabledValue, foundTypedAuto := annotations[config.TypedAutoAnnotation]
	excludeConfigmapAnnotationValue, foundExcludeConfigmap := annotations[reloaderOpts.ConfigmapExcludeReloaderAnnotation]
	excludeSecretAnnotationValue, foundExcludeSecret := annotations[reloaderOpts.SecretExcludeReloaderAnnotation]
	excludeSecretProviderClassProviderAnnotationValue, foundExcludeSecretProviderClass := annotations[reloaderOpts.SecretProviderClassExcludeReloaderAnnotation]

	if !found && !foundAuto && !foundTypedAuto && !foundSearchAnn {
		annotations = podAnnotations
		annotationValue = annotations[config.Annotation]
		searchAnnotationValue = annotations[reloaderOpts.AutoSearchAnnotation]
		reloaderEnabledValue = annotations[reloaderOpts.ReloaderAutoAnnotation]
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

	case constants.SecretProviderClassEnvVarPostfix:
		if foundExcludeSecretProviderClass {
			isResourceExcluded = checkIfResourceIsExcluded(config.ResourceName, excludeSecretProviderClassProviderAnnotationValue)
		}
	}

	if isResourceExcluded {
		return ReloadCheckResult{
			ShouldReload: false,
		}
	}

	var regexErrors []error
	values := strings.Split(annotationValue, ",")
	for _, value := range values {
		value = strings.TrimSpace(value)
		re, err := regexp.Compile("^" + value + "$")
		if err != nil {
			regexErrors = append(regexErrors, fmt.Errorf("invalid regex %q in reload annotation %q: %w", value, config.Annotation, err))
			continue
		}
		if re.Match([]byte(config.ResourceName)) {
			return ReloadCheckResult{
				ShouldReload: true,
				AutoReload:   false,
				Errors:       regexErrors,
			}
		}
	}

	if searchAnnotationValue == "true" {
		matchAnnotationValue := config.ResourceAnnotations[reloaderOpts.SearchMatchAnnotation]
		if matchAnnotationValue == "true" {
			return ReloadCheckResult{
				ShouldReload: true,
				AutoReload:   true,
				Errors:       regexErrors,
			}
		}
	}

	reloaderEnabled, _ := strconv.ParseBool(reloaderEnabledValue)
	typedAutoAnnotationEnabled, _ := strconv.ParseBool(typedAutoAnnotationEnabledValue)
	if reloaderEnabled || typedAutoAnnotationEnabled || reloaderEnabledValue == "" && typedAutoAnnotationEnabledValue == "" && reloaderOpts.AutoReloadAll {
		return ReloadCheckResult{
			ShouldReload: true,
			AutoReload:   true,
			Errors:       regexErrors,
		}
	}

	return ReloadCheckResult{
		ShouldReload: false,
		Errors:       regexErrors,
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

func GetCommandLineOptions() *ReloaderOptions {
	// Return an independent snapshot: event workers and the window scheduler run concurrently.
	commandLineOptions := &ReloaderOptions{}
	commandLineOptions.EnableRestartWindows = options.EnableRestartWindows

	commandLineOptions.AutoReloadAll = options.AutoReloadAll
	commandLineOptions.ConfigmapUpdateOnChangeAnnotation = options.ConfigmapUpdateOnChangeAnnotation
	commandLineOptions.SecretUpdateOnChangeAnnotation = options.SecretUpdateOnChangeAnnotation
	commandLineOptions.SecretProviderClassUpdateOnChangeAnnotation = options.SecretProviderClassUpdateOnChangeAnnotation
	commandLineOptions.ReloaderAutoAnnotation = options.ReloaderAutoAnnotation
	commandLineOptions.IgnoreResourceAnnotation = options.IgnoreResourceAnnotation
	commandLineOptions.ConfigmapReloaderAutoAnnotation = options.ConfigmapReloaderAutoAnnotation
	commandLineOptions.SecretReloaderAutoAnnotation = options.SecretReloaderAutoAnnotation
	commandLineOptions.SecretProviderClassReloaderAutoAnnotation = options.SecretProviderClassReloaderAutoAnnotation
	commandLineOptions.ConfigmapExcludeReloaderAnnotation = options.ConfigmapExcludeReloaderAnnotation
	commandLineOptions.SecretExcludeReloaderAnnotation = options.SecretExcludeReloaderAnnotation
	commandLineOptions.SecretProviderClassExcludeReloaderAnnotation = options.SecretProviderClassExcludeReloaderAnnotation
	commandLineOptions.AutoSearchAnnotation = options.AutoSearchAnnotation
	commandLineOptions.SearchMatchAnnotation = options.SearchMatchAnnotation
	commandLineOptions.RolloutStrategyAnnotation = options.RolloutStrategyAnnotation
	commandLineOptions.PauseDeploymentAnnotation = options.PauseDeploymentAnnotation
	commandLineOptions.PauseDeploymentTimeAnnotation = options.PauseDeploymentTimeAnnotation
	commandLineOptions.LogFormat = options.LogFormat
	commandLineOptions.LogLevel = options.LogLevel
	commandLineOptions.ReloadStrategy = options.ReloadStrategy
	commandLineOptions.SyncAfterRestart = options.SyncAfterRestart
	commandLineOptions.EnableHA = options.EnableHA
	commandLineOptions.LeaderElectionLeaseDuration = options.LeaderElectionLeaseDuration.String()
	commandLineOptions.LeaderElectionRenewDeadline = options.LeaderElectionRenewDeadline.String()
	commandLineOptions.LeaderElectionRetryPeriod = options.LeaderElectionRetryPeriod.String()
	commandLineOptions.EnableCSIIntegration = options.EnableCSIIntegration
	commandLineOptions.WebhookUrl = options.WebhookUrl
	commandLineOptions.ResourcesToIgnore = options.ResourcesToIgnore
	commandLineOptions.WorkloadTypesToIgnore = options.WorkloadTypesToIgnore
	commandLineOptions.NamespaceSelectors = options.NamespaceSelectors
	commandLineOptions.ResourceSelectors = options.ResourceSelectors
	commandLineOptions.NamespacesToIgnore = options.NamespacesToIgnore
	commandLineOptions.IsArgoRollouts = parseBool(options.IsArgoRollouts)
	commandLineOptions.ReloadOnCreate = parseBool(options.ReloadOnCreate)
	commandLineOptions.ReloadOnDelete = parseBool(options.ReloadOnDelete)
	commandLineOptions.EnablePProf = options.EnablePProf
	commandLineOptions.PProfAddr = options.PProfAddr

	return commandLineOptions
}

// RefreshCommandLineOptions publishes a current immutable options snapshot.
// Call it during startup before worker goroutines begin reading the value.
func RefreshCommandLineOptions() *ReloaderOptions {
	CommandLineOptions = GetCommandLineOptions()
	return CommandLineOptions
}

func init() {
	RefreshCommandLineOptions()
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
