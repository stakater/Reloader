package handler

import (
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

// ResourceHandler handles the creation and update of resources
type ResourceHandler interface {
	Handle() error
}

// Handle processes the newly created resource
func (r ResourceCreatedHandler) Handle() error {
	if r.Resource == nil {
		logrus.Infof("Error in Handler")
	} else {
		logrus.Infof("Detected changes in object %s", r.Resource)
		// process resource based on its type
		if _, ok := r.Resource.(*v1.ConfigMap); ok {
			logrus.Infof("Performing 'Added' action for resource of type 'configmap'")
		} else if _, ok := r.Resource.(*v1.Secret); ok {
			logrus.Infof("Performing 'Added' action for resource of type 'secret'")
		} else {
			logrus.Infof("Invalid resource")
		}
	}
	return nil
}

// Handle processes the updated resource
func (r ResourceUpdatedHandler) Handle() error {
	if r.Resource == nil || r.OldResource == nil {
		logrus.Infof("Error in Handler")
	} else {
		logrus.Infof("Detected changes in object %s", r.Resource)
		// process resource based on its type
		if _, ok := r.Resource.(*v1.ConfigMap); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'configmap'")
		} else if _, ok := r.Resource.(*v1.Secret); ok {
			logrus.Infof("Performing 'Updated' action for resource of type 'secret'")
		} else {
			logrus.Infof("Invalid resource")
		}
	}
	return nil
}
