package handler

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// mockDeploymentForDelete creates a deployment with containers for testing delete strategies
func mockDeploymentForDelete(name, namespace string, containers []v1.Container, volumes []v1.Volume) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: v1.PodSpec{
					Containers: containers,
					Volumes:    volumes,
				},
			},
		},
	}
}

// Mock funcs for testing
func mockContainersFunc(item runtime.Object) []v1.Container {
	deployment, ok := item.(*appsv1.Deployment)
	if !ok {
		return nil
	}
	return deployment.Spec.Template.Spec.Containers
}

func mockInitContainersFunc(item runtime.Object) []v1.Container {
	deployment, ok := item.(*appsv1.Deployment)
	if !ok {
		return nil
	}
	return deployment.Spec.Template.Spec.InitContainers
}

func mockVolumesFunc(item runtime.Object) []v1.Volume {
	deployment, ok := item.(*appsv1.Deployment)
	if !ok {
		return nil
	}
	return deployment.Spec.Template.Spec.Volumes
}

func mockPodAnnotationsFunc(item runtime.Object) map[string]string {
	deployment, ok := item.(*appsv1.Deployment)
	if !ok {
		return nil
	}
	return deployment.Spec.Template.Annotations
}

func mockPatchTemplatesFunc() callbacks.PatchTemplates {
	return callbacks.PatchTemplates{
		AnnotationTemplate:   `{"spec":{"template":{"metadata":{"annotations":{"%s":"%s"}}}}}`,
		EnvVarTemplate:       `{"spec":{"template":{"spec":{"containers":[{"name":"%s","env":[{"name":"%s","value":"%s"}]}]}}}}`,
		DeleteEnvVarTemplate: `[{"op":"remove","path":"/spec/template/spec/containers/%d/env/%d"}]`,
	}
}

func TestRemoveContainerEnvVars(t *testing.T) {
	tests := []struct {
		name          string
		containers    []v1.Container
		volumes       []v1.Volume
		config        common.Config
		autoReload    bool
		expected      constants.Result
		envVarRemoved bool
	}{
		{
			name: "Remove existing env var - configmap envFrom",
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "my-configmap",
								},
							},
						},
					},
					Env: []v1.EnvVar{
						{Name: "STAKATER_MY_CONFIGMAP_CONFIGMAP", Value: "sha-value"},
					},
				},
			},
			volumes: []v1.Volume{},
			config: common.Config{
				ResourceName: "my-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload:    true,
			expected:      constants.Updated,
			envVarRemoved: true,
		},
		{
			name: "No env var to remove",
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "my-configmap",
								},
							},
						},
					},
					Env: []v1.EnvVar{}, // No env vars
				},
			},
			volumes: []v1.Volume{},
			config: common.Config{
				ResourceName: "my-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload:    true,
			expected:      constants.NotUpdated,
			envVarRemoved: false,
		},
		{
			name: "Remove existing env var - secret envFrom",
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							SecretRef: &v1.SecretEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "my-secret",
								},
							},
						},
					},
					Env: []v1.EnvVar{
						{Name: "STAKATER_MY_SECRET_SECRET", Value: "sha-value"},
					},
				},
			},
			volumes: []v1.Volume{},
			config: common.Config{
				ResourceName: "my-secret",
				Type:         constants.SecretEnvVarPostfix,
			},
			autoReload:    true,
			expected:      constants.Updated,
			envVarRemoved: true,
		},
		{
			name:       "No container found",
			containers: []v1.Container{},
			volumes:    []v1.Volume{},
			config: common.Config{
				ResourceName: "my-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload:    true,
			expected:      constants.NoContainerFound,
			envVarRemoved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := mockDeploymentForDelete("test-deploy", "default", tt.containers, tt.volumes)

			funcs := callbacks.RollingUpgradeFuncs{
				ContainersFunc:     mockContainersFunc,
				InitContainersFunc: mockInitContainersFunc,
				VolumesFunc:        mockVolumesFunc,
				PodAnnotationsFunc: mockPodAnnotationsFunc,
				PatchTemplatesFunc: mockPatchTemplatesFunc,
				SupportsPatch:      true,
			}

			result := removeContainerEnvVars(funcs, deployment, tt.config, tt.autoReload)

			assert.Equal(t, tt.expected, result.Result)

			if tt.envVarRemoved {
				// Verify env var was removed from container
				containers := deployment.Spec.Template.Spec.Containers
				for _, c := range containers {
					for _, env := range c.Env {
						envVarName := getEnvVarName(tt.config.ResourceName, tt.config.Type)
						assert.NotEqual(t, envVarName, env.Name, "Env var should have been removed")
					}
				}
			}
		})
	}
}

func TestInvokeDeleteStrategy(t *testing.T) {
	// Save original strategy and restore after test
	originalStrategy := options.ReloadStrategy
	defer func() {
		options.ReloadStrategy = originalStrategy
	}()

	tests := []struct {
		name           string
		reloadStrategy string
		containers     []v1.Container
		volumes        []v1.Volume
		config         common.Config
	}{
		{
			name:           "Annotations strategy",
			reloadStrategy: constants.AnnotationsReloadStrategy,
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "my-configmap",
								},
							},
						},
					},
				},
			},
			volumes: []v1.Volume{},
			config: common.Config{
				ResourceName: "my-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
				SHAValue:     "sha-value",
			},
		},
		{
			name:           "EnvVars strategy",
			reloadStrategy: constants.EnvVarsReloadStrategy,
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "my-configmap",
								},
							},
						},
					},
					Env: []v1.EnvVar{
						{Name: "STAKATER_MY_CONFIGMAP_CONFIGMAP", Value: "sha-value"},
					},
				},
			},
			volumes: []v1.Volume{},
			config: common.Config{
				ResourceName: "my-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options.ReloadStrategy = tt.reloadStrategy

			deployment := mockDeploymentForDelete("test-deploy", "default", tt.containers, tt.volumes)

			funcs := callbacks.RollingUpgradeFuncs{
				ContainersFunc:     mockContainersFunc,
				InitContainersFunc: mockInitContainersFunc,
				VolumesFunc:        mockVolumesFunc,
				PodAnnotationsFunc: mockPodAnnotationsFunc,
				PatchTemplatesFunc: mockPatchTemplatesFunc,
				SupportsPatch:      true,
			}

			result := invokeDeleteStrategy(funcs, deployment, tt.config, true)

			// Should return a valid result
			assert.NotNil(t, result)
		})
	}
}

func TestRemovePodAnnotations(t *testing.T) {
	tests := []struct {
		name       string
		containers []v1.Container
		volumes    []v1.Volume
		config     common.Config
	}{
		{
			name: "Remove pod annotations - configmap",
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "my-configmap",
								},
							},
						},
					},
				},
			},
			volumes: []v1.Volume{},
			config: common.Config{
				ResourceName: "my-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
				SHAValue:     "sha-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := mockDeploymentForDelete("test-deploy", "default", tt.containers, tt.volumes)

			funcs := callbacks.RollingUpgradeFuncs{
				ContainersFunc:     mockContainersFunc,
				InitContainersFunc: mockInitContainersFunc,
				VolumesFunc:        mockVolumesFunc,
				PodAnnotationsFunc: mockPodAnnotationsFunc,
				PatchTemplatesFunc: mockPatchTemplatesFunc,
				SupportsPatch:      false, // No patch for annotations removal test
			}

			result := removePodAnnotations(funcs, deployment, tt.config, true)

			// Should return Updated since it sets the SHA to empty data hash
			assert.Equal(t, constants.Updated, result.Result)
		})
	}
}
