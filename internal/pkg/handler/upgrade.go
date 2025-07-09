package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	patchtypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

// GetDeploymentRollingUpgradeFuncs returns all callback funcs for a deployment
func GetDeploymentRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemFunc:           callbacks.GetDeploymentItem,
		ItemsFunc:          callbacks.GetDeploymentItems,
		AnnotationsFunc:    callbacks.GetDeploymentAnnotations,
		PodAnnotationsFunc: callbacks.GetDeploymentPodAnnotations,
		ContainersFunc:     callbacks.GetDeploymentContainers,
		InitContainersFunc: callbacks.GetDeploymentInitContainers,
		UpdateFunc:         callbacks.UpdateDeployment,
		PatchFunc:          callbacks.PatchDeployment,
		PatchTemplatesFunc: callbacks.GetPatchTemplates,
		VolumesFunc:        callbacks.GetDeploymentVolumes,
		ResourceType:       "Deployment",
		SupportsPatch:      true,
	}
}

// GetDeploymentRollingUpgradeFuncs returns all callback funcs for a cronjob
func GetCronJobCreateJobFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemFunc:           callbacks.GetCronJobItem,
		ItemsFunc:          callbacks.GetCronJobItems,
		AnnotationsFunc:    callbacks.GetCronJobAnnotations,
		PodAnnotationsFunc: callbacks.GetCronJobPodAnnotations,
		ContainersFunc:     callbacks.GetCronJobContainers,
		InitContainersFunc: callbacks.GetCronJobInitContainers,
		UpdateFunc:         callbacks.CreateJobFromCronjob,
		PatchFunc:          callbacks.PatchCronJob,
		PatchTemplatesFunc: func() callbacks.PatchTemplates { return callbacks.PatchTemplates{} },
		VolumesFunc:        callbacks.GetCronJobVolumes,
		ResourceType:       "CronJob",
		SupportsPatch:      false,
	}
}

// GetDeploymentRollingUpgradeFuncs returns all callback funcs for a cronjob
func GetJobCreateJobFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemFunc:           callbacks.GetJobItem,
		ItemsFunc:          callbacks.GetJobItems,
		AnnotationsFunc:    callbacks.GetJobAnnotations,
		PodAnnotationsFunc: callbacks.GetJobPodAnnotations,
		ContainersFunc:     callbacks.GetJobContainers,
		InitContainersFunc: callbacks.GetJobInitContainers,
		UpdateFunc:         callbacks.ReCreateJobFromjob,
		PatchFunc:          callbacks.PatchJob,
		PatchTemplatesFunc: func() callbacks.PatchTemplates { return callbacks.PatchTemplates{} },
		VolumesFunc:        callbacks.GetJobVolumes,
		ResourceType:       "Job",
		SupportsPatch:      false,
	}
}

// GetDaemonSetRollingUpgradeFuncs returns all callback funcs for a daemonset
func GetDaemonSetRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemFunc:           callbacks.GetDaemonSetItem,
		ItemsFunc:          callbacks.GetDaemonSetItems,
		AnnotationsFunc:    callbacks.GetDaemonSetAnnotations,
		PodAnnotationsFunc: callbacks.GetDaemonSetPodAnnotations,
		ContainersFunc:     callbacks.GetDaemonSetContainers,
		InitContainersFunc: callbacks.GetDaemonSetInitContainers,
		UpdateFunc:         callbacks.UpdateDaemonSet,
		PatchFunc:          callbacks.PatchDaemonSet,
		PatchTemplatesFunc: callbacks.GetPatchTemplates,
		VolumesFunc:        callbacks.GetDaemonSetVolumes,
		ResourceType:       "DaemonSet",
		SupportsPatch:      true,
	}
}

// GetStatefulSetRollingUpgradeFuncs returns all callback funcs for a statefulSet
func GetStatefulSetRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemFunc:           callbacks.GetStatefulSetItem,
		ItemsFunc:          callbacks.GetStatefulSetItems,
		AnnotationsFunc:    callbacks.GetStatefulSetAnnotations,
		PodAnnotationsFunc: callbacks.GetStatefulSetPodAnnotations,
		ContainersFunc:     callbacks.GetStatefulSetContainers,
		InitContainersFunc: callbacks.GetStatefulSetInitContainers,
		UpdateFunc:         callbacks.UpdateStatefulSet,
		PatchFunc:          callbacks.PatchStatefulSet,
		PatchTemplatesFunc: callbacks.GetPatchTemplates,
		VolumesFunc:        callbacks.GetStatefulSetVolumes,
		ResourceType:       "StatefulSet",
		SupportsPatch:      true,
	}
}

// GetArgoRolloutRollingUpgradeFuncs returns all callback funcs for a rollout
func GetArgoRolloutRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemFunc:           callbacks.GetRolloutItem,
		ItemsFunc:          callbacks.GetRolloutItems,
		AnnotationsFunc:    callbacks.GetRolloutAnnotations,
		PodAnnotationsFunc: callbacks.GetRolloutPodAnnotations,
		ContainersFunc:     callbacks.GetRolloutContainers,
		InitContainersFunc: callbacks.GetRolloutInitContainers,
		UpdateFunc:         callbacks.UpdateRollout,
		PatchFunc:          callbacks.PatchRollout,
		PatchTemplatesFunc: func() callbacks.PatchTemplates { return callbacks.PatchTemplates{} },
		VolumesFunc:        callbacks.GetRolloutVolumes,
		ResourceType:       "Rollout",
		SupportsPatch:      false,
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

	if options.IsArgoRollouts {
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

	for _, item := range items {
		err := retryOnConflict(retry.DefaultRetry, func(fetchResource bool) error {
			return upgradeResource(clients, config, upgradeFuncs, collectors, recorder, strategy, item, fetchResource)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func retryOnConflict(backoff wait.Backoff, fn func(_ bool) error) error {
	var lastError error
	fetchResource := false // do not fetch resource on first attempt, already done by ItemsFunc
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := fn(fetchResource)
		fetchResource = true
		switch {
		case err == nil:
			return true, nil
		case apierrors.IsConflict(err):
			lastError = err
			return false, nil
		default:
			return false, err
		}
	})
	if wait.Interrupted(err) {
		err = lastError
	}
	return err
}

func upgradeResource(clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, recorder record.EventRecorder, strategy invokeStrategy, resource runtime.Object, fetchResource bool) error {
	accessor, err := meta.Accessor(resource)
	if err != nil {
		return err
	}

	resourceName := accessor.GetName()
	if fetchResource {
		resource, err = upgradeFuncs.ItemFunc(clients, resourceName, config.Namespace)
		if err != nil {
			return err
		}
	}

	reloadCheckResult := util.ShouldReload(config, upgradeFuncs.ResourceType, upgradeFuncs.AnnotationsFunc(resource), upgradeFuncs.PodAnnotationsFunc(resource))
	if !reloadCheckResult.ShouldReload {
		return nil
	}

	strategyResult := strategy(upgradeFuncs, resource, config, reloadCheckResult.AutoReload)

	if strategyResult.Result != constants.Updated {
		return nil
	}

	if upgradeFuncs.SupportsPatch && strategyResult.Patch != nil {
		err = upgradeFuncs.PatchFunc(clients, config.Namespace, resource, strategyResult.Patch.Type, strategyResult.Patch.Bytes)
	} else {
		err = upgradeFuncs.UpdateFunc(clients, config.Namespace, resource)
	}

	if err != nil {
		message := fmt.Sprintf("Update for '%s' of type '%s' in namespace '%s' failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)
		logrus.Errorf("Update for '%s' of type '%s' in namespace '%s' failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)

		collectors.Reloaded.With(prometheus.Labels{"success": "false"}).Inc()
		collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "false", "namespace": config.Namespace}).Inc()
		if recorder != nil {
			recorder.Event(resource, v1.EventTypeWarning, "ReloadFail", message)
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
			recorder.Event(resource, v1.EventTypeNormal, "Reloaded", message)
		}
		if ok && alert_on_reload == "true" {
			msg := fmt.Sprintf(
				"Reloader detected changes in *%s* of type *%s* in namespace *%s*. Hence reloaded *%s* of type *%s* in namespace *%s*",
				config.ResourceName, config.Type, config.Namespace, resourceName, upgradeFuncs.ResourceType, config.Namespace)
			alert.SendWebhookAlert(msg)
		}
	}

	return nil
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

type Patch struct {
	Type  patchtypes.PatchType
	Bytes []byte
}

type InvokeStrategyResult struct {
	Result constants.Result
	Patch  *Patch
}

type invokeStrategy func(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) InvokeStrategyResult

func invokeReloadStrategy(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) InvokeStrategyResult {
	if options.ReloadStrategy == constants.AnnotationsReloadStrategy {
		return updatePodAnnotations(upgradeFuncs, item, config, autoReload)
	}
	return updateContainerEnvVars(upgradeFuncs, item, config, autoReload)
}

func updatePodAnnotations(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) InvokeStrategyResult {
	container := getContainerUsingResource(upgradeFuncs, item, config, autoReload)
	if container == nil {
		return InvokeStrategyResult{constants.NoContainerFound, nil}
	}

	// Generate reloaded annotations. Attaching this to the item's annotation will trigger a rollout
	// Note: the data on this struct is purely informational and is not used for future updates
	reloadSource := util.NewReloadSourceFromConfig(config, []string{container.Name})
	annotations, patch, err := createReloadedAnnotations(&reloadSource, upgradeFuncs)
	if err != nil {
		logrus.Errorf("Failed to create reloaded annotations for %s! error = %v", config.ResourceName, err)
		return InvokeStrategyResult{constants.NotUpdated, nil}
	}

	// Copy the all annotations to the item's annotations
	pa := upgradeFuncs.PodAnnotationsFunc(item)
	if pa == nil {
		return InvokeStrategyResult{constants.NotUpdated, nil}
	}

	for k, v := range annotations {
		pa[k] = v
	}

	return InvokeStrategyResult{constants.Updated, &Patch{Type: patchtypes.StrategicMergePatchType, Bytes: patch}}
}

func getReloaderAnnotationKey() string {
	return fmt.Sprintf("%s/%s",
		constants.ReloaderAnnotationPrefix,
		constants.LastReloadedFromAnnotation,
	)
}

func createReloadedAnnotations(target *util.ReloadSource, upgradeFuncs callbacks.RollingUpgradeFuncs) (map[string]string, []byte, error) {
	if target == nil {
		return nil, nil, errors.New("target is required")
	}

	// Create a single "last-invokeReloadStrategy-from" annotation that stores metadata about the
	// resource that caused the last invokeReloadStrategy.
	// Intentionally only storing the last item in order to keep
	// the generated annotations as small as possible.
	annotations := make(map[string]string)
	lastReloadedResourceName := getReloaderAnnotationKey()

	lastReloadedResource, err := json.Marshal(target)
	if err != nil {
		return nil, nil, err
	}

	annotations[lastReloadedResourceName] = string(lastReloadedResource)

	var patch []byte
	if upgradeFuncs.SupportsPatch {
		escapedValue, err := jsonEscape(annotations[lastReloadedResourceName])
		if err != nil {
			return nil, nil, err
		}
		patch = fmt.Appendf(nil, upgradeFuncs.PatchTemplatesFunc().AnnotationTemplate, lastReloadedResourceName, escapedValue)
	}

	return annotations, patch, nil
}

func getEnvVarName(resourceName string, typeName string) string {
	return constants.EnvVarPrefix + util.ConvertToEnvVarName(resourceName) + "_" + typeName
}

func updateContainerEnvVars(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) InvokeStrategyResult {
	envVar := getEnvVarName(config.ResourceName, config.Type)
	container := getContainerUsingResource(upgradeFuncs, item, config, autoReload)

	if container == nil {
		return InvokeStrategyResult{constants.NoContainerFound, nil}
	}

	//update if env var exists
	updateResult := updateEnvVar(container, envVar, config.SHAValue)

	// if no existing env var exists lets create one
	if updateResult == constants.NoEnvVarFound {
		e := v1.EnvVar{
			Name:  envVar,
			Value: config.SHAValue,
		}
		container.Env = append(container.Env, e)
		updateResult = constants.Updated
	}

	var patch []byte
	if upgradeFuncs.SupportsPatch {
		patch = fmt.Appendf(nil, upgradeFuncs.PatchTemplatesFunc().EnvVarTemplate, container.Name, envVar, config.SHAValue)
	}

	return InvokeStrategyResult{updateResult, &Patch{Type: patchtypes.StrategicMergePatchType, Bytes: patch}}
}

func updateEnvVar(container *v1.Container, envVar string, shaData string) constants.Result {
	envs := container.Env
	for j := range envs {
		if envs[j].Name == envVar {
			if envs[j].Value != shaData {
				envs[j].Value = shaData
				return constants.Updated
			}
			return constants.NotUpdated
		}
	}

	return constants.NoEnvVarFound
}

func jsonEscape(toEscape string) (string, error) {
	bytes, err := json.Marshal(toEscape)
	if err != nil {
		return "", err
	}
	escaped := string(bytes)
	return escaped[1 : len(escaped)-1], nil
}
