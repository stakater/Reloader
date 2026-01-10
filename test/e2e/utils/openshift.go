package utils

import (
	"k8s.io/client-go/discovery"
)

// HasDeploymentConfigSupport checks if the cluster has OpenShift DeploymentConfig API available.
func HasDeploymentConfigSupport(discoveryClient discovery.DiscoveryInterface) bool {
	_, apiLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false
	}

	for _, apiList := range apiLists {
		for _, resource := range apiList.APIResources {
			if resource.Kind == "DeploymentConfig" {
				return true
			}
		}
	}

	return false
}
