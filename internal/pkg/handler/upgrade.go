package handler

import (
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	v1 "k8s.io/api/core/v1"
)

// GetDeploymentRollingUpgradeFuncs returns all callback funcs for a deployment
func GetDeploymentRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetDeploymentItems,
		ContainersFunc:     callbacks.GetDeploymentContainers,
		InitContainersFunc: callbacks.GetDeploymentInitContainers,
		UpdateFunc:         callbacks.UpdateDeployment,
		VolumesFunc:        callbacks.GetDeploymentVolumes,
		ResourceType:       "Deployment",
	}
}

// GetDaemonSetRollingUpgradeFuncs returns all callback funcs for a daemonset
func GetDaemonSetRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetDaemonSetItems,
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
		ContainersFunc:     callbacks.GetStatefulsetContainers,
		InitContainersFunc: callbacks.GetStatefulsetInitContainers,
		UpdateFunc:         callbacks.UpdateStatefulset,
		VolumesFunc:        callbacks.GetStatefulsetVolumes,
		ResourceType:       "StatefulSet",
	}
}

// GetDeploymentConfigRollingUpgradeFuncs returns all callback funcs for a deploymentConfig
func GetDeploymentConfigRollingUpgradeFuncs() callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		ItemsFunc:          callbacks.GetDeploymentConfigItems,
		ContainersFunc:     callbacks.GetDeploymentConfigContainers,
		InitContainersFunc: callbacks.GetDeploymentConfigInitContainers,
		UpdateFunc:         callbacks.UpdateDeploymentConfig,
		VolumesFunc:        callbacks.GetDeploymentConfigVolumes,
		ResourceType:       "DeploymentConfig",
	}
}

func doRollingUpgrade(config util.Config) {
	clients := kube.GetClients()

	rollingUpgrade(clients, config, GetDeploymentRollingUpgradeFuncs())
	rollingUpgrade(clients, config, GetDaemonSetRollingUpgradeFuncs())
	rollingUpgrade(clients, config, GetStatefulSetRollingUpgradeFuncs())

	if kube.IsOpenshift {
		rollingUpgrade(clients, config, GetDeploymentConfigRollingUpgradeFuncs())
	}
}

func rollingUpgrade(clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs) {

	err := PerformRollingUpgrade(clients, config, upgradeFuncs)
	if err != nil {
		logrus.Errorf("Rolling upgrade for '%s' failed with error = %v", config.ResourceName, err)
	}
}

// PerformRollingUpgrade upgrades the deployment if there is any change in configmap or secret data
func PerformRollingUpgrade(clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs) error {
	items := upgradeFuncs.ItemsFunc(clients, config.Namespace)
	var err error
	for _, i := range items {
		// find correct annotation and update the resource
		annotationValue := util.ToObjectMeta(i).Annotations[config.Annotation]
		reloaderEnabledValue := util.ToObjectMeta(i).Annotations[options.ReloaderAutoAnnotation]
		result := constants.NotUpdated
		reloaderEnabled, err := strconv.ParseBool(reloaderEnabledValue)
		if err == nil && reloaderEnabled {
			result = updateContainers(upgradeFuncs, i, config, true)
		}

		if result != constants.Updated && annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				if value == config.ResourceName {
					result = updateContainers(upgradeFuncs, i, config, false)
					if result == constants.Updated {
						break
					}
				}
			}
		}

		if result == constants.Updated {
			err = upgradeFuncs.UpdateFunc(clients, config.Namespace, i)
			resourceName := util.ToObjectMeta(i).Name
			if err != nil {
				logrus.Errorf("Update for '%s' of type '%s' in namespace '%s' failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)
			} else {
				logrus.Infof("Changes detected in '%s' of type '%s' in namespace '%s'", config.ResourceName, config.Type, config.Namespace)
				logrus.Infof("Updated '%s' of type '%s' in namespace '%s'", resourceName, upgradeFuncs.ResourceType, config.Namespace)
			}
		}
	}
	return err
}

func getVolumeMountName(volumes []v1.Volume, mountType string, volumeName string) string {
	for i := range volumes {
		if mountType == constants.ConfigmapEnvVarPostfix && volumes[i].ConfigMap != nil && volumes[i].ConfigMap.Name == volumeName {
			return volumes[i].Name
		} else if mountType == constants.SecretEnvVarPostfix && volumes[i].Secret != nil && volumes[i].Secret.SecretName == volumeName {
			return volumes[i].Name
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

func getContainerToUpdate(upgradeFuncs callbacks.RollingUpgradeFuncs, item interface{}, config util.Config, autoReload bool) *v1.Container {
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

func updateContainers(upgradeFuncs callbacks.RollingUpgradeFuncs, item interface{}, config util.Config, autoReload bool) constants.Result {
	var result constants.Result
	envar := constants.EnvVarPrefix + util.ConvertToEnvVarName(config.ResourceName) + "_" + config.Type
	container := getContainerToUpdate(upgradeFuncs, item, config, autoReload)

	if container == nil {
		return constants.NoContainerFound
	}

	//update if env var exists
	result = updateEnvVar(upgradeFuncs.ContainersFunc(item), envar, config.SHAValue)

	// if no existing env var exists lets create one
	if result == constants.NoEnvVarFound {
		e := v1.EnvVar{
			Name:  envar,
			Value: config.SHAValue,
		}
		container.Env = append(container.Env, e)
		result = constants.Updated
	}
	return result
}

func updateEnvVar(containers []v1.Container, envar string, shaData string) constants.Result {
	for i := range containers {
		envs := containers[i].Env
		for j := range envs {
			if envs[j].Name == envar {
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
