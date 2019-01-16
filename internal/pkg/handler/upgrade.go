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

func doRollingUpgrade(config util.Config, envVarPostfix string) {
	rollingUpgrade(config, envVarPostfix, callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		ResourceType:   "Deployment",
	})
	rollingUpgrade(config, envVarPostfix, callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDaemonSetItems,
		ContainersFunc: callbacks.GetDaemonSetContainers,
		UpdateFunc:     callbacks.UpdateDaemonSet,
		ResourceType:   "DaemonSet",
	})
	rollingUpgrade(config, envVarPostfix, callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetStatefulSetItems,
		ContainersFunc: callbacks.GetStatefulsetContainers,
		UpdateFunc:     callbacks.UpdateStatefulset,
		ResourceType:   "StatefulSet",
	})
}

func rollingUpgrade(config util.Config, envarPostfix string, upgradeFuncs callbacks.RollingUpgradeFuncs) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}

	err = PerformRollingUpgrade(client, config, envarPostfix, upgradeFuncs)
	if err != nil {
		logrus.Errorf("Rolling upgrade for %s failed with error = %v", config.ResourceName, err)
	}
}

// PerformRollingUpgrade upgrades the deployment if there is any change in configmap or secret data
func PerformRollingUpgrade(client kubernetes.Interface, config util.Config, envarPostfix string, upgradeFuncs callbacks.RollingUpgradeFuncs) error {
	items := upgradeFuncs.ItemsFunc(client, config.Namespace)
	var err error
	for _, i := range items {
		containers := upgradeFuncs.ContainersFunc(i)
		// find correct annotation and update the resource
		annotationValue := util.ToObjectMeta(i).Annotations[config.Annotation]
		if len(containers) > 0 && annotationValue != "" {
			resourceName := util.ToObjectMeta(i).Name
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				if value == config.ResourceName {
					result := updateContainers(containers, value, config.SHAValue, envarPostfix)
					if result == constants.Updated {
						err = upgradeFuncs.UpdateFunc(client, config.Namespace, i)
						if err != nil {
							logrus.Errorf("Update for %s of type %s in namespace %s failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)
						} else {
							logrus.Infof("Updated %s of type %s in namespace: %s", resourceName, upgradeFuncs.ResourceType, config.Namespace)
						}
						break
					}
				}
			}
		}
	}
	return err
}

func updateContainers(containers []v1.Container, annotationValue string, shaData string, envarPostfix string) constants.Result {
	result := constants.NotUpdated
	envar := constants.EnvVarPrefix + util.ConvertToEnvVarName(annotationValue) + "_" + envarPostfix

	//update if env var exists
	result = updateEnvVar(containers, envar, shaData)

	// if no existing env var exists lets create one
	if result == constants.NoEnvFound {
		e := v1.EnvVar{
			Name:  envar,
			Value: shaData,
		}
		containers[0].Env = append(containers[0].Env, e)
		result = constants.Updated
	}
	return result
}

func updateEnvVar(containers []v1.Container, envar string, shaData string) constants.Result {
	for i := range containers {
		for j := range containers[i].Env {
			if containers[i].Env[j].Name == envar {
				if containers[i].Env[j].Value != shaData {
					containers[i].Env[j].Value = shaData
					return constants.Updated
				}
				return constants.NotUpdated
			}
		}
	}
	return constants.NoEnvFound
}
