package handler

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"

	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/common"
)

func TestGetRollingUpgradeFuncs(t *testing.T) {
	tests := []struct {
		name          string
		getFuncs      func() callbacks.RollingUpgradeFuncs
		resourceType  string
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
			mountType:  constants.ConfigmapEnvVarPostfix,
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
		name      string
		container *v1.Container
		envVar    string
		shaData   string
		expected  constants.Result
		newValue  string
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
				_, exists := annotations[getReloaderAnnotationKey()]
				assert.True(t, exists)
			}
		})
	}
}

// Helper function to create a mock deployment for testing
func createTestDeployment(containers []v1.Container, initContainers []v1.Container, volumes []v1.Volume) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers:     containers,
					InitContainers: initContainers,
					Volumes:        volumes,
				},
			},
		},
	}
}

// mockRollingUpgradeFuncs creates mock callbacks for testing getContainerUsingResource
func mockRollingUpgradeFuncs(deployment *appsv1.Deployment) callbacks.RollingUpgradeFuncs {
	return callbacks.RollingUpgradeFuncs{
		VolumesFunc: func(item runtime.Object) []v1.Volume {
			return deployment.Spec.Template.Spec.Volumes
		},
		ContainersFunc: func(item runtime.Object) []v1.Container {
			return deployment.Spec.Template.Spec.Containers
		},
		InitContainersFunc: func(item runtime.Object) []v1.Container {
			return deployment.Spec.Template.Spec.InitContainers
		},
	}
}

func TestGetContainerUsingResource(t *testing.T) {
	tests := []struct {
		name           string
		containers     []v1.Container
		initContainers []v1.Container
		volumes        []v1.Volume
		config         common.Config
		autoReload     bool
		expectNil      bool
		expectedName   string
	}{
		{
			name: "Volume mount in regular container",
			containers: []v1.Container{
				{
					Name: "app",
					VolumeMounts: []v1.VolumeMount{
						{Name: "config-volume", MountPath: "/etc/config"},
					},
				},
			},
			initContainers: []v1.Container{},
			volumes: []v1.Volume{
				{
					Name: "config-volume",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{Name: "my-configmap"},
						},
					},
				},
			},
			config: common.Config{
				ResourceName: "my-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "app",
		},
		{
			name: "Volume mount in init container returns first regular container",
			containers: []v1.Container{
				{Name: "main-app"},
				{Name: "sidecar"},
			},
			initContainers: []v1.Container{
				{
					Name: "init",
					VolumeMounts: []v1.VolumeMount{
						{Name: "secret-volume", MountPath: "/etc/secrets"},
					},
				},
			},
			volumes: []v1.Volume{
				{
					Name: "secret-volume",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{SecretName: "my-secret"},
					},
				},
			},
			config: common.Config{
				ResourceName: "my-secret",
				Type:         constants.SecretEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "main-app",
		},
		{
			name: "EnvFrom ConfigMap in regular container",
			containers: []v1.Container{
				{
					Name: "app",
					EnvFrom: []v1.EnvFromSource{
						{
							ConfigMapRef: &v1.ConfigMapEnvSource{
								LocalObjectReference: v1.LocalObjectReference{Name: "env-configmap"},
							},
						},
					},
				},
			},
			initContainers: []v1.Container{},
			volumes:        []v1.Volume{},
			config: common.Config{
				ResourceName: "env-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "app",
		},
		{
			name: "EnvFrom Secret in init container returns first regular container",
			containers: []v1.Container{
				{Name: "main-app"},
			},
			initContainers: []v1.Container{
				{
					Name: "init",
					EnvFrom: []v1.EnvFromSource{
						{
							SecretRef: &v1.SecretEnvSource{
								LocalObjectReference: v1.LocalObjectReference{Name: "init-secret"},
							},
						},
					},
				},
			},
			volumes: []v1.Volume{},
			config: common.Config{
				ResourceName: "init-secret",
				Type:         constants.SecretEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "main-app",
		},
		{
			name: "autoReload=false with no mount returns first container (explicit annotation)",
			containers: []v1.Container{
				{Name: "first-container"},
				{Name: "second-container"},
			},
			initContainers: []v1.Container{},
			volumes:        []v1.Volume{},
			config: common.Config{
				ResourceName: "external-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "first-container",
		},
		{
			name: "autoReload=true with no mount returns nil",
			containers: []v1.Container{
				{Name: "app"},
			},
			initContainers: []v1.Container{},
			volumes:        []v1.Volume{},
			config: common.Config{
				ResourceName: "unmounted-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload: true,
			expectNil:  true,
		},
		{
			name:           "Empty containers returns nil",
			containers:     []v1.Container{},
			initContainers: []v1.Container{},
			volumes:        []v1.Volume{},
			config: common.Config{
				ResourceName: "any-configmap",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload: false,
			expectNil:  true,
		},
		{
			name:       "Init container with volume but no regular containers returns nil",
			containers: []v1.Container{},
			initContainers: []v1.Container{
				{
					Name: "init",
					VolumeMounts: []v1.VolumeMount{
						{Name: "config-volume", MountPath: "/etc/config"},
					},
				},
			},
			volumes: []v1.Volume{
				{
					Name: "config-volume",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{Name: "init-only-cm"},
						},
					},
				},
			},
			config: common.Config{
				ResourceName: "init-only-cm",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload: false,
			expectNil:  true,
		},
		{
			name: "CSI SecretProviderClass volume",
			containers: []v1.Container{
				{
					Name: "app",
					VolumeMounts: []v1.VolumeMount{
						{Name: "csi-volume", MountPath: "/mnt/secrets"},
					},
				},
			},
			initContainers: []v1.Container{},
			volumes: []v1.Volume{
				{
					Name: "csi-volume",
					VolumeSource: v1.VolumeSource{
						CSI: &v1.CSIVolumeSource{
							Driver: "secrets-store.csi.k8s.io",
							VolumeAttributes: map[string]string{
								"secretProviderClass": "my-spc",
							},
						},
					},
				},
			},
			config: common.Config{
				ResourceName: "my-spc",
				Type:         constants.SecretProviderClassEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "app",
		},
		{
			name: "Env ValueFrom ConfigMapKeyRef",
			containers: []v1.Container{
				{
					Name: "app",
					Env: []v1.EnvVar{
						{
							Name: "CONFIG_VALUE",
							ValueFrom: &v1.EnvVarSource{
								ConfigMapKeyRef: &v1.ConfigMapKeySelector{
									LocalObjectReference: v1.LocalObjectReference{Name: "keyref-cm"},
									Key:                  "my-key",
								},
							},
						},
					},
				},
			},
			initContainers: []v1.Container{},
			volumes:        []v1.Volume{},
			config: common.Config{
				ResourceName: "keyref-cm",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "app",
		},
		{
			name: "Env ValueFrom SecretKeyRef",
			containers: []v1.Container{
				{
					Name: "app",
					Env: []v1.EnvVar{
						{
							Name: "SECRET_VALUE",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{Name: "keyref-secret"},
									Key:                  "password",
								},
							},
						},
					},
				},
			},
			initContainers: []v1.Container{},
			volumes:        []v1.Volume{},
			config: common.Config{
				ResourceName: "keyref-secret",
				Type:         constants.SecretEnvVarPostfix,
			},
			autoReload:   false,
			expectNil:    false,
			expectedName: "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := createTestDeployment(tt.containers, tt.initContainers, tt.volumes)
			funcs := mockRollingUpgradeFuncs(deployment)

			result := getContainerUsingResource(funcs, deployment, tt.config, tt.autoReload)

			if tt.expectNil {
				assert.Nil(t, result, "Expected nil container")
			} else {
				assert.NotNil(t, result, "Expected non-nil container")
				assert.Equal(t, tt.expectedName, result.Name)
			}
		})
	}
}

func TestRetryOnConflict(t *testing.T) {
	tests := []struct {
		name      string
		fnResults []struct {
			matched bool
			err     error
		}
		expectMatched bool
		expectError   bool
	}{
		{
			name: "Success on first try",
			fnResults: []struct {
				matched bool
				err     error
			}{
				{matched: true, err: nil},
			},
			expectMatched: true,
			expectError:   false,
		},
		{
			name: "Conflict then success",
			fnResults: []struct {
				matched bool
				err     error
			}{
				{matched: false,
					err: apierrors.NewConflict(schema.GroupResource{Group: "", Resource: "deployments"}, "test",
						errors.New("conflict"))},
				{matched: true, err: nil},
			},
			expectMatched: true,
			expectError:   false,
		},
		{
			name: "Non-conflict error returns immediately",
			fnResults: []struct {
				matched bool
				err     error
			}{
				{matched: false, err: errors.New("some other error")},
			},
			expectMatched: false,
			expectError:   true,
		},
		{
			name: "Multiple conflicts then success",
			fnResults: []struct {
				matched bool
				err     error
			}{
				{matched: false, err: apierrors.NewConflict(schema.GroupResource{}, "test", errors.New("conflict 1"))},
				{matched: false, err: apierrors.NewConflict(schema.GroupResource{}, "test", errors.New("conflict 2"))},
				{matched: true, err: nil},
			},
			expectMatched: true,
			expectError:   false,
		},
		{
			name: "Not matched but no error",
			fnResults: []struct {
				matched bool
				err     error
			}{
				{matched: false, err: nil},
			},
			expectMatched: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			fn := func(fetchResource bool) (bool, error) {
				if callCount >= len(tt.fnResults) {
					return true, nil
				}
				result := tt.fnResults[callCount]
				callCount++
				return result.matched, result.err
			}

			matched, err := retryOnConflict(retry.DefaultRetry, fn)

			assert.Equal(t, tt.expectMatched, matched)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetVolumeMountNameCSI(t *testing.T) {
	tests := []struct {
		name       string
		volumes    []v1.Volume
		mountType  string
		volumeName string
		expected   string
	}{
		{
			name: "CSI SecretProviderClass volume match",
			volumes: []v1.Volume{
				{
					Name: "csi-secrets",
					VolumeSource: v1.VolumeSource{
						CSI: &v1.CSIVolumeSource{
							Driver: "secrets-store.csi.k8s.io",
							VolumeAttributes: map[string]string{
								"secretProviderClass": "my-vault-spc",
							},
						},
					},
				},
			},
			mountType:  constants.SecretProviderClassEnvVarPostfix,
			volumeName: "my-vault-spc",
			expected:   "csi-secrets",
		},
		{
			name: "CSI volume with different SPC name - no match",
			volumes: []v1.Volume{
				{
					Name: "csi-secrets",
					VolumeSource: v1.VolumeSource{
						CSI: &v1.CSIVolumeSource{
							Driver: "secrets-store.csi.k8s.io",
							VolumeAttributes: map[string]string{
								"secretProviderClass": "other-spc",
							},
						},
					},
				},
			},
			mountType:  constants.SecretProviderClassEnvVarPostfix,
			volumeName: "my-vault-spc",
			expected:   "",
		},
		{
			name: "CSI volume without secretProviderClass attribute",
			volumes: []v1.Volume{
				{
					Name: "csi-volume",
					VolumeSource: v1.VolumeSource{
						CSI: &v1.CSIVolumeSource{
							Driver:           "other-csi-driver",
							VolumeAttributes: map[string]string{},
						},
					},
				},
			},
			mountType:  constants.SecretProviderClassEnvVarPostfix,
			volumeName: "any-spc",
			expected:   "",
		},
		{
			name: "CSI volume with nil VolumeAttributes",
			volumes: []v1.Volume{
				{
					Name: "csi-volume",
					VolumeSource: v1.VolumeSource{
						CSI: &v1.CSIVolumeSource{
							Driver: "secrets-store.csi.k8s.io",
						},
					},
				},
			},
			mountType:  constants.SecretProviderClassEnvVarPostfix,
			volumeName: "any-spc",
			expected:   "",
		},
		{
			name: "Multiple volumes with CSI match",
			volumes: []v1.Volume{
				{
					Name: "config-volume",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{Name: "my-cm"},
						},
					},
				},
				{
					Name: "csi-secrets",
					VolumeSource: v1.VolumeSource{
						CSI: &v1.CSIVolumeSource{
							Driver: "secrets-store.csi.k8s.io",
							VolumeAttributes: map[string]string{
								"secretProviderClass": "target-spc",
							},
						},
					},
				},
			},
			mountType:  constants.SecretProviderClassEnvVarPostfix,
			volumeName: "target-spc",
			expected:   "csi-secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVolumeMountName(tt.volumes, tt.mountType, tt.volumeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecretProviderClassAnnotationReloaded(t *testing.T) {
	tests := []struct {
		name           string
		oldAnnotations map[string]string
		newConfig      common.Config
		expected       bool
	}{
		{
			name: "Annotation contains matching SPC name and SHA",
			oldAnnotations: map[string]string{
				"reloader.stakater.com/last-reloaded-from": `{"name":"my-spc","sha":"abc123"}`,
			},
			newConfig: common.Config{
				ResourceName: "my-spc",
				SHAValue:     "abc123",
			},
			expected: true,
		},
		{
			name: "Annotation contains SPC name but different SHA",
			oldAnnotations: map[string]string{
				"reloader.stakater.com/last-reloaded-from": `{"name":"my-spc","sha":"old-sha"}`,
			},
			newConfig: common.Config{
				ResourceName: "my-spc",
				SHAValue:     "new-sha",
			},
			expected: false,
		},
		{
			name: "Annotation contains different SPC name",
			oldAnnotations: map[string]string{
				"reloader.stakater.com/last-reloaded-from": `{"name":"other-spc","sha":"abc123"}`,
			},
			newConfig: common.Config{
				ResourceName: "my-spc",
				SHAValue:     "abc123",
			},
			expected: false,
		},
		{
			name:           "Empty annotations",
			oldAnnotations: map[string]string{},
			newConfig: common.Config{
				ResourceName: "my-spc",
				SHAValue:     "abc123",
			},
			expected: false,
		},
		{
			name:           "Nil annotations",
			oldAnnotations: nil,
			newConfig: common.Config{
				ResourceName: "my-spc",
				SHAValue:     "abc123",
			},
			expected: false,
		},
		{
			name: "Annotation key missing",
			oldAnnotations: map[string]string{
				"other-annotation": "some-value",
			},
			newConfig: common.Config{
				ResourceName: "my-spc",
				SHAValue:     "abc123",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secretProviderClassAnnotationReloaded(tt.oldAnnotations, tt.newConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInvokeReloadStrategy(t *testing.T) {
	originalStrategy := options.ReloadStrategy
	defer func() { options.ReloadStrategy = originalStrategy }()

	deployment := createTestDeployment(
		[]v1.Container{
			{
				Name: "app",
				EnvFrom: []v1.EnvFromSource{
					{
						ConfigMapRef: &v1.ConfigMapEnvSource{
							LocalObjectReference: v1.LocalObjectReference{Name: "my-configmap"},
						},
					},
				},
			},
		},
		[]v1.Container{},
		[]v1.Volume{},
	)
	deployment.Spec.Template.Annotations = map[string]string{}

	funcs := callbacks.RollingUpgradeFuncs{
		VolumesFunc: func(item runtime.Object) []v1.Volume {
			return deployment.Spec.Template.Spec.Volumes
		},
		ContainersFunc: func(item runtime.Object) []v1.Container {
			return deployment.Spec.Template.Spec.Containers
		},
		InitContainersFunc: func(item runtime.Object) []v1.Container {
			return deployment.Spec.Template.Spec.InitContainers
		},
		PodAnnotationsFunc: func(item runtime.Object) map[string]string {
			return deployment.Spec.Template.Annotations
		},
		SupportsPatch: false,
	}

	config := common.Config{
		ResourceName: "my-configmap",
		Type:         constants.ConfigmapEnvVarPostfix,
		SHAValue:     "sha256:abc123",
		Namespace:    "default",
	}

	tests := []struct {
		name           string
		reloadStrategy string
		autoReload     bool
		expectResult   constants.Result
	}{
		{
			name:           "Annotations strategy",
			reloadStrategy: constants.AnnotationsReloadStrategy,
			autoReload:     false,
			expectResult:   constants.Updated,
		},
		{
			name:           "Env vars strategy with container found",
			reloadStrategy: constants.EnvVarsReloadStrategy,
			autoReload:     false,
			expectResult:   constants.Updated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options.ReloadStrategy = tt.reloadStrategy
			deployment.Spec.Template.Annotations = map[string]string{}

			result := invokeReloadStrategy(funcs, deployment, config, tt.autoReload)
			assert.Equal(t, tt.expectResult, result.Result)
		})
	}
}
