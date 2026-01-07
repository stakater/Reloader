package crypto

import (
	"testing"
)

// TestGenerateSHA generates the sha from given data and verifies whether it is correct or not
func TestGenerateSHA(t *testing.T) {
	data := "www.stakater.com"
	sha := "abd4ed82fb04548388a6cf3c339fd9dc84d275df"
	result := GenerateSHA(data)
	if result != sha {
		t.Errorf("Failed to generate SHA")
	}
}

// TestGenerateSHAEmptyString verifies that empty string generates a valid hash
// This ensures consistent behavior and avoids issues with string matching operations
func TestGenerateSHAEmptyString(t *testing.T) {
	result := GenerateSHA("")
	expected := "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	if result != expected {
		t.Errorf("Failed to generate SHA for empty string. Expected: %s, Got: %s", expected, result)
	}
	if len(result) != 40 {
		t.Errorf("SHA hash should be 40 characters long, got %d", len(result))
	}
}
