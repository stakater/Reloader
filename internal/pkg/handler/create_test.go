package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
)

func TestResourceCreatedHandler_GetConfig(t *testing.T) {
	tests := []struct {
		name              string
		resource          interface{}
		expectedName      string
		expectedNS        string
		expectedType      string
		expectSHANotEmpty bool
		expectOldSHAEmpty bool
	}{
		{
			name: "ConfigMap with data",
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-configmap",
					Namespace: "test-ns",
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedName:      "my-configmap",
			expectedNS:        "test-ns",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name: "ConfigMap with empty data",
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-configmap",
					Namespace: "default",
				},
				Data: map[string]string{},
			},
			expectedName:      "empty-configmap",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name: "ConfigMap with binary data",
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binary-configmap",
					Namespace: "default",
				},
				BinaryData: map[string][]byte{
					"binary-key": []byte("binary-value"),
				},
			},
			expectedName:      "binary-configmap",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name: "ConfigMap with annotations",
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "annotated-configmap",
					Namespace: "default",
					Annotations: map[string]string{
						"reloader.stakater.com/match": "true",
					},
				},
				Data: map[string]string{"key": "value"},
			},
			expectedName:      "annotated-configmap",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name: "Secret with data",
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "secret-ns",
				},
				Data: map[string][]byte{
					"password": []byte("secret-password"),
				},
			},
			expectedName:      "my-secret",
			expectedNS:        "secret-ns",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name: "Secret with empty data",
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{},
			},
			expectedName:      "empty-secret",
			expectedNS:        "default",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name: "Secret with StringData",
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stringdata-secret",
					Namespace: "default",
				},
				StringData: map[string]string{
					"username": "admin",
				},
			},
			expectedName:      "stringdata-secret",
			expectedNS:        "default",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name: "Secret with labels",
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret",
					Namespace: "default",
					Labels: map[string]string{
						"app": "test",
					},
				},
				Data: map[string][]byte{"key": []byte("value")},
			},
			expectedName:      "labeled-secret",
			expectedNS:        "default",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectOldSHAEmpty: true,
		},
		{
			name:              "Invalid resource type - string",
			resource:          "invalid-string",
			expectedName:      "",
			expectedNS:        "",
			expectedType:      "",
			expectSHANotEmpty: false,
			expectOldSHAEmpty: true,
		},
		{
			name:              "Invalid resource type - int",
			resource:          123,
			expectedName:      "",
			expectedNS:        "",
			expectedType:      "",
			expectSHANotEmpty: false,
			expectOldSHAEmpty: true,
		},
		{
			name:              "Invalid resource type - struct",
			resource:          struct{ Name string }{Name: "test"},
			expectedName:      "",
			expectedNS:        "",
			expectedType:      "",
			expectSHANotEmpty: false,
			expectOldSHAEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ResourceCreatedHandler{
				Resource:   tt.resource,
				Collectors: metrics.NewCollectors(),
			}

			config, oldSHA := handler.GetConfig()

			assert.Equal(t, tt.expectedName, config.ResourceName)
			assert.Equal(t, tt.expectedNS, config.Namespace)
			assert.Equal(t, tt.expectedType, config.Type)

			if tt.expectSHANotEmpty {
				assert.NotEmpty(t, config.SHAValue, "SHA should not be empty")
			}

			if tt.expectOldSHAEmpty {
				assert.Empty(t, oldSHA, "oldSHA should always be empty for create handler")
			}
		})
	}
}

func TestResourceCreatedHandler_GetConfig_Annotations(t *testing.T) {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "annotated-cm",
			Namespace: "default",
			Annotations: map[string]string{
				"reloader.stakater.com/match":  "true",
				"reloader.stakater.com/search": "true",
			},
		},
		Data: map[string]string{"key": "value"},
	}

	handler := ResourceCreatedHandler{
		Resource:   cm,
		Collectors: metrics.NewCollectors(),
	}

	config, _ := handler.GetConfig()

	assert.NotNil(t, config.ResourceAnnotations)
	assert.Equal(t, "true", config.ResourceAnnotations["reloader.stakater.com/match"])
	assert.Equal(t, "true", config.ResourceAnnotations["reloader.stakater.com/search"])
}

func TestResourceCreatedHandler_GetConfig_Labels(t *testing.T) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "labeled-secret",
			Namespace: "default",
			Labels: map[string]string{
				"app":     "myapp",
				"version": "v1",
			},
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	handler := ResourceCreatedHandler{
		Resource:   secret,
		Collectors: metrics.NewCollectors(),
	}

	config, _ := handler.GetConfig()

	assert.NotNil(t, config.Labels)
	assert.Equal(t, "myapp", config.Labels["app"])
	assert.Equal(t, "v1", config.Labels["version"])
}

func TestResourceCreatedHandler_Handle(t *testing.T) {
	tests := []struct {
		name        string
		resource    interface{}
		expectError bool
	}{
		{
			name:        "Nil resource",
			resource:    nil,
			expectError: false,
		},
		{
			name: "Valid ConfigMap - no workloads to update",
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value"},
			},
			expectError: false,
		},
		{
			name: "Valid Secret - no workloads to update",
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{"key": []byte("value")},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ResourceCreatedHandler{
				Resource:   tt.resource,
				Collectors: metrics.NewCollectors(),
			}

			err := handler.Handle()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResourceCreatedHandler_SHAConsistency(t *testing.T) {
	data := map[string]string{"key": "value"}

	cm1 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "default"},
		Data:       data,
	}
	cm2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: "default"},
		Data:       data,
	}

	handler1 := ResourceCreatedHandler{Resource: cm1, Collectors: metrics.NewCollectors()}
	handler2 := ResourceCreatedHandler{Resource: cm2, Collectors: metrics.NewCollectors()}

	config1, _ := handler1.GetConfig()
	config2, _ := handler2.GetConfig()

	assert.Equal(t, config1.SHAValue, config2.SHAValue)
}

func TestResourceCreatedHandler_SHADifference(t *testing.T) {
	cm1 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{"key": "value1"},
	}
	cm2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{"key": "value2"},
	}

	handler1 := ResourceCreatedHandler{Resource: cm1, Collectors: metrics.NewCollectors()}
	handler2 := ResourceCreatedHandler{Resource: cm2, Collectors: metrics.NewCollectors()}

	config1, _ := handler1.GetConfig()
	config2, _ := handler2.GetConfig()

	assert.NotEqual(t, config1.SHAValue, config2.SHAValue)
}
