package matcher

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
		t.Run(
			string(tt.resourceType), func(t *testing.T) {
				got := tt.resourceType.Kind()
				if got != tt.want {
					t.Errorf("ResourceType(%q).Kind() = %v, want %v", tt.resourceType, got, tt.want)
				}
			},
		)
	}
}

func TestResourceTypeSecretProviderClassKind(t *testing.T) {
	if got := ResourceTypeSecretProviderClass.Kind(); got != "SecretProviderClass" {
		t.Fatalf("Kind() = %q, want SecretProviderClass", got)
	}
	if string(ResourceTypeSecretProviderClass) != "secretproviderclass" {
		t.Fatalf("value = %q, want secretproviderclass", ResourceTypeSecretProviderClass)
	}
}
