package openshift

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
)

const (
	// DeploymentConfigAPIGroup is the API group for DeploymentConfig.
	DeploymentConfigAPIGroup = "apps.openshift.io"
	// DeploymentConfigAPIVersion is the API version for DeploymentConfig.
	DeploymentConfigAPIVersion = "v1"
	// DeploymentConfigResource is the resource name for DeploymentConfig.
	DeploymentConfigResource = "deploymentconfigs"
)

// HasDeploymentConfigSupport checks if the cluster supports DeploymentConfig
func HasDeploymentConfigSupport(client discovery.DiscoveryInterface, log logr.Logger) bool {
	resources, err := client.ServerResourcesForGroupVersion(DeploymentConfigAPIGroup + "/" + DeploymentConfigAPIVersion)
	if err != nil {
		log.V(1).Info("DeploymentConfig API not available", "error", err)
		return false
	}

	for _, r := range resources.APIResources {
		if r.Name == DeploymentConfigResource {
			log.Info("DeploymentConfig API detected, enabling support")
			return true
		}
	}

	log.V(1).Info("DeploymentConfig resource not found in apps.openshift.io/v1")
	return false
}
