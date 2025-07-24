package common

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/options"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Map map[string]string

type ReloadCheckResult struct {
	ShouldReload bool
	AutoReload   bool
}

func PublishMetaInfoConfigmap(clientset kubernetes.Interface) {
	namespace := os.Getenv("RELOADER_NAMESPACE")
	if namespace == "" {
		logrus.Warn("RELOADER_NAMESPACE is not set, skipping meta info configmap creation")
		return
	}

	metaInfo := &MetaInfo{
		BuildInfo:       *NewBuildInfo(),
		ReloaderOptions: *options.GetCommandLineOptions(),
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

func ShouldReload(config util.Config, resourceType string, annotations Map, podAnnotations Map, options *options.ReloaderOptions) ReloadCheckResult {

	if resourceType == "Rollout" && !options.IsArgoRollouts {
		return ReloadCheckResult{
			ShouldReload: false,
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
