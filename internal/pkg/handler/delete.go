package handler

import (
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// ResourceDeleteHandler contains new objects
type ResourceDeleteHandler struct {
	Resource   interface{}
	Collectors metrics.Collectors
	Recorder   record.EventRecorder
}

// Handle processes resources being deleted
func (r ResourceDeleteHandler) Handle() error {
	if r.Resource == nil {
		logrus.Errorf("Resource delete handler received nil resource")
	} else {
		config, _ := r.GetConfig()
		// Send webhook
		if options.WebhookUrl != "" {
			return sendUpgradeWebhook(config, options.WebhookUrl)
		}
		// process resource based on its type
		return doRollingUpgrade(config, r.Collectors, r.Recorder, invokeDeleteStrategy)
	}
	return nil
}

// GetConfig gets configurations containing SHA, annotations, namespace and resource name
func (r ResourceDeleteHandler) GetConfig() (util.Config, string) {
	var oldSHAData string
	var config util.Config
	if _, ok := r.Resource.(*v1.ConfigMap); ok {
		config = util.GetConfigmapConfig(r.Resource.(*v1.ConfigMap))
	} else if _, ok := r.Resource.(*v1.Secret); ok {
		config = util.GetSecretConfig(r.Resource.(*v1.Secret))
	} else {
		logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
	}
	return config, oldSHAData
}

func invokeDeleteStrategy(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) (constants.Result, []byte) {
	if options.ReloadStrategy == constants.AnnotationsReloadStrategy {
		return removePodAnnotations(upgradeFuncs, item, config, autoReload)
	}

	return removeContainerEnvVars(upgradeFuncs, item, config, autoReload), nil
}

func removePodAnnotations(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) (constants.Result, []byte) {
	config.SHAValue = testutil.GetSHAfromEmptyData()
	return updatePodAnnotations(upgradeFuncs, item, config, autoReload)
}

func removeContainerEnvVars(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config util.Config, autoReload bool) constants.Result {
	envVar := getEnvVarName(config.ResourceName, config.Type)
	container := getContainerUsingResource(upgradeFuncs, item, config, autoReload)

	if container == nil {
		return constants.NoContainerFound
	}

	//remove if env var exists
	containers := upgradeFuncs.ContainersFunc(item)
	for i := range containers {
		envs := containers[i].Env
		index := -1
		for j := range envs {
			if envs[j].Name == envVar {
				index = j
				break
			}
		}
		if index != -1 {
			containers[i].Env = append(containers[i].Env[:index], containers[i].Env[index+1:]...)
			return constants.Updated
		}
	}

	return constants.NotUpdated
}
