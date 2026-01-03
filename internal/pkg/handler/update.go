package handler

import (
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// ResourceUpdatedHandler contains updated objects
type ResourceUpdatedHandler struct {
	Resource    interface{}
	OldResource interface{}
	Collectors  metrics.Collectors
	Recorder    record.EventRecorder
}

// Handle processes the updated resource
func (r ResourceUpdatedHandler) Handle() error {
	if r.Resource == nil || r.OldResource == nil {
		logrus.Errorf("Resource update handler received nil resource")
	} else {
		config, oldSHAData := r.GetConfig()
		if config.SHAValue != oldSHAData {
			// Send a webhook if update
			if options.WebhookUrl != "" {
				return sendUpgradeWebhook(config, options.WebhookUrl)
			}
			// process resource based on its type
			return doRollingUpgrade(config, r.Collectors, r.Recorder, invokeReloadStrategy)
		}
	}
	return nil
}

// GetConfig gets configurations containing SHA, annotations, namespace and resource name
func (r ResourceUpdatedHandler) GetConfig() (common.Config, string) {
	var oldSHAData string
	var config common.Config
	switch res := r.Resource.(type) {
	case *v1.ConfigMap:
		oldSHAData = util.GetSHAfromConfigmap(r.OldResource.(*v1.ConfigMap))
		config = common.GetConfigmapConfig(res)
	case *v1.Secret:
		oldSHAData = util.GetSHAfromSecret(r.OldResource.(*v1.Secret).Data)
		config = common.GetSecretConfig(res)
	case *csiv1.SecretProviderClassPodStatus:
		oldSHAData = util.GetSHAfromSecretProviderClassPodStatus(r.OldResource.(*csiv1.SecretProviderClassPodStatus).Status)
		config = common.GetSecretProviderClassPodStatusConfig(res)
	default:
		logrus.Warnf("Invalid resource: Resource should be 'Secret', 'Configmap' or 'SecretProviderClassPodStatus' but found, %v", r.Resource)
	}
	return config, oldSHAData
}
