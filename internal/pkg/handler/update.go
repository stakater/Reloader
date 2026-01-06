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
	var oldSHAData string
	var config common.Config
	if _, ok := r.Resource.(*v1.ConfigMap); ok {
		oldSHAData = util.GetSHAfromConfigmap(r.OldResource.(*v1.ConfigMap))
		config = common.GetConfigmapConfig(r.Resource.(*v1.ConfigMap))
	} else if _, ok := r.Resource.(*v1.Secret); ok {
		oldSHAData = util.GetSHAfromSecret(r.OldResource.(*v1.Secret).Data)
		config = common.GetSecretConfig(r.Resource.(*v1.Secret))
	} else {
		logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
	}
	return config, oldSHAData
}
