// Package utils provides helper functions for e2e tests.
package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,staticcheck
)

// Run executes the provided command and returns its combined stdout/stderr output.
// The command is executed from the project directory.
func Run(cmd *exec.Cmd) (string, error) {
	dir, err := GetProjectDir()
	if err != nil {
		return "", fmt.Errorf("failed to get project dir: %w", err)
	}
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := stdout.String() + stderr.String()
	if err != nil {
		return output, fmt.Errorf("%q failed with error %q: %w", command, output, err)
	}

	return output, nil
}

// GetProjectDir returns the root directory of the project.
// It works by finding the directory containing go.mod.
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Walk up the directory tree looking for go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			break
		}
		dir = parent
	}

	// Fallback: try to strip common test paths
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	wd = strings.ReplaceAll(wd, "/test/e2e/annotations", "")
	wd = strings.ReplaceAll(wd, "/test/e2e/envvars", "")
	wd = strings.ReplaceAll(wd, "/test/e2e/flags", "")
	wd = strings.ReplaceAll(wd, "/test/e2e/advanced", "")
	wd = strings.ReplaceAll(wd, "/test/e2e/argo", "")
	wd = strings.ReplaceAll(wd, "/test/e2e/openshift", "")

	return wd, nil
}

// GetKubeconfig returns the path to the kubeconfig file.
// It checks KUBECONFIG environment variable first, then falls back to ~/.kube/config.
func GetKubeconfig() string {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}
