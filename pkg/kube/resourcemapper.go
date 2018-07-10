package kube

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DefaultResource = "default"
)

// MapToRuntimeObject maps the resource type string to the actual resource
func MapToRuntimeObject(resourceType string) runtime.Object {
	rType, ok := ResourceMap[resourceType]
	if !ok {
		return ResourceMap[DefaultResource]
	}
	return rType
}

// ResourceMap are resources from where changes are going to be detected
var ResourceMap = map[string]runtime.Object{
	"configMaps": &v1.ConfigMap{},
	"secrets": &v1.Secret{},
}
