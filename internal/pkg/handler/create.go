package handler

import (
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/util"
	v1 "k8s.io/api/core/v1"
)

// ResourceCreatedHandler contains new objects
type ResourceCreatedHandler struct {
	Resource   interface{}
	Collectors metrics.Collectors
}

// Handle processes the newly created resource
func (r ResourceCreatedHandler) Handle() error {
	if r.Resource == nil {
		logrus.Errorf("Resource creation handler received nil resource")
	} else {
		config, _ := r.GetConfig()
		// process resource based on its type
		doRollingUpgrade(config, r.Collectors)
	}
	return nil
}

// GetConfig gets configurations containing SHA, annotations, namespace and resource name
func (r ResourceCreatedHandler) GetConfig() (util.Config, string) {
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
