package kube

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ResourceMap are resources from where changes are going to be detected
var ResourceMap = map[string]runtime.Object{
	"configMaps": &v1.ConfigMap{},
	"secrets":    &v1.Secret{},
	"namespaces":    &v1.Namespace{},
}
