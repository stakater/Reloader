package util

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/internal/pkg/options"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ConvertToEnvVarName converts the given text into a usable env var
// removing any special chars with '_' and transforming text to upper case
func ConvertToEnvVarName(text string) string {
	var buffer bytes.Buffer
	upper := strings.ToUpper(text)
	lastCharValid := false
	for i := 0; i < len(upper); i++ {
		ch := upper[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			buffer.WriteString(string(ch))
			lastCharValid = true
		} else {
			if lastCharValid {
				buffer.WriteString("_")
			}
			lastCharValid = false
		}
	}
	return buffer.String()
}

func GetSHAfromConfigmap(configmap *v1.ConfigMap) string {
	values := []string{}
	for k, v := range configmap.Data {
		values = append(values, k+"="+v)
	}
	for k, v := range configmap.BinaryData {
		values = append(values, k+"="+base64.StdEncoding.EncodeToString(v))
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

func GetSHAfromSecret(data map[string][]byte) string {
	values := []string{}
	for k, v := range data {
		values = append(values, k+"="+string(v[:]))
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

type List []string

type Map map[string]string

func (l *List) Contains(s string) bool {
	for _, v := range *l {
		if v == s {
			return true
		}
	}
	return false
}

func ConfigureReloaderFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(&options.AutoReloadAll, "auto-reload-all", false, "Auto reload all resources")
	cmd.PersistentFlags().StringVar(&options.ConfigmapUpdateOnChangeAnnotation, "configmap-annotation", "configmap.reloader.stakater.com/reload", "annotation to detect changes in configmaps, specified by name")
	cmd.PersistentFlags().StringVar(&options.SecretUpdateOnChangeAnnotation, "secret-annotation", "secret.reloader.stakater.com/reload", "annotation to detect changes in secrets, specified by name")
	cmd.PersistentFlags().StringVar(&options.ReloaderAutoAnnotation, "auto-annotation", "reloader.stakater.com/auto", "annotation to detect changes in secrets/configmaps")
	cmd.PersistentFlags().StringVar(&options.ConfigmapReloaderAutoAnnotation, "configmap-auto-annotation", "configmap.reloader.stakater.com/auto", "annotation to detect changes in configmaps")
	cmd.PersistentFlags().StringVar(&options.SecretReloaderAutoAnnotation, "secret-auto-annotation", "secret.reloader.stakater.com/auto", "annotation to detect changes in secrets")
	cmd.PersistentFlags().StringVar(&options.AutoSearchAnnotation, "auto-search-annotation", "reloader.stakater.com/search", "annotation to detect changes in configmaps or secrets tagged with special match annotation")
	cmd.PersistentFlags().StringVar(&options.SearchMatchAnnotation, "search-match-annotation", "reloader.stakater.com/match", "annotation to mark secrets or configmaps to match the search")
	cmd.PersistentFlags().StringVar(&options.LogFormat, "log-format", "", "Log format to use (empty string for text, or JSON)")
	cmd.PersistentFlags().StringVar(&options.LogLevel, "log-level", "info", "Log level to use (trace, debug, info, warning, error, fatal and panic)")
	cmd.PersistentFlags().StringVar(&options.WebhookUrl, "webhook-url", "", "webhook to trigger instead of performing a reload")
	cmd.PersistentFlags().StringSliceVar(&options.ResourcesToIgnore, "resources-to-ignore", options.ResourcesToIgnore, "list of resources to ignore (valid options 'configMaps' or 'secrets')")
	cmd.PersistentFlags().StringSliceVar(&options.NamespacesToIgnore, "namespaces-to-ignore", options.NamespacesToIgnore, "list of namespaces to ignore")
	cmd.PersistentFlags().StringSliceVar(&options.NamespaceSelectors, "namespace-selector", options.NamespaceSelectors, "list of key:value labels to filter on for namespaces")
	cmd.PersistentFlags().StringSliceVar(&options.ResourceSelectors, "resource-label-selector", options.ResourceSelectors, "list of key:value labels to filter on for configmaps and secrets")
	cmd.PersistentFlags().StringVar(&options.IsArgoRollouts, "is-Argo-Rollouts", "false", "Add support for argo rollouts")
	cmd.PersistentFlags().StringVar(&options.ReloadStrategy, constants.ReloadStrategyFlag, constants.EnvVarsReloadStrategy, "Specifies the desired reload strategy")
	cmd.PersistentFlags().StringVar(&options.ReloadOnCreate, "reload-on-create", "false", "Add support to watch create events")
	cmd.PersistentFlags().StringVar(&options.ReloadOnDelete, "reload-on-delete", "false", "Add support to watch delete events")
	cmd.PersistentFlags().BoolVar(&options.EnableHA, "enable-ha", false, "Adds support for running multiple replicas via leadership election")
	cmd.PersistentFlags().BoolVar(&options.SyncAfterRestart, "sync-after-restart", false, "Sync add events after reloader restarts")
}

type ReloadCheckResult struct {
	ShouldReload bool
	AutoReload   bool
}

func ShouldReload(config Config, resourceType string, annotations Map, podAnnotations Map) ReloadCheckResult {

	if resourceType == "Rollout" && options.IsArgoRollouts == "false" {
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

func GetNamespaceLabelSelector() (string, error) {
	slice := options.NamespaceSelectors

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

func GetResourceLabelSelector() (string, error) {
	slice := options.ResourceSelectors

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

func GetIgnoredResourcesList() (List, error) {

	ignoredResourcesList := options.ResourcesToIgnore // getStringSliceFromFlags(cmd, "resources-to-ignore")

	for _, v := range ignoredResourcesList {
		if v != "configMaps" && v != "secrets" {
			return nil, fmt.Errorf("'resources-to-ignore' only accepts 'configMaps' or 'secrets', not '%s'", v)
		}
	}

	if len(ignoredResourcesList) > 1 {
		return nil, errors.New("'resources-to-ignore' only accepts 'configMaps' or 'secrets', not both")
	}

	return ignoredResourcesList, nil
}
