// Package csi provides detection of the secrets-store CSI driver CRDs.
package csi

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
)

const (
	// CSIAPIGroup is the API group for the secrets-store CSI driver.
	CSIAPIGroup = "secrets-store.csi.x-k8s.io"
	// CSIAPIVersion is the API version used by Reloader.
	CSIAPIVersion = "v1"
	// CSIPodStatusResource is the watched resource.
	CSIPodStatusResource = "secretproviderclasspodstatuses"
)

// HasCSISupport reports whether the cluster has the secrets-store CSI driver
// CRDs installed (specifically the SecretProviderClassPodStatus resource).
func HasCSISupport(client discovery.DiscoveryInterface, log logr.Logger) bool {
	resources, err := client.ServerResourcesForGroupVersion(CSIAPIGroup + "/" + CSIAPIVersion)
	if err != nil {
		log.V(1).Info("CSI API not available", "error", err)
		return false
	}

	for _, r := range resources.APIResources {
		if r.Name == CSIPodStatusResource {
			log.Info("CSI provider detected, enabling SecretProviderClass support")
			return true
		}
	}

	log.V(1).Info("CSI resource not found in " + CSIAPIGroup + "/" + CSIAPIVersion)
	return false
}
