package handler

import (
	"time"

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
	EnqueueTime time.Time // Time when this handler was added to the queue
}

// GetEnqueueTime returns when this handler was enqueued
func (r ResourceUpdatedHandler) GetEnqueueTime() time.Time {
	return r.EnqueueTime
}

// Handle processes the updated resource
func (r ResourceUpdatedHandler) Handle() error {
	startTime := time.Now()
	result := "success"

	defer func() {
		r.Collectors.RecordReconcile(result, time.Since(startTime))
	}()

	if r.Resource == nil || r.OldResource == nil {
		logrus.Errorf("Resource update handler received nil resource")
		result = "error"
	} else {
		config, oldSHAData := r.GetConfig()
		if config.SHAValue != oldSHAData {
			// Send a webhook if update
			if options.WebhookUrl != "" {
				err := sendUpgradeWebhook(config, options.WebhookUrl)
				if err != nil {
					result = "error"
				}
				return err
			}
			// process resource based on its type
			err := doRollingUpgrade(config, r.Collectors, r.Recorder, invokeReloadStrategy)
			if err != nil {
				result = "error"
			}
			return err
		} else {
			// No data change - skip
			result = "skipped"
			r.Collectors.RecordSkipped("no_data_change")
		}
	}
	return nil
}

// GetConfig gets configurations containing SHA, annotations, namespace and resource name
func (r ResourceUpdatedHandler) GetConfig() (common.Config, string) {
	var (
		oldSHAData string
		config     common.Config
	)

	switch res := r.Resource.(type) {
	case *v1.ConfigMap:
		if old, ok := r.OldResource.(*v1.ConfigMap); ok && old != nil {
			oldSHAData = util.GetSHAfromConfigmap(old)
		}
		config = common.GetConfigmapConfig(res)

	case *v1.Secret:
		if old, ok := r.OldResource.(*v1.Secret); ok && old != nil {
			oldSHAData = util.GetSHAfromSecret(old.Data)
		}
		config = common.GetSecretConfig(res)

	case *csiv1.SecretProviderClassPodStatus:
		if old, ok := r.OldResource.(*csiv1.SecretProviderClassPodStatus); ok && old != nil && old.Status.Objects != nil {
			oldSHAData = util.GetSHAfromSecretProviderClassPodStatus(old.Status)
		}
		config = common.GetSecretProviderClassPodStatusConfig(res)
	default:
		logrus.Warnf("Invalid resource: Resource should be 'Secret', 'Configmap' or 'SecretProviderClassPodStatus' but found, %T", r.Resource)
	}
	return config, oldSHAData
}
