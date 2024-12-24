package kube

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// ResourceMap are resources from where changes are going to be detected
var ResourceMap = map[string]runtime.Object{
	"configMaps":                     &v1.ConfigMap{},
	"secrets":                        &v1.Secret{},
	"namespaces":                     &v1.Namespace{},
	"secretproviderclasspodstatuses": &csiv1.SecretProviderClassPodStatus{},
}
