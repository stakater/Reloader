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

			if len(result) != tt.length {
				t.Errorf("RandSeq(%d) returned string of length %d, want %d",
					tt.length, len(result), tt.length)
			}

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
	const iterations = 10
	const length = 20

	seen := make(map[string]bool)
	for i := 0; i < iterations; i++ {
		s := RandSeq(length)
		if seen[s] {
			t.Errorf("RandSeq generated duplicate: %q", s)
		}
		seen[s] = true
	}

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

			expectedPrefix := tt.prefix + "-"
			if len(result) <= len(expectedPrefix) {
				t.Errorf("RandName(%q) = %q, too short", tt.prefix, result)
				return
			}

			if result[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("RandName(%q) = %q, doesn't start with %q",
					tt.prefix, result, expectedPrefix)
			}

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
	prefixes := []string{"deploy", "cm", "secret", "test-app", "my-resource"}
	k8sNamePattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	for _, prefix := range prefixes {
		name := RandName(prefix)
		if !k8sNamePattern.MatchString(name) {
			t.Errorf("RandName(%q) = %q is not a valid Kubernetes name", prefix, name)
		}
	}
}
