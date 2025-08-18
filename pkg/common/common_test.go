package common

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/options"
)

func TestShouldReload_IgnoredWorkloadTypes(t *testing.T) {
	// Save original state
	originalWorkloadTypes := options.WorkloadTypesToIgnore
	defer func() {
		options.WorkloadTypesToIgnore = originalWorkloadTypes
	}()

	tests := []struct {
		name                 string
		ignoredWorkloadTypes []string
		resourceType         string
		shouldReload         bool
		description          string
	}{
		{
			name:                 "Jobs ignored - Job should not reload",
			ignoredWorkloadTypes: []string{"jobs"},
			resourceType:         "Job",
			shouldReload:         false,
			description:          "When jobs are ignored, Job resources should not be reloaded",
		},
		{
			name:                 "Jobs ignored - CronJob should reload",
			ignoredWorkloadTypes: []string{"jobs"},
			resourceType:         "CronJob",
			shouldReload:         true,
			description:          "When jobs are ignored, CronJob resources should still be processed",
		},
		{
			name:                 "CronJobs ignored - CronJob should not reload",
			ignoredWorkloadTypes: []string{"cronjobs"},
			resourceType:         "CronJob",
			shouldReload:         false,
			description:          "When cronjobs are ignored, CronJob resources should not be reloaded",
		},
		{
			name:                 "CronJobs ignored - Job should reload",
			ignoredWorkloadTypes: []string{"cronjobs"},
			resourceType:         "Job",
			shouldReload:         true,
			description:          "When cronjobs are ignored, Job resources should still be processed",
		},
		{
			name:                 "Both ignored - Job should not reload",
			ignoredWorkloadTypes: []string{"jobs", "cronjobs"},
			resourceType:         "Job",
			shouldReload:         false,
			description:          "When both are ignored, Job resources should not be reloaded",
		},
		{
			name:                 "Both ignored - CronJob should not reload",
			ignoredWorkloadTypes: []string{"jobs", "cronjobs"},
			resourceType:         "CronJob",
			shouldReload:         false,
			description:          "When both are ignored, CronJob resources should not be reloaded",
		},
		{
			name:                 "Both ignored - Deployment should reload",
			ignoredWorkloadTypes: []string{"jobs", "cronjobs"},
			resourceType:         "Deployment",
			shouldReload:         true,
			description:          "When both are ignored, other workload types should still be processed",
		},
		{
			name:                 "None ignored - Job should reload",
			ignoredWorkloadTypes: []string{},
			resourceType:         "Job",
			shouldReload:         true,
			description:          "When nothing is ignored, all workload types should be processed",
		},
		{
			name:                 "None ignored - CronJob should reload",
			ignoredWorkloadTypes: []string{},
			resourceType:         "CronJob",
			shouldReload:         true,
			description:          "When nothing is ignored, all workload types should be processed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the ignored workload types
			options.WorkloadTypesToIgnore = tt.ignoredWorkloadTypes

			// Create minimal test config and options
			config := Config{
				ResourceName: "test-resource",
				Annotation:   "configmap.reloader.stakater.com/reload",
			}

			annotations := Map{
				"configmap.reloader.stakater.com/reload": "test-config",
			}

			// Create ReloaderOptions with the ignored workload types
			opts := &ReloaderOptions{
				WorkloadTypesToIgnore:  tt.ignoredWorkloadTypes,
				AutoReloadAll:          true, // Enable auto-reload to simplify test
				ReloaderAutoAnnotation: "reloader.stakater.com/auto",
			}

			// Call ShouldReload
			result := ShouldReload(config, tt.resourceType, annotations, Map{}, opts)

			// Check the result
			if result.ShouldReload != tt.shouldReload {
				t.Errorf("For resource type %s with ignored types %v, expected ShouldReload=%v, got=%v",
					tt.resourceType, tt.ignoredWorkloadTypes, tt.shouldReload, result.ShouldReload)
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}

func TestShouldReload_IgnoredWorkloadTypes_ValidationError(t *testing.T) {
	// Save original state
	originalWorkloadTypes := options.WorkloadTypesToIgnore
	defer func() {
		options.WorkloadTypesToIgnore = originalWorkloadTypes
	}()

	// Test with invalid workload type - should still continue processing
	options.WorkloadTypesToIgnore = []string{"invalid"}

	config := Config{
		ResourceName: "test-resource",
		Annotation:   "configmap.reloader.stakater.com/reload",
	}

	annotations := Map{
		"configmap.reloader.stakater.com/reload": "test-config",
	}

	opts := &ReloaderOptions{
		WorkloadTypesToIgnore:  []string{"invalid"},
		AutoReloadAll:          true, // Enable auto-reload to simplify test
		ReloaderAutoAnnotation: "reloader.stakater.com/auto",
	}

	// Should not panic and should continue with normal processing
	result := ShouldReload(config, "Job", annotations, Map{}, opts)

	// Since validation failed, it should continue with normal processing (should reload)
	if !result.ShouldReload {
		t.Errorf("Expected ShouldReload=true when validation fails, got=%v", result.ShouldReload)
	}
}

// Test that validates the fix for issue #996
func TestShouldReload_IssueRBACPermissionFixed(t *testing.T) {
	// Save original state
	originalWorkloadTypes := options.WorkloadTypesToIgnore
	defer func() {
		options.WorkloadTypesToIgnore = originalWorkloadTypes
	}()

	tests := []struct {
		name                 string
		ignoredWorkloadTypes []string
		resourceType         string
		description          string
	}{
		{
			name:                 "Issue #996 - ignoreJobs prevents Job processing",
			ignoredWorkloadTypes: []string{"jobs"},
			resourceType:         "Job",
			description:          "Job resources are skipped entirely, preventing RBAC permission errors",
		},
		{
			name:                 "Issue #996 - ignoreCronJobs prevents CronJob processing",
			ignoredWorkloadTypes: []string{"cronjobs"},
			resourceType:         "CronJob",
			description:          "CronJob resources are skipped entirely, preventing RBAC permission errors",
		},
		{
			name:                 "Issue #996 - both ignored prevent both types",
			ignoredWorkloadTypes: []string{"jobs", "cronjobs"},
			resourceType:         "Job",
			description:          "Job resources are skipped entirely when both types are ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the ignored workload types
			options.WorkloadTypesToIgnore = tt.ignoredWorkloadTypes

			config := Config{
				ResourceName: "test-resource",
				Annotation:   "configmap.reloader.stakater.com/reload",
			}

			annotations := Map{
				"configmap.reloader.stakater.com/reload": "test-config",
			}

			opts := &ReloaderOptions{
				WorkloadTypesToIgnore:  tt.ignoredWorkloadTypes,
				AutoReloadAll:          true, // Enable auto-reload to simplify test
				ReloaderAutoAnnotation: "reloader.stakater.com/auto",
			}

			// Call ShouldReload
			result := ShouldReload(config, tt.resourceType, annotations, Map{}, opts)

			// Should not reload when workload type is ignored
			if result.ShouldReload {
				t.Errorf("Expected ShouldReload=false for ignored workload type %s, got=%v",
					tt.resourceType, result.ShouldReload)
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}
