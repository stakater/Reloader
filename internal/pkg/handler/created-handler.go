package handler

import (
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

// ResourceCreatedHandler contains new objects
type ResourceCreatedHandler struct {
	Resource interface{}
}

// Handle processes the newly created resource
func (r ResourceCreatedHandler) Handle() error {
	if r.Resource == nil {
		logrus.Errorf("Error in Handler")
	} else {
		logrus.Infof("Detected changes in object %s", r.Resource)
		// process resource based on its type
		if _, ok := r.Resource.(*v1.ConfigMap); ok {
			logrus.Infof("Performing 'Added' action for resource of type 'configmap'")
		} else if _, ok := r.Resource.(*v1.Secret); ok {
			logrus.Infof("Performing 'Added' action for resource of type 'secret'")
		} else {
			logrus.Warnf("Invalid resource: Resource should be 'Secret' or 'Configmap' but found, %v", r.Resource)
		}
	}
	return nil
}
