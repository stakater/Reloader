package handler

import (
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/common"
)

// ResourceCreatedHandler contains new objects
type ResourceCreatedHandler struct {
	Resource    interface{}
	Collectors  metrics.Collectors
	Recorder    record.EventRecorder
	EnqueueTime time.Time // Time when this handler was added to the queue
}

// GetEnqueueTime returns when this handler was enqueued
func (r ResourceCreatedHandler) GetEnqueueTime() time.Time {
	return r.EnqueueTime
}

// Handle processes the newly created resource
func (r ResourceCreatedHandler) Handle() error {
	startTime := time.Now()
	result := "error"

	defer func() {
		r.Collectors.RecordReconcile(result, time.Since(startTime))
	}()

	if r.Resource == nil {
		logrus.Errorf("Resource creation handler received nil resource")
		return nil
	}

	config, _ := r.GetConfig()
	// Send webhook
	if options.WebhookUrl != "" {
		err := sendUpgradeWebhook(config, options.WebhookUrl)
		if err == nil {
			result = "success"
		}
		return err
	}
	// process resource based on its type
	err := doRollingUpgrade(config, r.Collectors, r.Recorder, invokeReloadStrategy)
	if err == nil {
		result = "success"
	}
	return err
}

// GetConfig gets configurations containing SHA, annotations, namespace and resource name
func (r ResourceCreatedHandler) GetConfig() (common.Config, string) {
	var oldSHAData string
	var config common.Config
	if cm, ok := r.Resource.(*v1.ConfigMap); ok {
		config = common.GetConfigmapConfig(cm)
	} else if secret, ok := r.Resource.(*v1.Secret); ok {
		config = common.GetSecretConfig(secret)
	} else {
		logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
	}
	return config, oldSHAData
}
