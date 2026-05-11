package util

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/stakater/Reloader/internal/pkg/options"
)

func TestConvertToEnvVarName(t *testing.T) {
	data := "www.stakater.com"
	envVar := ConvertToEnvVarName(data)
	if envVar != "WWW_STAKATER_COM" {
		t.Errorf("Failed to convert data into environment variable")
	}
}

func TestGetHashFromConfigMap(t *testing.T) {
	data := map[*v1.ConfigMap]string{
		{
			Data: map[string]string{"test": "test"},
		}: "Only Data",
		{
			Data:       map[string]string{"test": "test"},
			BinaryData: map[string][]byte{"bintest": []byte("test")},
		}: "Both Data and BinaryData",
		{
			BinaryData: map[string][]byte{"bintest": []byte("test")},
		}: "Only BinaryData",
	}
	converted := map[string]string{}
	for cm, cmName := range data {
		converted[cmName] = GetSHAfromConfigmap(cm)
	}

	// Test that the has for each configmap is really unique
	for cmName, cmHash := range converted {
		count := 0
		for _, cmHash2 := range converted {
			if cmHash == cmHash2 {
				count++
			}
		}
		if count > 1 {
			t.Errorf("Found duplicate hashes for %v", cmName)
		}
	}
}

func TestGetIgnoredWorkloadTypesList(t *testing.T) {
	// Save original state
	originalWorkloadTypes := options.WorkloadTypesToIgnore
	defer func() {
		options.WorkloadTypesToIgnore = originalWorkloadTypes
	}()

	tests := []struct {
		name          string
		workloadTypes []string
		expectError   bool
		expected      []string
	}{
		{
			name:          "Both jobs and cronjobs",
			workloadTypes: []string{"jobs", "cronjobs"},
			expectError:   false,
			expected:      []string{"jobs", "cronjobs"},
		},
		{
			name:          "Only jobs",
			workloadTypes: []string{"jobs"},
			expectError:   false,
			expected:      []string{"jobs"},
		},
		{
			name:          "Only cronjobs",
			workloadTypes: []string{"cronjobs"},
			expectError:   false,
			expected:      []string{"cronjobs"},
		},
		{
			name:          "Empty list",
			workloadTypes: []string{},
			expectError:   false,
			expected:      []string{},
		},
		{
			name:          "Invalid workload type",
			workloadTypes: []string{"invalid"},
			expectError:   true,
			expected:      nil,
		},
		{
			name:          "Mixed valid and invalid",
			workloadTypes: []string{"jobs", "invalid"},
			expectError:   true,
			expected:      nil,
		},
		{
			name:          "Duplicate values",
			workloadTypes: []string{"jobs", "jobs"},
			expectError:   false,
			expected:      []string{"jobs", "jobs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the global option
			options.WorkloadTypesToIgnore = tt.workloadTypes

			result, err := GetIgnoredWorkloadTypesList()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if len(result) != len(tt.expected) {
					t.Errorf("Expected %v, got %v", tt.expected, result)
					return
				}

				for i, expected := range tt.expected {
					if i >= len(result) || result[i] != expected {
						t.Errorf("Expected %v, got %v", tt.expected, result)
						break
					}
				}
			}
		})
	}
}

func TestListContains(t *testing.T) {
	tests := []struct {
		name     string
		list     List
		item     string
		expected bool
	}{
		{
			name:     "List contains item",
			list:     List{"jobs", "cronjobs"},
			item:     "jobs",
			expected: true,
		},
		{
			name:     "List does not contain item",
			list:     List{"jobs"},
			item:     "cronjobs",
			expected: false,
		},
		{
			name:     "Empty list",
			list:     List{},
			item:     "jobs",
			expected: false,
		},
		{
			name:     "Case sensitive matching",
			list:     List{"jobs", "cronjobs"},
			item:     "Jobs",
			expected: false,
		},
		{
			name:     "Multiple occurrences",
			list:     List{"jobs", "jobs", "cronjobs"},
			item:     "jobs",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.list.Contains(tt.item)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
