package utils

import (
	"testing"
)

func TestMergeAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		maps     []map[string]string
		expected map[string]string
	}{
		{
			name:     "no maps",
			maps:     []map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "single map",
			maps: []map[string]string{
				{"key1": "value1"},
			},
			expected: map[string]string{
				"key1": "value1",
			},
		},
		{
			name: "two maps no overlap",
			maps: []map[string]string{
				{"key1": "value1"},
				{"key2": "value2"},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "three maps with overlap - last wins",
			maps: []map[string]string{
				{"key1": "value1", "shared": "first"},
				{"key2": "value2", "shared": "second"},
				{"key3": "value3", "shared": "third"},
			},
			expected: map[string]string{
				"key1":   "value1",
				"key2":   "value2",
				"key3":   "value3",
				"shared": "third", // Last map wins
			},
		},
		{
			name: "empty map in the middle",
			maps: []map[string]string{
				{"key1": "value1"},
				{},
				{"key2": "value2"},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "nil map in the middle",
			maps: []map[string]string{
				{"key1": "value1"},
				nil,
				{"key2": "value2"},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "realistic use case - auto annotation with reload annotation",
			maps: []map[string]string{
				BuildAutoTrueAnnotation(),
				BuildConfigMapReloadAnnotation("my-config"),
			},
			expected: map[string]string{
				AnnotationAuto:            AnnotationValueTrue,
				AnnotationConfigMapReload: "my-config",
			},
		},
		{
			name: "realistic use case - pause period with reload annotation",
			maps: []map[string]string{
				BuildConfigMapReloadAnnotation("config1"),
				BuildPausePeriodAnnotation("10s"),
			},
			expected: map[string]string{
				AnnotationConfigMapReload:       "config1",
				AnnotationDeploymentPausePeriod: "10s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeAnnotations(tt.maps...)

			if len(result) != len(tt.expected) {
				t.Errorf("MergeAnnotations() returned %d entries, want %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Want: %v", tt.expected)
				return
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("MergeAnnotations()[%q] = %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

func TestMergeAnnotationsDoesNotModifyInput(t *testing.T) {
	// Ensure MergeAnnotations doesn't modify the input maps
	map1 := map[string]string{"key1": "value1"}
	map2 := map[string]string{"key2": "value2"}

	_ = MergeAnnotations(map1, map2)

	// Verify original maps are unchanged
	if len(map1) != 1 || map1["key1"] != "value1" {
		t.Errorf("map1 was modified: %v", map1)
	}
	if len(map2) != 1 || map2["key2"] != "value2" {
		t.Errorf("map2 was modified: %v", map2)
	}
}

func TestMergeAnnotationsReturnsNewMap(t *testing.T) {
	// Ensure MergeAnnotations returns a new map, not a reference to an input
	input := map[string]string{"key1": "value1"}
	result := MergeAnnotations(input)

	// Modify the result
	result["key2"] = "value2"

	// Verify original is unchanged
	if _, exists := input["key2"]; exists {
		t.Error("modifying result affected input map - should return a new map")
	}
}
