package handler

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func doRollingUpgrade(config util.Config) {
	rollingUpgrade(config, callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		VolumesFunc:    callbacks.GetDeploymentVolumes,
		ResourceType:   "Deployment",
	})
	rollingUpgrade(config, callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDaemonSetItems,
		ContainersFunc: callbacks.GetDaemonSetContainers,
		UpdateFunc:     callbacks.UpdateDaemonSet,
		VolumesFunc:    callbacks.GetDaemonSetVolumes,
		ResourceType:   "DaemonSet",
	})
	rollingUpgrade(config, callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetStatefulSetItems,
		ContainersFunc: callbacks.GetStatefulsetContainers,
		UpdateFunc:     callbacks.UpdateStatefulset,
		VolumesFunc:    callbacks.GetStatefulsetVolumes,
		ResourceType:   "StatefulSet",
	})
}

func rollingUpgrade(config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}

	err = PerformRollingUpgrade(client, config, upgradeFuncs)
	if err != nil {
		logrus.Errorf("Rolling upgrade for '%s' failed with error = %v", config.ResourceName, err)
	}
}

// PerformRollingUpgrade upgrades the deployment if there is any change in configmap or secret data
func PerformRollingUpgrade(client kubernetes.Interface, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs) error {
	items := upgradeFuncs.ItemsFunc(client, config.Namespace)
	var err error
	for _, i := range items {
		containers := upgradeFuncs.ContainersFunc(i)
		volumes := upgradeFuncs.VolumesFunc(i)
		// find correct annotation and update the resource
		annotationValue := util.ToObjectMeta(i).Annotations[config.Annotation]
		reloaderEnabled := util.ToObjectMeta(i).Annotations[constants.ReloaderEnabledAnnotation]
		if len(containers) > 0 && annotationValue != "" {
			resourceName := util.ToObjectMeta(i).Name
			var result constants.Result
			if util.ParseBool(reloaderEnabled) {
				result = updateContainers(volumes, containers, config.ResourceName, config)
			} else {
				values := strings.Split(annotationValue, ",")
				for _, value := range values {
					if value == config.ResourceName {
						result = updateContainers(volumes, containers, value, config)
						if result == constants.Updated {
							break
						}
					}
				}
			}
			if result == constants.Updated {
				err = upgradeFuncs.UpdateFunc(client, config.Namespace, i)
				if err != nil {
					logrus.Errorf("Update for '%s' of type '%s' in namespace '%s' failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)
				} else {
					logrus.Infof("Updated '%s' of type '%s' in namespace '%s'", resourceName, upgradeFuncs.ResourceType, config.Namespace)
				}
			}
		}
	}
	return err
}

func getVolumeMountName(volumes []v1.Volume, envarPostfix string, volumeName string) string {
	for i := range volumes {
		if volumes[i].ConfigMap.Name == volumeName && envarPostfix == constants.ConfigmapEnvVarPostfix {
			return volumes[i].Name
		} else if volumes[i].Secret.SecretName == volumeName && envarPostfix == constants.SecretEnvVarPostfix {
			return volumes[i].Name
		}
	}
	return ""
}

func getContainerToUpdate(volumes []v1.Volume, containers []v1.Container, envarPostfix string, volumeName string) *v1.Container {
	// Get the volumeMountName to find volumeMount in container
	volumeMountName := getVolumeMountName(volumes, envarPostfix, volumeName)
	// Get the container with mounted configmap/secret
	if volumeMountName != "" {
		for i := range containers {
			volumeMounts := containers[i].VolumeMounts
			for j := range volumeMounts {
				if volumeMounts[j].Name == volumeMountName {
					return &containers[i]
				}
			}
		}
	}

	// Get the container with referenced secret or configmap
	for i := range containers {
		envs := containers[i].Env
		for j := range envs {
			if envs[j].ValueFrom.SecretKeyRef.LocalObjectReference.Name == volumeName {
				return &containers[i]
			} else if envs[j].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name == volumeName {
				return &containers[i]
			}
		}
	}

	return nil
}

func updateContainers(volumes []v1.Volume, containers []v1.Container, annotationValue string, config util.Config) constants.Result {
	result := constants.NotUpdated
	envar := constants.EnvVarPrefix + util.ConvertToEnvVarName(annotationValue) + "_" + config.Type
	container := getContainerToUpdate(volumes, containers, config.Type, config.ResourceName)

	if container == nil {
		return constants.NoContainerFound
	}

	//update if env var exists
	result = updateEnvVar(containers, envar, config.SHAValue)

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
