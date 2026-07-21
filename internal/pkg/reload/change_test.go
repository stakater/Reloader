package reload

import (
	"testing"

	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/pkg/matcher"
)

func TestSecretProviderClassChange(t *testing.T) {
	status := csiv1.SecretProviderClassPodStatusStatus{
		SecretProviderClassName: "my-spc",
		Objects:                 []csiv1.SecretProviderClassObject{{ID: "a", Version: "1"}},
	}
	c := SecretProviderClassChange{
		Name:        "my-spc",
		Namespace:   "ns1",
		Annotations: map[string]string{"k": "v"},
		Status:      status,
		EventType:   EventTypeUpdate,
	}

	if c.IsNil() {
		t.Fatal("IsNil() = true, want false")
	}
	if c.GetName() != "my-spc" {
		t.Fatalf("GetName() = %q", c.GetName())
	}
	if c.GetNamespace() != "ns1" {
		t.Fatalf("GetNamespace() = %q", c.GetNamespace())
	}
	if c.GetResourceType() != matcher.ResourceTypeSecretProviderClass {
		t.Fatalf("GetResourceType() = %q", c.GetResourceType())
	}
	if c.GetEventType() != EventTypeUpdate {
		t.Fatalf("GetEventType() = %q", c.GetEventType())
	}
	if c.GetAnnotations()["k"] != "v" {
		t.Fatalf("GetAnnotations() missing key")
	}
	h := NewHasher()
	if c.ComputeHash(h) != h.HashSecretProviderClass(status) {
		t.Fatalf("ComputeHash mismatch")
	}
}

func TestSecretProviderClassChangeIsNil(t *testing.T) {
	c := SecretProviderClassChange{Name: ""}
	if !c.IsNil() {
		t.Fatal("IsNil() = false, want true for empty name")
	}
}
