package handler

import (
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/util"
	"k8s.io/api/core/v1"
)

// ResourceCreatedHandler contains new objects
type ResourceCreatedHandler struct {
	Resource interface{}
}

// Handle processes the newly created resource
func (r ResourceCreatedHandler) Handle() error {
	if r.Resource == nil {
		logrus.Errorf("Resource creation handler received nil resource")
	} else {
		config, envVarPostfix, _ := r.GetConfig()
		logrus.Infof("Resource '%s' of type '%s' in namespace '%s' has been created", config.ResourceName, envVarPostfix, config.Namespace)
		// process resource based on its type
		doRollingUpgrade(config, envVarPostfix)
	}
	return nil
}

// GetConfig gets configurations containing SHA, annotations, namespace and resource name
func (r ResourceCreatedHandler) GetConfig() (util.Config, string, string) {
	var oldSHAData, envVarPostfix string
	var config util.Config
	if _, ok := r.Resource.(*v1.ConfigMap); ok {
		config = util.GetConfigmapConfig(r.Resource.(*v1.ConfigMap))
		envVarPostfix = constants.ConfigmapEnvVarPostfix
	} else if _, ok := r.Resource.(*v1.Secret); ok {
		config = util.GetSecretConfig(r.Resource.(*v1.Secret))
		envVarPostfix = constants.SecretEnvVarPostfix
	} else {
		logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
	}
	return config, envVarPostfix, oldSHAData
}
