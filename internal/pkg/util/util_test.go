package util

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
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

func TestGetIgnoredResourcesList(t *testing.T) {
	// Save original state
	originalResources := options.ResourcesToIgnore
	defer func() {
		options.ResourcesToIgnore = originalResources
	}()

	tests := []struct {
		name        string
		resources   []string
		expectError bool
		expected    []string
	}{
		{
			name:        "Lowercase configmaps (canonical) normalizes to configmaps",
			resources:   []string{"configmaps"},
			expectError: false,
			expected:    []string{"configmaps"},
		},
		{
			name:        "Legacy camelCase configMaps normalizes to configmaps",
			resources:   []string{"configMaps"},
			expectError: false,
			expected:    []string{"configmaps"},
		},
		{
			name:        "Mixed-case ConfigMaps normalizes to configmaps",
			resources:   []string{"ConfigMaps"},
			expectError: false,
			expected:    []string{"configmaps"},
		},
		{
			name:        "secrets",
			resources:   []string{"secrets"},
			expectError: false,
			expected:    []string{"secrets"},
		},
		{
			name:        "Mixed-case sEcrets normalizes to secrets",
			resources:   []string{"sEcrets"},
			expectError: false,
			expected:    []string{"secrets"},
		},
		{
			name:        "Empty list",
			resources:   []string{},
			expectError: false,
			expected:    []string{},
		},
		{
			name:        "Invalid resource",
			resources:   []string{"deployments"},
			expectError: true,
			expected:    nil,
		},
		{
			name:        "Both configmaps and secrets rejected",
			resources:   []string{"configmaps", "secrets"},
			expectError: true,
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options.ResourcesToIgnore = tt.resources
			result, err := GetIgnoredResourcesList()

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
					if result[i] != expected {
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

func TestConfigureReloaderFlagsLeaderElectionTimings(t *testing.T) {
	origLease, origRenew, origRetry := options.LeaderElectionLeaseDuration, options.LeaderElectionRenewDeadline, options.LeaderElectionRetryPeriod
	defer func() {
		options.LeaderElectionLeaseDuration = origLease
		options.LeaderElectionRenewDeadline = origRenew
		options.LeaderElectionRetryPeriod = origRetry
	}()

	cmd := &cobra.Command{Use: "reloader"}
	ConfigureReloaderFlags(cmd)

	defaults := map[string]time.Duration{
		"leader-election-lease-duration": 15 * time.Second,
		"leader-election-renew-deadline": 10 * time.Second,
		"leader-election-retry-period":   2 * time.Second,
	}
	for name, want := range defaults {
		flag := cmd.PersistentFlags().Lookup(name)
		if flag == nil {
			t.Fatalf("flag --%s is not registered", name)
		}
		if flag.DefValue != want.String() {
			t.Errorf("flag --%s default: got %s, want %s", name, flag.DefValue, want)
		}
	}

	err := cmd.PersistentFlags().Parse([]string{
		"--leader-election-lease-duration=60s",
		"--leader-election-renew-deadline=45s",
		"--leader-election-retry-period=10s",
	})
	if err != nil {
		t.Fatalf("failed to parse leader election flags: %v", err)
	}

	if options.LeaderElectionLeaseDuration != 60*time.Second {
		t.Errorf("lease duration: got %s, want 60s", options.LeaderElectionLeaseDuration)
	}
	if options.LeaderElectionRenewDeadline != 45*time.Second {
		t.Errorf("renew deadline: got %s, want 45s", options.LeaderElectionRenewDeadline)
	}
	if options.LeaderElectionRetryPeriod != 10*time.Second {
		t.Errorf("retry period: got %s, want 10s", options.LeaderElectionRetryPeriod)
	}
}
