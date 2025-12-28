package reload

import (
	"testing"
)

func TestResourceType_Kind(t *testing.T) {
	tests := []struct {
		resourceType ResourceType
		want         string
	}{
		{ResourceTypeConfigMap, "ConfigMap"},
		{ResourceTypeSecret, "Secret"},
		{ResourceType("unknown"), "unknown"},
		{ResourceType("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.resourceType), func(t *testing.T) {
			got := tt.resourceType.Kind()
			if got != tt.want {
				t.Errorf("ResourceType(%q).Kind() = %v, want %v", tt.resourceType, got, tt.want)
			}
		})
	}
}

func TestResourceTypeConstants(t *testing.T) {
	// Verify the constant values are as expected
	if ResourceTypeConfigMap != "configmap" {
		t.Errorf("ResourceTypeConfigMap = %v, want configmap", ResourceTypeConfigMap)
	}
	if ResourceTypeSecret != "secret" {
		t.Errorf("ResourceTypeSecret = %v, want secret", ResourceTypeSecret)
	}
}
