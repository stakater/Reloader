package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Helm-related constants.
const (
	// DefaultTestImage is the default image to test if RELOADER_IMAGE is not set.
	DefaultTestImage = "ghcr.io/stakater/reloader:test"

	// DefaultHelmReleaseName is the Helm release name for Reloader.
	DefaultHelmReleaseName = "reloader"

	// DefaultHelmChartPath is the path to the Helm chart relative to project root.
	DefaultHelmChartPath = "deployments/kubernetes/chart/reloader"

	// StakaterEnvVarPrefix is the prefix for Stakater environment variables.
	StakaterEnvVarPrefix = "STAKATER_"
)

// DeployOptions configures how Reloader is deployed.
type DeployOptions struct {
	// Namespace to deploy Reloader into.
	Namespace string

	// Image is the full image reference (e.g., "ghcr.io/stakater/reloader:test").
	Image string

	// Values are additional Helm values to set (key=value pairs).
	Values map[string]string

	// ReleaseName is the Helm release name. Defaults to DefaultHelmReleaseName.
	ReleaseName string

	// Timeout for Helm operations. Defaults to "120s".
	Timeout string
}

// DeployReloader deploys Reloader using Helm with the specified options.
func DeployReloader(opts DeployOptions) error {
	projectDir, err := GetProjectDir()
	if err != nil {
		return fmt.Errorf("getting project dir: %w", err)
	}

	if opts.ReleaseName == "" {
		opts.ReleaseName = DefaultHelmReleaseName
	}
	if opts.Timeout == "" {
		opts.Timeout = "180s"
	}
	if opts.Image == "" {
		opts.Image = GetTestImage()
	}

	cleanupClusterResources(opts.ReleaseName)

	chartPath := filepath.Join(projectDir, DefaultHelmChartPath)

	args := []string{
		"upgrade", "--install", opts.ReleaseName,
		chartPath,
		"--namespace", opts.Namespace,
		"--create-namespace",
		"--reset-values",
		"--set", fmt.Sprintf("image.repository=%s", GetImageRepository(opts.Image)),
		"--set", fmt.Sprintf("image.tag=%s", GetImageTag(opts.Image)),
		"--set", "image.pullPolicy=IfNotPresent",
		"--wait",
		"--timeout", opts.Timeout,
	}

	for key, value := range opts.Values {
		args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.Command("helm", args...)
	output, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("helm install failed: %s: %w", output, err)
	}

	return nil
}

// UndeployReloader removes the Reloader Helm release and cleans up cluster-scoped resources.
// This function waits for all resources to be fully deleted to prevent race conditions
// between test suites.
func UndeployReloader(namespace, releaseName string) error {
	if releaseName == "" {
		releaseName = DefaultHelmReleaseName
	}

	cmd := exec.Command("helm", "uninstall", releaseName, "--namespace", namespace, "--ignore-not-found", "--wait")
	output, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("helm uninstall failed: %s: %w", output, err)
	}

	clusterResources := []struct {
		kind string
		name string
	}{
		{"clusterrole", releaseName + "-reloader-role"},
		{"clusterrolebinding", releaseName + "-reloader-role-binding"},
	}

	for _, res := range clusterResources {
		cmd := exec.Command("kubectl", "delete", res.kind, res.name, "--ignore-not-found", "--wait=true")
		_, _ = Run(cmd)
	}

	waitForReloaderGone(namespace, releaseName)

	return nil
}

// waitForReloaderGone waits for the Reloader deployment to be fully removed using kubectl wait.
// This is watch-based (kubectl wait --for=delete) rather than a polling loop.
func waitForReloaderGone(namespace, releaseName string) {
	deploymentName := ReloaderDeploymentName(releaseName)
	cmd := exec.Command("kubectl", "wait",
		"deployment/"+deploymentName,
		"--for=delete",
		"--namespace", namespace,
		"--timeout=120s",
	)
	_, _ = Run(cmd)
}

// cleanupClusterResources removes cluster-scoped resources that might be left over
// from a previous test run. This is called before deploying to ensure clean state.
func cleanupClusterResources(releaseName string) {
	if releaseName == "" {
		releaseName = DefaultHelmReleaseName
	}

	clusterResources := []struct {
		kind string
		name string
	}{
		{"clusterrole", releaseName + "-reloader-role"},
		{"clusterrolebinding", releaseName + "-reloader-role-binding"},
	}

	for _, res := range clusterResources {
		cmd := exec.Command("kubectl", "delete", res.kind, res.name, "--ignore-not-found", "--wait=true")
		_, _ = Run(cmd)
	}
}

// GetTestImage returns the test image from environment or the default.
func GetTestImage() string {
	if img := os.Getenv("RELOADER_IMAGE"); img != "" {
		return img
	}
	return DefaultTestImage
}

// GetImageRepository extracts the repository (without tag or digest) from a full image reference.
// Examples:
//
//	"ghcr.io/stakater/reloader:v1.0.0"          -> "ghcr.io/stakater/reloader"
//	"ghcr.io/stakater/reloader@sha256:abc123"    -> "ghcr.io/stakater/reloader"
func GetImageRepository(image string) string {
	// Digest-based: repo@sha256:hash — split at '@'
	if idx := strings.Index(image, "@"); idx != -1 {
		return image[:idx]
	}
	// Tag-based: repo:tag — split at last ':' only if it comes after the last '/'
	if lastColon := strings.LastIndex(image, ":"); lastColon != -1 {
		if lastSlash := strings.LastIndex(image, "/"); lastSlash < lastColon {
			return image[:lastColon]
		}
	}
	return image
}

// GetImageTag extracts the tag from a full image reference.
// Examples:
//
//	"ghcr.io/stakater/reloader:v1.0.0"          -> "v1.0.0"
//	"ghcr.io/stakater/reloader@sha256:abc123"    -> "sha256:abc123"
//
// Returns "latest" if no tag or digest is found.
func GetImageTag(image string) string {
	// Digest-based: return everything after '@'
	if idx := strings.Index(image, "@"); idx != -1 {
		return image[idx+1:]
	}
	// Tag-based: return everything after last ':' (only if it comes after the last '/')
	if lastColon := strings.LastIndex(image, ":"); lastColon != -1 {
		if lastSlash := strings.LastIndex(image, "/"); lastSlash < lastColon {
			return image[lastColon+1:]
		}
	}
	return "latest"
}

// ReloaderDeploymentName returns the full deployment name for Reloader.
func ReloaderDeploymentName(releaseName string) string {
	if releaseName == "" {
		releaseName = DefaultHelmReleaseName
	}
	return releaseName + "-reloader"
}

// ReloaderPodSelector returns the label selector for Reloader pods.
func ReloaderPodSelector(releaseName string) string {
	if releaseName == "" {
		releaseName = DefaultHelmReleaseName
	}
	return "app=" + releaseName + "-reloader"
}
