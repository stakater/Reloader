package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/parnurzeal/gorequest"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	alert "github.com/stakater/Reloader/internal/pkg/alerts"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// GetDeploymentRollingUpgradeFuncs returns all callback funcs for a deployment
func GetDeploymentRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetDeploymentItems,
		AnnotationsFunc:    callbacks.GetDeploymentAnnotations,
		PodAnnotationsFunc: callbacks.GetDeploymentPodAnnotations,
		ContainersFunc:     callbacks.GetDeploymentContainers,
		InitContainersFunc: callbacks.GetDeploymentInitContainers,
		UpdateFunc:         callbacks.UpdateDeployment,
		VolumesFunc:        callbacks.GetDeploymentVolumes,
		ResourceType:       "Deployment",
	}
}

// GetDeploymentRollingUpgradeFuncs returns all callback funcs for a cronjob
func GetCronJobCreateJobFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetCronJobItems,
		AnnotationsFunc:    callbacks.GetCronJobAnnotations,
		PodAnnotationsFunc: callbacks.GetCronJobPodAnnotations,
		ContainersFunc:     callbacks.GetCronJobContainers,
		InitContainersFunc: callbacks.GetCronJobInitContainers,
		UpdateFunc:         callbacks.CreateJobFromCronjob,
		VolumesFunc:        callbacks.GetCronJobVolumes,
		ResourceType:       "CronJob",
	}
}

// GetDeploymentRollingUpgradeFuncs returns all callback funcs for a cronjob
func GetJobCreateJobFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetJobItems,
		AnnotationsFunc:    callbacks.GetJobAnnotations,
		PodAnnotationsFunc: callbacks.GetJobPodAnnotations,
		ContainersFunc:     callbacks.GetJobContainers,
		InitContainersFunc: callbacks.GetJobInitContainers,
		UpdateFunc:         callbacks.ReCreateJobFromjob,
		VolumesFunc:        callbacks.GetJobVolumes,
		ResourceType:       "Job",
	}
}

// GetDaemonSetRollingUpgradeFuncs returns all callback funcs for a daemonset
func GetDaemonSetRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetDaemonSetItems,
		AnnotationsFunc:    callbacks.GetDaemonSetAnnotations,
		PodAnnotationsFunc: callbacks.GetDaemonSetPodAnnotations,
		ContainersFunc:     callbacks.GetDaemonSetContainers,
		InitContainersFunc: callbacks.GetDaemonSetInitContainers,
		UpdateFunc:         callbacks.UpdateDaemonSet,
		VolumesFunc:        callbacks.GetDaemonSetVolumes,
		ResourceType:       "DaemonSet",
	}
}

// GetStatefulSetRollingUpgradeFuncs returns all callback funcs for a statefulSet
func GetStatefulSetRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetStatefulSetItems,
		AnnotationsFunc:    callbacks.GetStatefulSetAnnotations,
		PodAnnotationsFunc: callbacks.GetStatefulSetPodAnnotations,
		ContainersFunc:     callbacks.GetStatefulSetContainers,
		InitContainersFunc: callbacks.GetStatefulSetInitContainers,
		UpdateFunc:         callbacks.UpdateStatefulSet,
		VolumesFunc:        callbacks.GetStatefulSetVolumes,
		ResourceType:       "StatefulSet",
	}
}

// GetDeploymentConfigRollingUpgradeFuncs returns all callback funcs for a deploymentConfig
func GetDeploymentConfigRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetDeploymentConfigItems,
		AnnotationsFunc:    callbacks.GetDeploymentConfigAnnotations,
		PodAnnotationsFunc: callbacks.GetDeploymentConfigPodAnnotations,
		ContainersFunc:     callbacks.GetDeploymentConfigContainers,
		InitContainersFunc: callbacks.GetDeploymentConfigInitContainers,
		UpdateFunc:         callbacks.UpdateDeploymentConfig,
		VolumesFunc:        callbacks.GetDeploymentConfigVolumes,
		ResourceType:       "DeploymentConfig",
	}
}

// GetArgoRolloutRollingUpgradeFuncs returns all callback funcs for a rollout
func GetArgoRolloutRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetRolloutItems,
		AnnotationsFunc:    callbacks.GetRolloutAnnotations,
		PodAnnotationsFunc: callbacks.GetRolloutPodAnnotations,
		ContainersFunc:     callbacks.GetRolloutContainers,
		InitContainersFunc: callbacks.GetRolloutInitContainers,
		UpdateFunc:         callbacks.UpdateRollout,
		VolumesFunc:        callbacks.GetRolloutVolumes,
		ResourceType:       "Rollout",
	}
}

func sendUpgradeWebhook(config util.Config, webhookUrl string) error {
	logrus.Infof("Changes detected in '%s' of type '%s' in namespace '%s', Sending webhook to '%s'",
		config.ResourceName, config.Type, config.Namespace, webhookUrl)

	body, errs := sendWebhook(webhookUrl)
	if errs != nil {
		// return the first error
		return errs[0]
	} else {
		logrus.Info(body)
	}

	return nil
}

func sendWebhook(url string) (string, []error) {
	request := gorequest.New()
	resp, _, err := request.Post(url).Send(`{"webhook":"update successful"}`).End()
	if err != nil {
		// the reloader seems to retry automatically so no retry logic added
		return "", err
	}
	defer resp.Body.Close()
	var buffer bytes.Buffer
	_, bufferErr := io.Copy(&buffer, resp.Body)
	if bufferErr != nil {
		logrus.Error(bufferErr)
	}
	return buffer.String(), nil
}

func doRollingUpgrade(config util.Config, collectors metrics.Collectors, recorder record.EventRecorder, invoke invokeStrategy) error {
	clients := kube.GetClients()

	err := rollingUpgrade(clients, config, GetDeploymentRollingUpgradeFuncs(), collectors, recorder, invoke)
	if err != nil {
		return err
	}
	err = rollingUpgrade(clients, config, GetCronJobCreateJobFuncs(), collectors, recorder, invoke)
	if err != nil {
		return err
	}
	err = rollingUpgrade(clients, config, GetJobCreateJobFuncs(), collectors, recorder, invoke)
	if err != nil {
		return err
	}
	err = rollingUpgrade(clients, config, GetDaemonSetRollingUpgradeFuncs(), collectors, recorder, invoke)
	if err != nil {
		return err
	}
	err = rollingUpgrade(clients, config, GetStatefulSetRollingUpgradeFuncs(), collectors, recorder, invoke)
	if err != nil {
		return err
	}

	if kube.IsOpenshift {
		err = rollingUpgrade(clients, config, GetDeploymentConfigRollingUpgradeFuncs(), collectors, recorder, invoke)
		if err != nil {
			return err
		}
	}

	if options.IsArgoRollouts == "true" {
		err = rollingUpgrade(clients, config, GetArgoRolloutRollingUpgradeFuncs(), collectors, recorder, invoke)
		if err != nil {
			return err
		}
	}

	return nil
}

func rollingUpgrade(clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, recorder record.EventRecorder, strategy invokeStrategy) error {

	err := PerformAction(clients, config, upgradeFuncs, collectors, recorder, strategy)
	if err != nil {
		logrus.Errorf("Rolling upgrade for '%s' failed with error = %v", config.ResourceName, err)
	}
	return err
}

// PerformAction invokes the deployment if there is any change in configmap or secret data
func PerformAction(clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, recorder record.EventRecorder, strategy invokeStrategy) error {
	items := upgradeFuncs.ItemsFunc(clients, config.Namespace)

	for _, i := range items {
		// find correct annotation and update the resource
		annotations := upgradeFuncs.AnnotationsFunc(i)
		annotationValue, found := annotations[config.Annotation]
		searchAnnotationValue, foundSearchAnn := annotations[options.AutoSearchAnnotation]
		reloaderEnabledValue, foundAuto := annotations[options.ReloaderAutoAnnotation]
		typedAutoAnnotationEnabledValue, foundTypedAuto := annotations[config.TypedAutoAnnotation]
		excludeConfigmapAnnotationValue, foundExcludeConfigmap := annotations[options.ConfigmapExcludeReloaderAnnotation]
		excludeSecretAnnotationValue, foundExcludeSecret := annotations[options.SecretExcludeReloaderAnnotation]

		if !found && !foundAuto && !foundTypedAuto && !foundSearchAnn {
			annotations = upgradeFuncs.PodAnnotationsFunc(i)
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
			continue
		}

		result := constants.NotUpdated
		reloaderEnabled, _ := strconv.ParseBool(reloaderEnabledValue)
		typedAutoAnnotationEnabled, _ := strconv.ParseBool(typedAutoAnnotationEnabledValue)
		if reloaderEnabled || typedAutoAnnotationEnabled || reloaderEnabledValue == "" && typedAutoAnnotationEnabledValue == "" && options.AutoReloadAll {
			result = strategy(upgradeFuncs, i, config, true)
		}

		if result != constants.Updated && annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				value = strings.TrimSpace(value)
				re := regexp.MustCompile("^" + value + "$")
				if re.Match([]byte(config.ResourceName)) {
					result = strategy(upgradeFuncs, i, config, false)
					if result == constants.Updated {
						break
					}
				}
			}
		}

		if result != constants.Updated && searchAnnotationValue == "true" {
			matchAnnotationValue := config.ResourceAnnotations[options.SearchMatchAnnotation]
			if matchAnnotationValue == "true" {
				result = strategy(upgradeFuncs, i, config, true)
			}
		}

		if result == constants.Updated {
			accessor, err := meta.Accessor(i)
			if err != nil {
				return err
			}
			resourceName := accessor.GetName()
			err = upgradeFuncs.UpdateFunc(clients, config.Namespace, i)
			if err != nil {
				message := fmt.Sprintf("Update for '%s' of type '%s' in namespace '%s' failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)
				logrus.Errorf("Update for '%s' of type '%s' in namespace '%s' failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)

				collectors.Reloaded.With(prometheus.Labels{"success": "false"}).Inc()
				collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "false", "namespace": config.Namespace}).Inc()
				if recorder != nil {
					recorder.Event(i, v1.EventTypeWarning, "ReloadFail", message)
				}
				return err
			} else {
				message := fmt.Sprintf("Changes detected in '%s' of type '%s' in namespace '%s'", config.ResourceName, config.Type, config.Namespace)
				message += fmt.Sprintf(", Updated '%s' of type '%s' in namespace '%s'", resourceName, upgradeFuncs.ResourceType, config.Namespace)

				logrus.Infof("Changes detected in '%s' of type '%s' in namespace '%s'; updated '%s' of type '%s' in namespace '%s'", config.ResourceName, config.Type, config.Namespace, resourceName, upgradeFuncs.ResourceType, config.Namespace)

				collectors.Reloaded.With(prometheus.Labels{"success": "true"}).Inc()
				collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": config.Namespace}).Inc()
				alert_on_reload, ok := os.LookupEnv("ALERT_ON_RELOAD")
				if recorder != nil {
					recorder.Event(i, v1.EventTypeNormal, "Reloaded", message)
				}
				if ok && alert_on_reload == "true" {
					msg := fmt.Sprintf(
						"Reloader detected changes in *%s* of type *%s* in namespace *%s*. Hence reloaded *%s* of type *%s* in namespace *%s*",
						config.ResourceName, config.Type, config.Namespace, resourceName, upgradeFuncs.ResourceType, config.Namespace)
					alert.SendWebhookAlert(msg)
				}
			}
		}
	}
	return nil
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

func getVolumeMountName(volumes []v1.Volume, mountType string, volumeName string) string {
	for i := range volumes {
		if mountType == constants.ConfigmapEnvVarPostfix {
			if volumes[i].ConfigMap != nil && volumes[i].ConfigMap.Name == volumeName {
				return volumes[i].Name
			}

			if volumes[i].Projected != nil {
				for j := range volumes[i].Projected.Sources {
					if volumes[i].Projected.Sources[j].ConfigMap != nil && volumes[i].Projected.Sources[j].ConfigMap.Name == volumeName {
						return volumes[i].Name
					}
				}
			}
		} else if mountType == constants.SecretEnvVarPostfix {
			if volumes[i].Secret != nil && volumes[i].Secret.SecretName == volumeName {
				return volumes[i].Name
			}

			if volumes[i].Projected != nil {
				for j := range volumes[i].Projected.Sources {
					if volumes[i].Projected.Sources[j].Secret != nil && volumes[i].Projected.Sources[j].Secret.Name == volumeName {
						return volumes[i].Name
					}
				}
			}
		}
	}

	return ""
}

func getContainerWithVolumeMount(containers []v1.Container, volumeMountName string) *v1.Container {
	for i := range containers {
		volumeMounts := containers[i].VolumeMounts
		for j := range volumeMounts {
			if volumeMounts[j].Name == volumeMountName {
				return &containers[i]
			}
		}
	}

	return nil
}

func getContainerWithEnvReference(containers []v1.Container, resourceName string, resourceType string) *v1.Container {
	for i := range containers {
		envs := containers[i].Env
		for j := range envs {
			envVarSource := envs[j].ValueFrom
			if envVarSource != nil {
				if resourceType == constants.SecretEnvVarPostfix && envVarSource.SecretKeyRef != nil && envVarSource.SecretKeyRef.LocalObjectReference.Name == resourceName {
					return &containers[i]
				} else if resourceType == constants.ConfigmapEnvVarPostfix && envVarSource.ConfigMapKeyRef != nil && envVarSource.ConfigMapKeyRef.LocalObjectReference.Name == resourceName {
					return &containers[i]
				}
			}
		}

		envsFrom := containers[i].EnvFrom
		for j := range envsFrom {
			if resourceType == constants.SecretEnvVarPostfix && envsFrom[j].SecretRef != nil && envsFrom[j].SecretRef.LocalObjectReference.Name == resourceName {
				return &containers[i]
			} else if resourceType == constants.ConfigmapEnvVarPostfix && envsFrom[j].ConfigMapRef != nil && envsFrom[j].ConfigMapRef.LocalObjectReference.Name == resourceName {
				return &containers[i]
			}
		}
	}
	return nil
}

func getContainerUsingResource(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) *v1.Container {
	volumes := upgradeFuncs.VolumesFunc(item)
	containers := upgradeFuncs.ContainersFunc(item)
	initContainers := upgradeFuncs.InitContainersFunc(item)
	var container *v1.Container
	// Get the volumeMountName to find volumeMount in container
	volumeMountName := getVolumeMountName(volumes, config.Type, config.ResourceName)
	// Get the container with mounted configmap/secret
	if volumeMountName != "" {
		container = getContainerWithVolumeMount(containers, volumeMountName)
		if container == nil && len(initContainers) > 0 {
			container = getContainerWithVolumeMount(initContainers, volumeMountName)
			if container != nil {
				// if configmap/secret is being used in init container then return the first Pod container to save reloader env
				return &containers[0]
			}
		} else if container != nil {
			return container
		}
	}

	// Get the container with referenced secret or configmap as env var
	container = getContainerWithEnvReference(containers, config.ResourceName, config.Type)
	if container == nil && len(initContainers) > 0 {
		container = getContainerWithEnvReference(initContainers, config.ResourceName, config.Type)
		if container != nil {
			// if configmap/secret is being used in init container then return the first Pod container to save reloader env
			return &containers[0]
		}
	}

	// Get the first container if the annotation is related to specified configmap or secret i.e. configmap.reloader.stakater.com/reload
	if container == nil && !autoReload {
		return &containers[0]
	}

	return container
}

type invokeStrategy func(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) constants.Result

func invokeReloadStrategy(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) constants.Result {
	if options.ReloadStrategy == constants.AnnotationsReloadStrategy {
		return updatePodAnnotations(upgradeFuncs, item, config, autoReload)
	}

	return updateContainerEnvVars(upgradeFuncs, item, config, autoReload)
}

func updatePodAnnotations(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) constants.Result {
	container := getContainerUsingResource(upgradeFuncs, item, config, autoReload)
	if container == nil {
		return constants.NoContainerFound
	}

	// Generate reloaded annotations. Attaching this to the item's annotation will trigger a rollout
	// Note: the data on this struct is purely informational and is not used for future updates
	reloadSource := util.NewReloadSourceFromConfig(config, []string{container.Name})
	annotations, err := createReloadedAnnotations(&reloadSource)
	if err != nil {
		logrus.Errorf("Failed to create reloaded annotations for %s! error = %v", config.ResourceName, err)
		return constants.NotUpdated
	}

	// Copy the all annotations to the item's annotations
	pa := upgradeFuncs.PodAnnotationsFunc(item)
	if pa == nil {
		return constants.NotUpdated
	}

	for k, v := range annotations {
		pa[k] = v
	}

	return constants.Updated
}

func getReloaderAnnotationKey() string {
	return fmt.Sprintf("%s/%s",
		constants.ReloaderAnnotationPrefix,
		constants.LastReloadedFromAnnotation,
	)
}

func createReloadedAnnotations(target *util.ReloadSource) (map[string]string, error) {
	if target == nil {
		return nil, errors.New("target is required")
	}

	// Create a single "last-invokeReloadStrategy-from" annotation that stores metadata about the
	// resource that caused the last invokeReloadStrategy.
	// Intentionally only storing the last item in order to keep
	// the generated annotations as small as possible.
	annotations := make(map[string]string)
	lastReloadedResourceName := getReloaderAnnotationKey()

	lastReloadedResource, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}

	annotations[lastReloadedResourceName] = string(lastReloadedResource)
	return annotations, nil
}

func getEnvVarName(resourceName string, typeName string) string {
	return constants.EnvVarPrefix + util.ConvertToEnvVarName(resourceName) + "_" + typeName
}

func updateContainerEnvVars(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) constants.Result {
	var result constants.Result
	envVar := getEnvVarName(config.ResourceName, config.Type)
	container := getContainerUsingResource(upgradeFuncs, item, config, autoReload)

	if container == nil {
		return constants.NoContainerFound
	}

	//update if env var exists
	result = updateEnvVar(upgradeFuncs.ContainersFunc(item), envVar, config.SHAValue)

	// if no existing env var exists lets create one
	if result == constants.NoEnvVarFound {
		e := v1.EnvVar{
			Name:  envVar,
			Value: config.SHAValue,
		}
		container.Env = append(container.Env, e)
		result = constants.Updated
	}
	return result
}

func updateEnvVar(containers []v1.Container, envVar string, shaData string) constants.Result {
	for i := range containers {
		envs := containers[i].Env
		for j := range envs {
			if envs[j].Name == envVar {
				if envs[j].Value != shaData {
					envs[j].Value = shaData
					return constants.Updated
				}
				return constants.NotUpdated
			}
		}
	}
	return constants.NoEnvVarFound
}
