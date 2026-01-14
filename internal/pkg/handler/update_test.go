package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
)

func TestResourceUpdatedHandler_GetConfig(t *testing.T) {
	tests := []struct {
		name              string
		oldResource       any
		newResource       any
		expectedName      string
		expectedNS        string
		expectedType      string
		expectSHANotEmpty bool
		expectSHAChanged  bool
	}{
		{
			name: "ConfigMap data changed",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key": "old-value"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key": "new-value"},
			},
			expectedName:      "my-cm",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  true,
		},
		{
			name: "ConfigMap data unchanged",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key": "same-value"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key": "same-value"},
			},
			expectedName:      "my-cm",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  false,
		},
		{
			name: "ConfigMap key added",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key1": "value1"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key1": "value1", "key2": "value2"},
			},
			expectedName:      "my-cm",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  true,
		},
		{
			name: "ConfigMap key removed",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key1": "value1", "key2": "value2"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
				Data:       map[string]string{"key1": "value1"},
			},
			expectedName:      "my-cm",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  true,
		},
		{
			name: "ConfigMap only labels changed - SHA unchanged",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cm",
					Namespace: "default",
					Labels:    map[string]string{"version": "v1"},
				},
				Data: map[string]string{"key": "value"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cm",
					Namespace: "default",
					Labels:    map[string]string{"version": "v2"},
				},
				Data: map[string]string{"key": "value"},
			},
			expectedName:      "my-cm",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  false,
		},
		{
			name: "ConfigMap only annotations changed - SHA unchanged",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "my-cm",
					Namespace:   "default",
					Annotations: map[string]string{"note": "old"},
				},
				Data: map[string]string{"key": "value"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "my-cm",
					Namespace:   "default",
					Annotations: map[string]string{"note": "new"},
				},
				Data: map[string]string{"key": "value"},
			},
			expectedName:      "my-cm",
			expectedNS:        "default",
			expectedType:      constants.ConfigmapEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  false,
		},
		{
			name: "Secret data changed",
			oldResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"password": []byte("old-pass")},
			},
			newResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"password": []byte("new-pass")},
			},
			expectedName:      "my-secret",
			expectedNS:        "default",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  true,
		},
		{
			name: "Secret data unchanged",
			oldResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"password": []byte("same-pass")},
			},
			newResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"password": []byte("same-pass")},
			},
			expectedName:      "my-secret",
			expectedNS:        "default",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  false,
		},
		{
			name: "Secret key added",
			oldResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"key1": []byte("value1")},
			},
			newResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			},
			expectedName:      "my-secret",
			expectedNS:        "default",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  true,
		},
		{
			name: "Secret only labels changed - SHA unchanged",
			oldResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Labels:    map[string]string{"env": "dev"},
				},
				Data: map[string][]byte{"key": []byte("value")},
			},
			newResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Labels:    map[string]string{"env": "prod"},
				},
				Data: map[string][]byte{"key": []byte("value")},
			},
			expectedName:      "my-secret",
			expectedNS:        "default",
			expectedType:      constants.SecretEnvVarPostfix,
			expectSHANotEmpty: true,
			expectSHAChanged:  false,
		},
		{
			name:              "Invalid resource type",
			oldResource:       "invalid",
			newResource:       "invalid",
			expectedName:      "",
			expectedNS:        "",
			expectedType:      "",
			expectSHANotEmpty: false,
			expectSHAChanged:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ResourceUpdatedHandler{
				Resource:    tt.newResource,
				OldResource: tt.oldResource,
				Collectors:  metrics.NewCollectors(),
			}

			config, oldSHA := handler.GetConfig()

			assert.Equal(t, tt.expectedName, config.ResourceName)
			assert.Equal(t, tt.expectedNS, config.Namespace)
			assert.Equal(t, tt.expectedType, config.Type)

			if tt.expectSHANotEmpty {
				assert.NotEmpty(t, config.SHAValue, "new SHA should not be empty")
				assert.NotEmpty(t, oldSHA, "old SHA should not be empty")
			}

			if tt.expectSHAChanged {
				assert.NotEqual(t, config.SHAValue, oldSHA, "SHA should have changed")
			} else if tt.expectSHANotEmpty {
				assert.Equal(t, config.SHAValue, oldSHA, "SHA should not have changed")
			}
		})
	}
}

func TestResourceUpdatedHandler_Handle(t *testing.T) {
	tests := []struct {
		name        string
		oldResource any
		newResource any
		expectError bool
	}{
		{
			name:        "Both resources nil",
			oldResource: nil,
			newResource: nil,
			expectError: false,
		},
		{
			name:        "Old resource nil",
			oldResource: nil,
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
				Data:       map[string]string{"key": "value"},
			},
			expectError: false,
		},
		{
			name: "New resource nil",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
				Data:       map[string]string{"key": "value"},
			},
			newResource: nil,
			expectError: false,
		},
		{
			name: "ConfigMap unchanged - no action",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
				Data:       map[string]string{"key": "same"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
				Data:       map[string]string{"key": "same"},
			},
			expectError: false,
		},
		{
			name: "ConfigMap changed - triggers update",
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
				Data:       map[string]string{"key": "old"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
				Data:       map[string]string{"key": "new"},
			},
			expectError: false,
		},
		{
			name: "Secret unchanged - no action",
			oldResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "default"},
				Data:       map[string][]byte{"key": []byte("same")},
			},
			newResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "default"},
				Data:       map[string][]byte{"key": []byte("same")},
			},
			expectError: false,
		},
		{
			name: "Secret changed - triggers update",
			oldResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "default"},
				Data:       map[string][]byte{"key": []byte("old")},
			},
			newResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "default"},
				Data:       map[string][]byte{"key": []byte("new")},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ResourceUpdatedHandler{
				Resource:    tt.newResource,
				OldResource: tt.oldResource,
				Collectors:  metrics.NewCollectors(),
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

func TestResourceUpdatedHandler_GetConfig_Annotations(t *testing.T) {
	oldCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "default",
			Annotations: map[string]string{
				"old-annotation": "old-value",
			},
		},
		Data: map[string]string{"key": "value"},
	}

	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "default",
			Annotations: map[string]string{
				"new-annotation": "new-value",
			},
		},
		Data: map[string]string{"key": "value"},
	}

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  metrics.NewCollectors(),
	}

	config, _ := handler.GetConfig()

	assert.Equal(t, "new-value", config.ResourceAnnotations["new-annotation"])
	_, hasOld := config.ResourceAnnotations["old-annotation"]
	assert.False(t, hasOld)
}

func TestResourceUpdatedHandler_GetConfig_Labels(t *testing.T) {
	oldSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: "default",
			Labels:    map[string]string{"version": "v1"},
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	newSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: "default",
			Labels:    map[string]string{"version": "v2"},
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	handler := ResourceUpdatedHandler{
		Resource:    newSecret,
		OldResource: oldSecret,
		Collectors:  metrics.NewCollectors(),
	}

	config, _ := handler.GetConfig()

	assert.Equal(t, "v2", config.Labels["version"])
}

func TestResourceUpdatedHandler_EmptyToNonEmpty(t *testing.T) {
	oldCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{},
	}
	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{"key": "value"},
	}

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  metrics.NewCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.NotEqual(t, config.SHAValue, oldSHA, "SHA should change when data is added")
}

func TestResourceUpdatedHandler_NonEmptyToEmpty(t *testing.T) {
	oldCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{"key": "value"},
	}
	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{},
	}

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  metrics.NewCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.NotEqual(t, config.SHAValue, oldSHA, "SHA should change when data is removed")
}

func TestResourceUpdatedHandler_BinaryDataChange(t *testing.T) {
	oldCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		BinaryData: map[string][]byte{"binary": []byte("old-binary")},
	}
	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		BinaryData: map[string][]byte{"binary": []byte("new-binary")},
	}

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  metrics.NewCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.NotEqual(t, config.SHAValue, oldSHA, "SHA should change when binary data changes")
}

func TestResourceUpdatedHandler_MixedDataAndBinaryData(t *testing.T) {
	oldCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{"text": "value"},
		BinaryData: map[string][]byte{"binary": []byte("binary-value")},
	}
	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
		Data:       map[string]string{"text": "value"},
		BinaryData: map[string][]byte{"binary": []byte("new-binary-value")},
	}

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  metrics.NewCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.NotEqual(t, config.SHAValue, oldSHA, "SHA should change when binary data changes")
}

func TestResourceUpdatedHandler_DifferentNamespaces(t *testing.T) {
	oldCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns1"},
		Data:       map[string]string{"key": "value"},
	}
	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns2"},
		Data:       map[string]string{"key": "value"},
	}

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  metrics.NewCollectors(),
	}

	config, _ := handler.GetConfig()

	assert.Equal(t, "ns2", config.Namespace)
}
