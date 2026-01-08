package handler

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestGetRollingUpgradeFuncs(t *testing.T) {
	tests := []struct {
		name         string
		getFuncs     func() callbacks.RollingUpgradeFuncs
		resourceType string
		supportsPatch bool
	}{
		{
			name:          "Deployment",
			getFuncs:      GetDeploymentRollingUpgradeFuncs,
			resourceType:  "Deployment",
			supportsPatch: true,
		},
		{
			name:          "CronJob",
			getFuncs:      GetCronJobCreateJobFuncs,
			resourceType:  "CronJob",
			supportsPatch: false,
		},
		{
			name:          "Job",
			getFuncs:      GetJobCreateJobFuncs,
			resourceType:  "Job",
			supportsPatch: false,
		},
		{
			name:          "DaemonSet",
			getFuncs:      GetDaemonSetRollingUpgradeFuncs,
			resourceType:  "DaemonSet",
			supportsPatch: true,
		},
		{
			name:          "StatefulSet",
			getFuncs:      GetStatefulSetRollingUpgradeFuncs,
			resourceType:  "StatefulSet",
			supportsPatch: true,
		},
		{
			name:          "ArgoRollout",
			getFuncs:      GetArgoRolloutRollingUpgradeFuncs,
			resourceType:  "Rollout",
			supportsPatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := tt.getFuncs()
			assert.Equal(t, tt.resourceType, funcs.ResourceType)
			assert.Equal(t, tt.supportsPatch, funcs.SupportsPatch)
			assert.NotNil(t, funcs.ItemFunc)
			assert.NotNil(t, funcs.ItemsFunc)
			assert.NotNil(t, funcs.AnnotationsFunc)
			assert.NotNil(t, funcs.PodAnnotationsFunc)
			assert.NotNil(t, funcs.ContainersFunc)
			assert.NotNil(t, funcs.InitContainersFunc)
			assert.NotNil(t, funcs.UpdateFunc)
			assert.NotNil(t, funcs.PatchFunc)
			assert.NotNil(t, funcs.PatchTemplatesFunc)
			assert.NotNil(t, funcs.VolumesFunc)
		})
	}
}

func TestGetVolumeMountName(t *testing.T) {
	tests := []struct {
		name       string
		volumes    []v1.Volume
		mountType  string
		volumeName string
		expected   string
	}{
		{
			name: "ConfigMap volume match",
			volumes: []v1.Volume{
				{
					Name: "config-volume",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "my-configmap",
							},
						},
					},
				},
			},
			mountType:  constants.ConfigmapEnvVarPostfix,
			volumeName: "my-configmap",
			expected:   "config-volume",
		},
		{
			name: "Secret volume match",
			volumes: []v1.Volume{
				{
					Name: "secret-volume",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "my-secret",
						},
					},
				},
			},
			mountType:  constants.SecretEnvVarPostfix,
			volumeName: "my-secret",
			expected:   "secret-volume",
		},
		{
			name: "ConfigMap in projected volume",
			volumes: []v1.Volume{
				{
					Name: "projected-volume",
					VolumeSource: v1.VolumeSource{
						Projected: &v1.ProjectedVolumeSource{
							Sources: []v1.VolumeProjection{
								{
									ConfigMap: &v1.ConfigMapProjection{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "projected-configmap",
										},
									},
								},
							},
						},
					},
				},
			},
			mountType:  constants.ConfigmapEnvVarPostfix,
			volumeName: "projected-configmap",
			expected:   "projected-volume",
		},
		{
			name: "Secret in projected volume",
			volumes: []v1.Volume{
				{
					Name: "projected-volume",
					VolumeSource: v1.VolumeSource{
						Projected: &v1.ProjectedVolumeSource{
							Sources: []v1.VolumeProjection{
								{
									Secret: &v1.SecretProjection{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "projected-secret",
										},
									},
								},
							},
						},
					},
				},
			},
			mountType:  constants.SecretEnvVarPostfix,
			volumeName: "projected-secret",
			expected:   "projected-volume",
		},
		{
			name: "No match - wrong configmap name",
			volumes: []v1.Volume{
				{
					Name: "config-volume",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "other-configmap",
							},
						},
					},
				},
			},
			mountType:  constants.ConfigmapEnvVarPostfix,
			volumeName: "my-configmap",
			expected:   "",
		},
		{
			name: "No match - wrong type",
			volumes: []v1.Volume{
				{
					Name: "secret-volume",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "my-secret",
						},
					},
				},
			},
			mountType:  constants.ConfigmapEnvVarPostfix, // Looking for configmap but volume is secret
			volumeName: "my-secret",
			expected:   "",
		},
		{
			name:       "Empty volumes",
			volumes:    []v1.Volume{},
			mountType:  constants.ConfigmapEnvVarPostfix,
			volumeName: "any",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVolumeMountName(tt.volumes, tt.mountType, tt.volumeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetContainerWithVolumeMount(t *testing.T) {
	tests := []struct {
		name            string
		containers      []v1.Container
		volumeMountName string
		expectFound     bool
		expectedName    string
	}{
		{
			name: "Container with matching volume mount",
			containers: []v1.Container{
				{
					Name: "app",
					VolumeMounts: []v1.VolumeMount{
						{Name: "config-volume", MountPath: "/etc/config"},
					},
				},
			},
			volumeMountName: "config-volume",
			expectFound:     true,
			expectedName:    "app",
		},
		{
			name: "Multiple containers, second has mount",
			containers: []v1.Container{
				{
					Name:         "init",
					VolumeMounts: []v1.VolumeMount{},
				},
				{
					Name: "app",
					VolumeMounts: []v1.VolumeMount{
						{Name: "config-volume", MountPath: "/etc/config"},
					},
				},
			},
			volumeMountName: "config-volume",
			expectFound:     true,
			expectedName:    "app",
		},
		{
			name: "No matching volume mount",
			containers: []v1.Container{
				{
					Name: "app",
					VolumeMounts: []v1.VolumeMount{
						{Name: "other-volume", MountPath: "/etc/other"},
					},
				},
			},
			volumeMountName: "config-volume",
			expectFound:     false,
		},
		{
			name:            "Empty containers",
			containers:      []v1.Container{},
			volumeMountName: "config-volume",
			expectFound:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getContainerWithVolumeMount(tt.containers, tt.volumeMountName)
			if tt.expectFound {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedName, result.Name)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestGetContainerWithEnvReference(t *testing.T) {
	tests := []struct {
		name         string
		containers   []v1.Container
		resourceName string
		resourceType string
		expectFound  bool
		expectedName string
	}{
		{
			name: "Container with ConfigMapKeyRef",
			containers: []v1.Container{
				{
					Name: "app",
					Env: []v1.EnvVar{
						{
							Name: "CONFIG_VALUE",
							ValueFrom: &v1.EnvVarSource{
								ConfigMapKeyRef: &v1.ConfigMapKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "my-configmap",
									},
									Key: "key",
								},
							},
						},
					},
				},
			},
			resourceName: "my-configmap",
			resourceType: constants.ConfigmapEnvVarPostfix,
			expectFound:  true,
			expectedName: "app",
		},
		{
			name: "Container with SecretKeyRef",
			containers: []v1.Container{
				{
					Name: "app",
					Env: []v1.EnvVar{
						{
							Name: "SECRET_VALUE",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "my-secret",
									},
									Key: "key",
								},
							},
						},
					},
				},
			},
			resourceName: "my-secret",
			resourceType: constants.SecretEnvVarPostfix,
			expectFound:  true,
			expectedName: "app",
		},
		{
			name: "Container with ConfigMapRef (envFrom)",
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
			resourceName: "my-configmap",
			resourceType: constants.ConfigmapEnvVarPostfix,
			expectFound:  true,
			expectedName: "app",
		},
		{
			name: "Container with SecretRef (envFrom)",
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
				},
			},
			resourceName: "my-secret",
			resourceType: constants.SecretEnvVarPostfix,
			expectFound:  true,
			expectedName: "app",
		},
		{
			name: "No match - wrong resource name",
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "other-configmap",
								},
							},
						},
					},
				},
			},
			resourceName: "my-configmap",
			resourceType: constants.ConfigmapEnvVarPostfix,
			expectFound:  false,
		},
		{
			name: "No match - wrong type (looking for secret but has configmap)",
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "my-resource",
								},
							},
						},
					},
				},
			},
			resourceName: "my-resource",
			resourceType: constants.SecretEnvVarPostfix,
			expectFound:  false,
		},
		{
			name:         "Empty containers",
			containers:   []v1.Container{},
			resourceName: "any",
			resourceType: constants.ConfigmapEnvVarPostfix,
			expectFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getContainerWithEnvReference(tt.containers, tt.resourceName, tt.resourceType)
			if tt.expectFound {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedName, result.Name)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestGetEnvVarName(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		typeName     string
		expected     string
	}{
		{
			name:         "ConfigMap with simple name",
			resourceName: "my-config",
			typeName:     constants.ConfigmapEnvVarPostfix,
			expected:     "STAKATER_MY_CONFIG_CONFIGMAP",
		},
		{
			name:         "Secret with simple name",
			resourceName: "my-secret",
			typeName:     constants.SecretEnvVarPostfix,
			expected:     "STAKATER_MY_SECRET_SECRET",
		},
		{
			name:         "Name with hyphens",
			resourceName: "my-app-config",
			typeName:     constants.ConfigmapEnvVarPostfix,
			expected:     "STAKATER_MY_APP_CONFIG_CONFIGMAP",
		},
		{
			name:         "Name with dots",
			resourceName: "my.app.config",
			typeName:     constants.ConfigmapEnvVarPostfix,
			expected:     "STAKATER_MY_APP_CONFIG_CONFIGMAP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEnvVarName(tt.resourceName, tt.typeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateEnvVar(t *testing.T) {
	tests := []struct {
		name       string
		container  *v1.Container
		envVar     string
		shaData    string
		expected   constants.Result
		newValue   string // expected value after update
	}{
		{
			name: "Update existing env var with different value",
			container: &v1.Container{
				Name: "app",
				Env: []v1.EnvVar{
					{Name: "STAKATER_CONFIG_CONFIGMAP", Value: "old-sha"},
				},
			},
			envVar:   "STAKATER_CONFIG_CONFIGMAP",
			shaData:  "new-sha",
			expected: constants.Updated,
			newValue: "new-sha",
		},
		{
			name: "No update when value is same",
			container: &v1.Container{
				Name: "app",
				Env: []v1.EnvVar{
					{Name: "STAKATER_CONFIG_CONFIGMAP", Value: "same-sha"},
				},
			},
			envVar:   "STAKATER_CONFIG_CONFIGMAP",
			shaData:  "same-sha",
			expected: constants.NotUpdated,
			newValue: "same-sha",
		},
		{
			name: "Env var not found",
			container: &v1.Container{
				Name: "app",
				Env: []v1.EnvVar{
					{Name: "OTHER_VAR", Value: "value"},
				},
			},
			envVar:   "STAKATER_CONFIG_CONFIGMAP",
			shaData:  "new-sha",
			expected: constants.NoEnvVarFound,
		},
		{
			name: "Empty env list",
			container: &v1.Container{
				Name: "app",
				Env:  []v1.EnvVar{},
			},
			envVar:   "STAKATER_CONFIG_CONFIGMAP",
			shaData:  "new-sha",
			expected: constants.NoEnvVarFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updateEnvVar(tt.container, tt.envVar, tt.shaData)
			assert.Equal(t, tt.expected, result)

			if tt.expected == constants.Updated || tt.expected == constants.NotUpdated {
				// Verify the value in the container
				for _, env := range tt.container.Env {
					if env.Name == tt.envVar {
						assert.Equal(t, tt.newValue, env.Value)
						break
					}
				}
			}
		})
	}
}

func TestGetReloaderAnnotationKey(t *testing.T) {
	result := getReloaderAnnotationKey()
	expected := "reloader.stakater.com/last-reloaded-from"
	assert.Equal(t, expected, result)
}

func TestJsonEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "Simple string",
			input:    "hello",
			expected: "hello",
			hasError: false,
		},
		{
			name:     "String with quotes",
			input:    `say "hello"`,
			expected: `say \"hello\"`,
			hasError: false,
		},
		{
			name:     "String with backslash",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
			hasError: false,
		},
		{
			name:     "String with newline",
			input:    "line1\nline2",
			expected: `line1\nline2`,
			hasError: false,
		},
		{
			name:     "JSON-like string",
			input:    `{"key":"value"}`,
			expected: `{\"key\":\"value\"}`,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := jsonEscape(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCreateReloadedAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		target   *common.ReloadSource
		hasError bool
	}{
		{
			name:     "Nil target",
			target:   nil,
			hasError: true,
		},
		{
			name: "Valid target",
			target: &common.ReloadSource{
				Name: "my-configmap",
				Type: "CONFIGMAP",
			},
			hasError: false,
		},
	}

	// Use a simple func that doesn't require patch templates
	funcs := callbacks.RollingUpgradeFuncs{
		SupportsPatch: false,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations, _, err := createReloadedAnnotations(tt.target, funcs)
			if tt.hasError {
				assert.Error(t, err)
				assert.Nil(t, annotations)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, annotations)
				// Verify annotation key exists
				_, exists := annotations[getReloaderAnnotationKey()]
				assert.True(t, exists)
			}
		})
	}
}
