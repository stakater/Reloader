package handler

import (
	"fmt"
	"slices"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/common"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	patchtypes "k8s.io/apimachinery/pkg/types"
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
func (r ResourceDeleteHandler) GetConfig() (common.Config, string) {
	var oldSHAData string
	var config common.Config
	if _, ok := r.Resource.(*v1.ConfigMap); ok {
		config = common.GetConfigmapConfig(r.Resource.(*v1.ConfigMap))
	} else if _, ok := r.Resource.(*v1.Secret); ok {
		config = common.GetSecretConfig(r.Resource.(*v1.Secret))
	} else {
		logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
	}
	return config, oldSHAData
}

func invokeDeleteStrategy(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config common.Config, autoReload bool) InvokeStrategyResult {
	if options.ReloadStrategy == constants.AnnotationsReloadStrategy {
		return removePodAnnotations(upgradeFuncs, item, config, autoReload)
	}

	return removeContainerEnvVars(upgradeFuncs, item, config, autoReload)
}

func removePodAnnotations(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config common.Config, autoReload bool) InvokeStrategyResult {
	config.SHAValue = testutil.GetSHAfromEmptyData()
	return updatePodAnnotations(upgradeFuncs, item, config, autoReload)
}

func removeContainerEnvVars(upgradeFuncs callbacks.RollingUpgradeFuncs, item runtime.Object, config common.Config, autoReload bool) InvokeStrategyResult {
	envVar := getEnvVarName(config.ResourceName, config.Type)
	container := getContainerUsingResource(upgradeFuncs, item, config, autoReload)

	if container == nil {
		return InvokeStrategyResult{constants.NoContainerFound, nil}
	}

	//remove if env var exists
	if len(container.Env) > 0 {
		index := slices.IndexFunc(container.Env, func(envVariable v1.EnvVar) bool {
			return envVariable.Name == envVar
		})
		if index != -1 {
			var patch []byte
			if upgradeFuncs.SupportsPatch {
				containers := upgradeFuncs.ContainersFunc(item)
				containerIndex := slices.IndexFunc(containers, func(c v1.Container) bool {
					return c.Name == container.Name
				})
				patch = fmt.Appendf(nil, upgradeFuncs.PatchTemplatesFunc().DeleteEnvVarTemplate, containerIndex, index)
			}

			container.Env = append(container.Env[:index], container.Env[index+1:]...)
			return InvokeStrategyResult{constants.Updated, &Patch{Type: patchtypes.JSONPatchType, Bytes: patch}}
		}
	}

	return InvokeStrategyResult{constants.NotUpdated, nil}
}
