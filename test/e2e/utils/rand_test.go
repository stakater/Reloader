package utils

import (
	"regexp"
	"testing"
)

func TestRandSeq(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"length 0", 0},
		{"length 1", 1},
		{"length 5", 5},
		{"length 10", 10},
		{"length 100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RandSeq(tt.length)

			// Verify length
			if len(result) != tt.length {
				t.Errorf("RandSeq(%d) returned string of length %d, want %d",
					tt.length, len(result), tt.length)
			}

			// Verify only lowercase letters
			if tt.length > 0 {
				matched, _ := regexp.MatchString("^[a-z]+$", result)
				if !matched {
					t.Errorf("RandSeq(%d) = %q, contains non-lowercase letters", tt.length, result)
				}
			}
		})
	}
}

func TestRandSeqRandomness(t *testing.T) {
	// Generate multiple sequences and verify they're different
	// (with very high probability)
	const iterations = 10
	const length = 20

	seen := make(map[string]bool)
	for i := 0; i < iterations; i++ {
		s := RandSeq(length)
		if seen[s] {
			// This is extremely unlikely with 20 chars (26^20 possibilities)
			t.Errorf("RandSeq generated duplicate: %q", s)
		}
		seen[s] = true
	}

	// Verify we got 10 unique strings
	if len(seen) != iterations {
		t.Errorf("Expected %d unique strings, got %d", iterations, len(seen))
	}
}

func TestRandName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"deploy prefix", "deploy"},
		{"cm prefix", "cm"},
		{"secret prefix", "secret"},
		{"test-app prefix", "test-app"},
		{"empty prefix", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RandName(tt.prefix)

			// Verify format: prefix-xxxxx
			expectedPrefix := tt.prefix + "-"
			if len(result) <= len(expectedPrefix) {
				t.Errorf("RandName(%q) = %q, too short", tt.prefix, result)
				return
			}

			// Check prefix
			if result[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("RandName(%q) = %q, doesn't start with %q",
					tt.prefix, result, expectedPrefix)
			}

			// Check random suffix is 5 lowercase letters
			suffix := result[len(expectedPrefix):]
			if len(suffix) != 5 {
				t.Errorf("RandName(%q) suffix length = %d, want 5", tt.prefix, len(suffix))
			}

			matched, _ := regexp.MatchString("^[a-z]{5}$", suffix)
			if !matched {
				t.Errorf("RandName(%q) suffix = %q, should be 5 lowercase letters",
					tt.prefix, suffix)
			}
		})
	}
}

func TestRandNameUniqueness(t *testing.T) {
	// Generate multiple names with same prefix and verify uniqueness
	const prefix = "test"
	const iterations = 100

	seen := make(map[string]bool)
	for i := 0; i < iterations; i++ {
		name := RandName(prefix)
		if seen[name] {
			t.Errorf("RandName generated duplicate: %q", name)
		}
		seen[name] = true
	}
}

func TestRandNameKubernetesCompatibility(t *testing.T) {
	// Verify generated names are valid Kubernetes resource names
	// Must match: [a-z0-9]([-a-z0-9]*[a-z0-9])?

	prefixes := []string{"deploy", "cm", "secret", "test-app", "my-resource"}
	k8sNamePattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	for _, prefix := range prefixes {
		name := RandName(prefix)
		if !k8sNamePattern.MatchString(name) {
			t.Errorf("RandName(%q) = %q is not a valid Kubernetes name", prefix, name)
		}
	}
}
