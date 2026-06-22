package reload

import (
	"testing"

	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// TestCSIDependencyAvailable ensures the CSI types are importable and the
// fields this feature depends on exist.
func TestCSIDependencyAvailable(t *testing.T) {
	status := csiv1.SecretProviderClassPodStatusStatus{
		SecretProviderClassName: "spc",
		Objects: []csiv1.SecretProviderClassObject{
			{ID: "secret/data/foo", Version: "1"},
		},
	}
	if status.SecretProviderClassName != "spc" {
		t.Fatalf("unexpected SecretProviderClassName")
	}
	if len(status.Objects) != 1 || status.Objects[0].ID != "secret/data/foo" {
		t.Fatalf("unexpected Objects")
	}
}
