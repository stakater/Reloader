package crypto

import (
	"testing"
)

// TestGenerateSHA generates the sha from given data and verifies whether it is correct or not
func TestGenerateSHA(t *testing.T) {
	data := "www.stakater.com"
	sha := "2e9aa975331b22861b4f62b7fcc69b63e001f938361fee3b4ed888adf26a10e3"
	result := GenerateSHA(data)
	if result != sha {
		t.Errorf("Failed to generate SHA")
	}
}

// TestGenerateSHAEmptyString verifies that empty string generates a valid hash
// This ensures consistent behavior and avoids issues with string matching operations
func TestGenerateSHAEmptyString(t *testing.T) {
	result := GenerateSHA("")
	expected := "c672b8d1ef56ed28ab87c3622c5114069bdd3ad7b8f9737498d0c01ecef0967a"
	if result != expected {
		t.Errorf("Failed to generate SHA for empty string. Expected: %s, Got: %s", expected, result)
	}
	if len(result) != 64 {
		t.Errorf("SHA hash should be 64 characters long, got %d", len(result))
	}
}
