package crypto

import (
	"testing"
)

// TestGenerateSHA generates the sha from given data and verifies whether it is correct or not
func TestGenerateSHA(t *testing.T) {
	data := "www.stakater.com"
	sha := "f9c4c51315e9ad36ec77279db875ab3f1d854b9deb77dabf7eb874427c36c2f12ab409318d3afd3e029a10913f18c0ca098a1e674fe914c5d8841f14e31542b3"
	result := GenerateSHA(data)
	if result != sha {
		t.Errorf("Failed to generate SHA")
	}
}
