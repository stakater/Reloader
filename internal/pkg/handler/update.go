package handler

import (
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/crypto"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ResourceUpdatedHandler contains updated objects
type ResourceUpdatedHandler struct {
	Resource    interface{}
	OldResource interface{}
}

// Handle processes the updated resource
func (r ResourceUpdatedHandler) Handle() error {
	if r.Resource == nil || r.OldResource == nil {
		logrus.Errorf("Error in Handler")
	} else {
		logrus.Infof("Detected changes in object %s", r.Resource)
		// process resource based on its type
		rollingUpgrade(r, callbacks.RollingUpgradeFuncs{
			ItemsFunc:      callbacks.GetDeploymentItems,
			ContainersFunc: callbacks.GetDeploymentContainers,
			UpdateFunc:     callbacks.UpdateDeployment,
			ResourceType:   "Deployment",
		})
		rollingUpgrade(r, callbacks.RollingUpgradeFuncs{
			ItemsFunc:      callbacks.GetDaemonSetItems,
			ContainersFunc: callbacks.GetDaemonSetContainers,
			UpdateFunc:     callbacks.UpdateDaemonSet,
			ResourceType:   "DaemonSet",
		})
		rollingUpgrade(r, callbacks.RollingUpgradeFuncs{
			ItemsFunc:      callbacks.GetStatefulSetItems,
			ContainersFunc: callbacks.GetStatefulsetContainers,
			UpdateFunc:     callbacks.UpdateStatefulset,
			ResourceType:   "StatefulSet",
		})
	}
	return nil
}

func rollingUpgrade(r ResourceUpdatedHandler, upgradeFuncs callbacks.RollingUpgradeFuncs) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}

	config, envVarPostfix, oldSHAData := getConfig(r)

	if config.SHAValue != oldSHAData {
		err = PerformRollingUpgrade(client, config, envVarPostfix, upgradeFuncs)
		if err != nil {
			logrus.Fatalf("Rolling upgrade failed with error = %v", err)
		}
	} else {
		logrus.Infof("Rolling upgrade will not happend because no actual change in data has been detected")
	}
}

func getConfig(r ResourceUpdatedHandler) (util.Config, string, string) {
	var oldSHAData, envVarPostfix string
	var config util.Config
	if _, ok := r.Resource.(*v1.ConfigMap); ok {
		logrus.Infof("Performing 'Updated' action for resource of type 'configmap'")
		oldSHAData = getSHAfromConfigmap(r.OldResource.(*v1.ConfigMap).Data)
		config = getConfigmapConfig(r)
		envVarPostfix = constants.ConfigmapEnvVarPostfix
	} else if _, ok := r.Resource.(*v1.Secret); ok {
		logrus.Infof("Performing 'Updated' action for resource of type 'secret'")
		oldSHAData = getSHAfromSecret(r.OldResource.(*v1.Secret).Data)
		config = getSecretConfig(r)
		envVarPostfix = constants.SecretEnvVarPostfix
	} else {
		logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
	}
	return config, envVarPostfix, oldSHAData
}

func getConfigmapConfig(r ResourceUpdatedHandler) util.Config {
	configmap := r.Resource.(*v1.ConfigMap)
	return util.Config{
		Namespace:    configmap.Namespace,
		ResourceName: configmap.Name,
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
		SHAValue:     getSHAfromConfigmap(configmap.Data),
	}
}

func getSecretConfig(r ResourceUpdatedHandler) util.Config {
	secret := r.Resource.(*v1.Secret)
	return util.Config{
		Namespace:    secret.Namespace,
		ResourceName: secret.Name,
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
		SHAValue:     getSHAfromSecret(secret.Data),
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
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			for _, value := range values {
				if value == config.ResourceName {
					updated := updateContainers(containers, value, config.SHAValue, envarPostfix)
					if !updated {
						logrus.Warnf("Rolling upgrade did not happen")
					} else {
						err = upgradeFuncs.UpdateFunc(client, config.Namespace, i)
						if err != nil {
							logrus.Errorf("Update %s failed %v", upgradeFuncs.ResourceType, err)
						} else {
							logrus.Infof("Updated %s of type %s", config.ResourceName, upgradeFuncs.ResourceType)
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
	envar := constants.EnvVarPrefix + util.ConvertToEnvVarName(annotationValue) + envarPostfix
	logrus.Infof("Generated environment variable: %s", envar)
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
			logrus.Infof("%s environment variable does not exist, creating a new envVar", envar)
		}
	}
	return updated
}

func updateEnvVar(envs []v1.EnvVar, envar string, shaData string) bool {
	for j := range envs {
		if envs[j].Name == envar {
			logrus.Infof("%s environment variable found", envar)
			if envs[j].Value != shaData {
				logrus.Infof("Updating %s", envar)
				envs[j].Value = shaData
				return true
			}
		}
	}
	return false
}

func getSHAfromConfigmap(data map[string]string) string {
	values := []string{}
	for k, v := range data {
		values = append(values, k+"="+v)
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

func getSHAfromSecret(data map[string][]byte) string {
	values := []string{}
	for k, v := range data {
		values = append(values, k+"="+string(v[:]))
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}
