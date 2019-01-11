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
		resourceName := util.ToObjectMeta(i).Name
		// find correct annotation and update the resource
		annotationValue := util.ToObjectMeta(i).Annotations[config.Annotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				if value == config.ResourceName {
					updated := updateContainers(containers, value, config.SHAValue, envarPostfix)
					if !updated {
						logrus.Warnf("Rolling upgrade failed because no container found to add environment variable in %s of type %s in namespace: %s", resourceName, upgradeFuncs.ResourceType, config.Namespace)
					} else {
						err = upgradeFuncs.UpdateFunc(client, config.Namespace, i)
						if err != nil {
							logrus.Errorf("Update for %s of type %s in namespace %s failed with error %v", resourceName, upgradeFuncs.ResourceType, config.Namespace, err)
						} else {
							logrus.Infof("Updated %s of type %s in namespace: %s ", resourceName, upgradeFuncs.ResourceType, config.Namespace)
						}
						break
					}
				}
			}
		}
	}
	return err
}

func updateContainers(containers []v1.Container, annotationValue string, shaData string, envarPostfix string) bool {
	updated := false
	envar := constants.EnvVarPrefix + util.ConvertToEnvVarName(annotationValue) + "_" + envarPostfix
	for i := range containers {
		envs := containers[i].Env

		//update if env var exists
		updated = updateEnvVar(envs, envar, shaData)

		// if no existing env var exists lets create one
		if !updated {
			e := v1.EnvVar{
				Name:  envar,
				Value: shaData,
			}
			containers[i].Env = append(containers[i].Env, e)
			updated = true
		}
	}
	return updated
}

func updateEnvVar(envs []v1.EnvVar, envar string, shaData string) bool {
	for j := range envs {
		if envs[j].Name == envar {
			if envs[j].Value != shaData {
				envs[j].Value = shaData
				return true
			}
		}
	}
	return false
}
