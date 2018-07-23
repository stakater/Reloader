package crypto

import (
	"testing"
)

// TestGenerateSHA generates the sha from given data and verifies whether it is correct or not
func TestGenerateSHA(t *testing.T) {
	data := "www.stakater.com"
	sha := GenerateSHA(data)
	length := len(sha)
	if length != 40 {
		t.Errorf("Failed to generate SHA")
	}
}
