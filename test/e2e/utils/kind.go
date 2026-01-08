package utils

import (
	"fmt"
	"os"
	"os/exec"
)

// GetKindClusterName returns the Kind cluster name from the KIND_CLUSTER environment variable,
// or "kind" as the default.
func GetKindClusterName() string {
	if cluster := os.Getenv("KIND_CLUSTER"); cluster != "" {
		return cluster
	}
	return "kind"
}

// LoadImageToKindCluster loads a Docker image into the Kind cluster using the default cluster name.
func LoadImageToKindCluster(image string) error {
	cmd := exec.Command("kind", "load", "docker-image", image, "--name", GetKindClusterName())
	output, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to load image %s to Kind cluster: %w\nOutput: %s",
			image, err, output)
	}
	return nil
}
