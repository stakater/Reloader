package utils

import (
	"testing"
)

func TestGetImageRepository(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "full image with tag",
			image:    "ghcr.io/stakater/reloader:v1.0.0",
			expected: "ghcr.io/stakater/reloader",
		},
		{
			name:     "image with latest tag",
			image:    "nginx:latest",
			expected: "nginx",
		},
		{
			name:     "image without tag",
			image:    "ghcr.io/stakater/reloader",
			expected: "ghcr.io/stakater/reloader",
		},
		{
			name:     "image with digest (not fully supported)",
			image:    "nginx@sha256:abc123",
			expected: "nginx@sha256", // Note: digest handling is limited
		},
		{
			name:     "simple image name",
			image:    "nginx",
			expected: "nginx",
		},
		{
			name:     "image with port in registry",
			image:    "localhost:5000/myimage:v1",
			expected: "localhost:5000/myimage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetImageRepository(tt.image)
			if result != tt.expected {
				t.Errorf("GetImageRepository(%q) = %q, want %q", tt.image, result, tt.expected)
			}
		})
	}
}

func TestGetImageTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "full image with tag",
			image:    "ghcr.io/stakater/reloader:v1.0.0",
			expected: "v1.0.0",
		},
		{
			name:     "image with latest tag",
			image:    "nginx:latest",
			expected: "latest",
		},
		{
			name:     "image without tag",
			image:    "ghcr.io/stakater/reloader",
			expected: "latest",
		},
		{
			name:     "simple image name",
			image:    "nginx",
			expected: "latest",
		},
		{
			name:     "image with port in registry",
			image:    "localhost:5000/myimage:v1",
			expected: "v1",
		},
		{
			name:     "tag with sha",
			image:    "myimage:sha-abc123",
			expected: "sha-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetImageTag(tt.image)
			if result != tt.expected {
				t.Errorf("GetImageTag(%q) = %q, want %q", tt.image, result, tt.expected)
			}
		})
	}
}

func TestReloaderDeploymentName(t *testing.T) {
	tests := []struct {
		name        string
		releaseName string
		expected    string
	}{
		{
			name:        "default release name",
			releaseName: "",
			expected:    "reloader-reloader",
		},
		{
			name:        "custom release name",
			releaseName: "my-reloader",
			expected:    "my-reloader-reloader",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReloaderDeploymentName(tt.releaseName)
			if result != tt.expected {
				t.Errorf("ReloaderDeploymentName(%q) = %q, want %q", tt.releaseName, result, tt.expected)
			}
		})
	}
}

func TestReloaderPodSelector(t *testing.T) {
	tests := []struct {
		name        string
		releaseName string
		expected    string
	}{
		{
			name:        "default release name",
			releaseName: "",
			expected:    "app=reloader-reloader",
		},
		{
			name:        "custom release name",
			releaseName: "my-reloader",
			expected:    "app=my-reloader-reloader",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReloaderPodSelector(tt.releaseName)
			if result != tt.expected {
				t.Errorf("ReloaderPodSelector(%q) = %q, want %q", tt.releaseName, result, tt.expected)
			}
		})
	}
}
