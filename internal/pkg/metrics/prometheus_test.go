package metrics

import (
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewCollectors_CreatesCounters(t *testing.T) {
	collectors := NewCollectors()

	if collectors.Reloaded == nil {
		t.Error("NewCollectors() should create Reloaded counter")
	}
	if collectors.ReloadedByNamespace == nil {
		t.Error("NewCollectors() should create ReloadedByNamespace counter")
	}
}

func TestNewCollectors_InitializesWithZero(t *testing.T) {
	collectors := NewCollectors()

	// Check that success=true counter is initialized to 0
	metric := &dto.Metric{}
	err := collectors.Reloaded.With(prometheus.Labels{"success": "true"}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 0 {
		t.Errorf("Initial success=true counter = %v, want 0", metric.Counter.GetValue())
	}

	// Check that success=false counter is initialized to 0
	err = collectors.Reloaded.With(prometheus.Labels{"success": "false"}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 0 {
		t.Errorf("Initial success=false counter = %v, want 0", metric.Counter.GetValue())
	}
}

func TestRecordReload_Success(t *testing.T) {
	collectors := NewCollectors()
	collectors.RecordReload(true, "default")

	metric := &dto.Metric{}
	err := collectors.Reloaded.With(prometheus.Labels{"success": "true"}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 1 {
		t.Errorf("success=true counter = %v, want 1", metric.Counter.GetValue())
	}
}

func TestRecordReload_Failure(t *testing.T) {
	collectors := NewCollectors()
	collectors.RecordReload(false, "default")

	metric := &dto.Metric{}
	err := collectors.Reloaded.With(prometheus.Labels{"success": "false"}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 1 {
		t.Errorf("success=false counter = %v, want 1", metric.Counter.GetValue())
	}
}

func TestRecordReload_MultipleIncrements(t *testing.T) {
	collectors := NewCollectors()
	collectors.RecordReload(true, "default")
	collectors.RecordReload(true, "default")
	collectors.RecordReload(false, "default")

	metric := &dto.Metric{}

	err := collectors.Reloaded.With(prometheus.Labels{"success": "true"}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 2 {
		t.Errorf("success=true counter = %v, want 2", metric.Counter.GetValue())
	}

	err = collectors.Reloaded.With(prometheus.Labels{"success": "false"}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 1 {
		t.Errorf("success=false counter = %v, want 1", metric.Counter.GetValue())
	}
}

func TestRecordReload_WithNamespaceTracking(t *testing.T) {
	// Enable namespace tracking
	os.Setenv("METRICS_COUNT_BY_NAMESPACE", "enabled")
	defer os.Unsetenv("METRICS_COUNT_BY_NAMESPACE")

	collectors := NewCollectors()
	collectors.RecordReload(true, "kube-system")

	metric := &dto.Metric{}
	err := collectors.ReloadedByNamespace.With(prometheus.Labels{
		"success":   "true",
		"namespace": "kube-system",
	}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 1 {
		t.Errorf("namespace counter = %v, want 1", metric.Counter.GetValue())
	}
}

func TestRecordReload_WithoutNamespaceTracking(t *testing.T) {
	// Ensure namespace tracking is disabled
	os.Unsetenv("METRICS_COUNT_BY_NAMESPACE")

	collectors := NewCollectors()
	collectors.RecordReload(true, "kube-system")

	// The ReloadedByNamespace counter should not be incremented
	// We can verify by checking countByNamespace is false
	if collectors.countByNamespace {
		t.Error("countByNamespace should be false when env var is not set")
	}
}

func TestNilCollectors_NoPanic(t *testing.T) {
	var c *Collectors = nil

	// This should not panic
	c.RecordReload(true, "default")
	c.RecordReload(false, "default")
}

func TestRecordReload_DifferentNamespaces(t *testing.T) {
	os.Setenv("METRICS_COUNT_BY_NAMESPACE", "enabled")
	defer os.Unsetenv("METRICS_COUNT_BY_NAMESPACE")

	collectors := NewCollectors()
	collectors.RecordReload(true, "namespace-a")
	collectors.RecordReload(true, "namespace-b")
	collectors.RecordReload(true, "namespace-a")

	metric := &dto.Metric{}

	// Check namespace-a has 2 reloads
	err := collectors.ReloadedByNamespace.With(prometheus.Labels{
		"success":   "true",
		"namespace": "namespace-a",
	}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 2 {
		t.Errorf("namespace-a counter = %v, want 2", metric.Counter.GetValue())
	}

	// Check namespace-b has 1 reload
	err = collectors.ReloadedByNamespace.With(prometheus.Labels{
		"success":   "true",
		"namespace": "namespace-b",
	}).Write(metric)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}
	if metric.Counter.GetValue() != 1 {
		t.Errorf("namespace-b counter = %v, want 1", metric.Counter.GetValue())
	}
}

func TestCollectors_MetricNames(t *testing.T) {
	collectors := NewCollectors()

	// Verify the Reloaded metric has correct description
	ch := make(chan *prometheus.Desc, 10)
	collectors.Reloaded.Describe(ch)
	close(ch)

	found := false
	for desc := range ch {
		if desc.String() != "" {
			found = true
		}
	}
	if !found {
		t.Error("Expected Reloaded metric to have a description")
	}
}
