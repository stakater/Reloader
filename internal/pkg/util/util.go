package util

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/internal/pkg/options"
	v1 "k8s.io/api/core/v1"
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

type ReloadCheckResult struct {
	ShouldReload bool
	AutoReload   bool
}

func ShouldReload(config Config, annotations Map, podAnnotations Map) ReloadCheckResult {

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
