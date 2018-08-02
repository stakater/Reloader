package handler

import (
	"github.com/sirupsen/logrus"
)

// ResourceCreatedHandler contains new objects
type ResourceCreatedHandler struct {
	Resource interface{}
}

// Handle processes the newly created resource
func (r ResourceCreatedHandler) Handle() error {
	if r.Resource == nil {
		logrus.Errorf("Resource creation handler received nil resource")
	}
	return nil
}
