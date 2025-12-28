package controller_test

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"k8s.io/apimachinery/pkg/labels"
)

func TestNamespaceCache_Basic(t *testing.T) {
	cache := controller.NewNamespaceCache(true)

	cache.Add("namespace-1")
	if !cache.Contains("namespace-1") {
		t.Error("Cache should contain namespace-1")
	}
	if cache.Contains("namespace-2") {
		t.Error("Cache should not contain namespace-2")
	}

	cache.Remove("namespace-1")
	if cache.Contains("namespace-1") {
		t.Error("Cache should not contain namespace-1 after removal")
	}
}

func TestNamespaceCache_Disabled(t *testing.T) {
	cache := controller.NewNamespaceCache(false)

	if !cache.Contains("any-namespace") {
		t.Error("Disabled cache should return true for any namespace")
	}
}

func TestNamespaceCache_List(t *testing.T) {
	cache := controller.NewNamespaceCache(true)
	cache.Add("ns-1")
	cache.Add("ns-2")
	cache.Add("ns-3")

	list := cache.List()
	if len(list) != 3 {
		t.Errorf("Expected 3 namespaces, got %d", len(list))
	}

	found := make(map[string]bool)
	for _, ns := range list {
		found[ns] = true
	}
	for _, expected := range []string{"ns-1", "ns-2", "ns-3"} {
		if !found[expected] {
			t.Errorf("Expected %s in list", expected)
		}
	}
}

func TestNamespaceCache_IsEnabled(t *testing.T) {
	if !controller.NewNamespaceCache(true).IsEnabled() {
		t.Error("EnabledCache.IsEnabled() should return true")
	}
	if controller.NewNamespaceCache(false).IsEnabled() {
		t.Error("DisabledCache.IsEnabled() should return false")
	}
}

func TestNamespaceReconciler_Add(t *testing.T) {
	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	ns := testutil.NewNamespace("test-ns", map[string]string{"env": "production"})
	reconciler := newNamespaceReconciler(t, cfg, cache, ns)

	assertReconcileSuccess(t, reconciler, namespaceRequest("test-ns"))

	if !cache.Contains("test-ns") {
		t.Error("Cache should contain test-ns after reconcile")
	}
}

func TestNamespaceReconciler_Remove_LabelChange(t *testing.T) {
	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	cache.Add("test-ns") // Pre-populate

	ns := testutil.NewNamespace("test-ns", map[string]string{"env": "staging"}) // Non-matching
	reconciler := newNamespaceReconciler(t, cfg, cache, ns)

	assertReconcileSuccess(t, reconciler, namespaceRequest("test-ns"))

	if cache.Contains("test-ns") {
		t.Error("Cache should not contain test-ns after reconcile (labels no longer match)")
	}
}

func TestNamespaceReconciler_Remove_Delete(t *testing.T) {
	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	cache.Add("deleted-ns") // Pre-populate

	reconciler := newNamespaceReconciler(t, cfg, cache) // No namespace in cluster

	assertReconcileSuccess(t, reconciler, namespaceRequest("deleted-ns"))

	if cache.Contains("deleted-ns") {
		t.Error("Cache should not contain deleted-ns after reconcile")
	}
}

func TestNamespaceReconciler_MultipleSelectors(t *testing.T) {
	cfg := config.NewDefault()
	selector1, _ := labels.Parse("env=production")
	selector2, _ := labels.Parse("team=platform")
	cfg.NamespaceSelectors = []labels.Selector{selector1, selector2}

	cache := controller.NewNamespaceCache(true)
	ns := testutil.NewNamespace("test-ns", map[string]string{"team": "platform"})
	reconciler := newNamespaceReconciler(t, cfg, cache, ns)

	assertReconcileSuccess(t, reconciler, namespaceRequest("test-ns"))

	if !cache.Contains("test-ns") {
		t.Error("Cache should contain test-ns (matches second selector)")
	}
}

func TestNamespaceReconciler_NoLabels(t *testing.T) {
	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	ns := testutil.NewNamespace("test-ns", nil) // No labels
	reconciler := newNamespaceReconciler(t, cfg, cache, ns)

	assertReconcileSuccess(t, reconciler, namespaceRequest("test-ns"))

	if cache.Contains("test-ns") {
		t.Error("Cache should not contain test-ns (no labels)")
	}
}
