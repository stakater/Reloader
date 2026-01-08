package handler

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to create a test ConfigMap
func createTestConfigMap(name, namespace string, data map[string]string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

// Helper function to create a test Secret
func createTestSecret(name, namespace string, data map[string][]byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

// Helper function to create test metrics collectors
func createTestCollectors() metrics.Collectors {
	return metrics.NewCollectors()
}

// ============================================================
// ResourceCreatedHandler Tests
// ============================================================

func TestResourceCreatedHandler_GetConfig_ConfigMap(t *testing.T) {
	cm := createTestConfigMap("test-cm", "default", map[string]string{"key": "value"})
	handler := ResourceCreatedHandler{
		Resource:   cm,
		Collectors: createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-cm", config.ResourceName)
	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, constants.ConfigmapEnvVarPostfix, config.Type)
	assert.NotEmpty(t, config.SHAValue)
	assert.Empty(t, oldSHA) // oldSHA is always empty for create handler
}

func TestResourceCreatedHandler_GetConfig_Secret(t *testing.T) {
	secret := createTestSecret("test-secret", "default", map[string][]byte{"key": []byte("value")})
	handler := ResourceCreatedHandler{
		Resource:   secret,
		Collectors: createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-secret", config.ResourceName)
	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, constants.SecretEnvVarPostfix, config.Type)
	assert.NotEmpty(t, config.SHAValue)
	assert.Empty(t, oldSHA)
}

func TestResourceCreatedHandler_GetConfig_InvalidResource(t *testing.T) {
	// Test with an invalid resource type
	handler := ResourceCreatedHandler{
		Resource:   "invalid",
		Collectors: createTestCollectors(),
	}

	config, _ := handler.GetConfig()

	// Config should be empty/zero for invalid resources
	assert.Empty(t, config.ResourceName)
}

func TestResourceCreatedHandler_Handle_NilResource(t *testing.T) {
	handler := ResourceCreatedHandler{
		Resource:   nil,
		Collectors: createTestCollectors(),
	}

	err := handler.Handle()

	// Should not return error even with nil resource (just logs error)
	assert.NoError(t, err)
}

// ============================================================
// ResourceDeleteHandler Tests
// ============================================================

func TestResourceDeleteHandler_GetConfig_ConfigMap(t *testing.T) {
	cm := createTestConfigMap("test-cm", "default", map[string]string{"key": "value"})
	handler := ResourceDeleteHandler{
		Resource:   cm,
		Collectors: createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-cm", config.ResourceName)
	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, constants.ConfigmapEnvVarPostfix, config.Type)
	assert.NotEmpty(t, config.SHAValue)
	assert.Empty(t, oldSHA)
}

func TestResourceDeleteHandler_GetConfig_Secret(t *testing.T) {
	secret := createTestSecret("test-secret", "default", map[string][]byte{"key": []byte("value")})
	handler := ResourceDeleteHandler{
		Resource:   secret,
		Collectors: createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-secret", config.ResourceName)
	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, constants.SecretEnvVarPostfix, config.Type)
	assert.NotEmpty(t, config.SHAValue)
	assert.Empty(t, oldSHA)
}

func TestResourceDeleteHandler_GetConfig_InvalidResource(t *testing.T) {
	handler := ResourceDeleteHandler{
		Resource:   "invalid",
		Collectors: createTestCollectors(),
	}

	config, _ := handler.GetConfig()

	assert.Empty(t, config.ResourceName)
}

func TestResourceDeleteHandler_Handle_NilResource(t *testing.T) {
	handler := ResourceDeleteHandler{
		Resource:   nil,
		Collectors: createTestCollectors(),
	}

	err := handler.Handle()

	assert.NoError(t, err)
}

// ============================================================
// ResourceUpdatedHandler Tests
// ============================================================

func TestResourceUpdatedHandler_GetConfig_ConfigMap(t *testing.T) {
	oldCM := createTestConfigMap("test-cm", "default", map[string]string{"key": "old-value"})
	newCM := createTestConfigMap("test-cm", "default", map[string]string{"key": "new-value"})

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-cm", config.ResourceName)
	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, constants.ConfigmapEnvVarPostfix, config.Type)
	assert.NotEmpty(t, config.SHAValue)
	assert.NotEmpty(t, oldSHA)
	// SHAs should be different since data changed
	assert.NotEqual(t, config.SHAValue, oldSHA)
}

func TestResourceUpdatedHandler_GetConfig_ConfigMap_SameData(t *testing.T) {
	oldCM := createTestConfigMap("test-cm", "default", map[string]string{"key": "same-value"})
	newCM := createTestConfigMap("test-cm", "default", map[string]string{"key": "same-value"})

	handler := ResourceUpdatedHandler{
		Resource:    newCM,
		OldResource: oldCM,
		Collectors:  createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-cm", config.ResourceName)
	// SHAs should be the same since data didn't change
	assert.Equal(t, config.SHAValue, oldSHA)
}

func TestResourceUpdatedHandler_GetConfig_Secret(t *testing.T) {
	oldSecret := createTestSecret("test-secret", "default", map[string][]byte{"key": []byte("old-value")})
	newSecret := createTestSecret("test-secret", "default", map[string][]byte{"key": []byte("new-value")})

	handler := ResourceUpdatedHandler{
		Resource:    newSecret,
		OldResource: oldSecret,
		Collectors:  createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-secret", config.ResourceName)
	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, constants.SecretEnvVarPostfix, config.Type)
	assert.NotEmpty(t, config.SHAValue)
	assert.NotEmpty(t, oldSHA)
	assert.NotEqual(t, config.SHAValue, oldSHA)
}

func TestResourceUpdatedHandler_GetConfig_Secret_SameData(t *testing.T) {
	oldSecret := createTestSecret("test-secret", "default", map[string][]byte{"key": []byte("same-value")})
	newSecret := createTestSecret("test-secret", "default", map[string][]byte{"key": []byte("same-value")})

	handler := ResourceUpdatedHandler{
		Resource:    newSecret,
		OldResource: oldSecret,
		Collectors:  createTestCollectors(),
	}

	config, oldSHA := handler.GetConfig()

	assert.Equal(t, "test-secret", config.ResourceName)
	// SHAs should be the same since data didn't change
	assert.Equal(t, config.SHAValue, oldSHA)
}

func TestResourceUpdatedHandler_GetConfig_InvalidResource(t *testing.T) {
	handler := ResourceUpdatedHandler{
		Resource:    "invalid",
		OldResource: "invalid",
		Collectors:  createTestCollectors(),
	}

	config, _ := handler.GetConfig()

	assert.Empty(t, config.ResourceName)
}

func TestResourceUpdatedHandler_Handle_NilResource(t *testing.T) {
	handler := ResourceUpdatedHandler{
		Resource:    nil,
		OldResource: nil,
		Collectors:  createTestCollectors(),
	}

	err := handler.Handle()

	assert.NoError(t, err)
}

func TestResourceUpdatedHandler_Handle_NilOldResource(t *testing.T) {
	cm := createTestConfigMap("test-cm", "default", map[string]string{"key": "value"})
	handler := ResourceUpdatedHandler{
		Resource:    cm,
		OldResource: nil,
		Collectors:  createTestCollectors(),
	}

	err := handler.Handle()

	// Should not return error (just logs error)
	assert.NoError(t, err)
}

func TestResourceUpdatedHandler_Handle_NoChange(t *testing.T) {
	// When SHA values are the same, Handle should return nil without doing anything
	cm := createTestConfigMap("test-cm", "default", map[string]string{"key": "same-value"})
	handler := ResourceUpdatedHandler{
		Resource:    cm,
		OldResource: cm, // Same resource = same SHA
		Collectors:  createTestCollectors(),
	}

	err := handler.Handle()

	assert.NoError(t, err)
}
