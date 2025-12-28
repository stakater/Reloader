package controller_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespaceCache_Basic(t *testing.T) {
	cache := controller.NewNamespaceCache(true)

	// Test Add and Contains
	cache.Add("namespace-1")
	if !cache.Contains("namespace-1") {
		t.Error("Cache should contain namespace-1")
	}
	if cache.Contains("namespace-2") {
		t.Error("Cache should not contain namespace-2")
	}

	// Test Remove
	cache.Remove("namespace-1")
	if cache.Contains("namespace-1") {
		t.Error("Cache should not contain namespace-1 after removal")
	}
}

func TestNamespaceCache_Disabled(t *testing.T) {
	cache := controller.NewNamespaceCache(false)

	// When disabled, Contains should always return true
	if !cache.Contains("any-namespace") {
		t.Error("Disabled cache should return true for any namespace")
	}
	if !cache.Contains("other-namespace") {
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

	// Check all namespaces are in the list
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
	enabledCache := controller.NewNamespaceCache(true)
	disabledCache := controller.NewNamespaceCache(false)

	if !enabledCache.IsEnabled() {
		t.Error("EnabledCache.IsEnabled() should return true")
	}
	if disabledCache.IsEnabled() {
		t.Error("DisabledCache.IsEnabled() should return false")
	}
}

func TestNamespaceReconciler_Add(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-ns",
			Labels: map[string]string{"env": "production"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns).
		Build()

	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	reconciler := &controller.NamespaceReconciler{
		Client: fakeClient,
		Log:    testr.New(t),
		Config: cfg,
		Cache:  cache,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-ns"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !cache.Contains("test-ns") {
		t.Error("Cache should contain test-ns after reconcile")
	}
}

func TestNamespaceReconciler_Remove_LabelChange(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Namespace with non-matching labels
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-ns",
			Labels: map[string]string{"env": "staging"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns).
		Build()

	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	// Pre-populate cache
	cache.Add("test-ns")

	reconciler := &controller.NamespaceReconciler{
		Client: fakeClient,
		Log:    testr.New(t),
		Config: cfg,
		Cache:  cache,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-ns"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if cache.Contains("test-ns") {
		t.Error("Cache should not contain test-ns after reconcile (labels no longer match)")
	}
}

func TestNamespaceReconciler_Remove_Delete(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// No namespace in cluster (simulates delete)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	// Pre-populate cache
	cache.Add("deleted-ns")

	reconciler := &controller.NamespaceReconciler{
		Client: fakeClient,
		Log:    testr.New(t),
		Config: cfg,
		Cache:  cache,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "deleted-ns"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if cache.Contains("deleted-ns") {
		t.Error("Cache should not contain deleted-ns after reconcile")
	}
}

func TestNamespaceReconciler_MultipleSelectors(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-ns",
			Labels: map[string]string{"team": "platform"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns).
		Build()

	cfg := config.NewDefault()
	selector1, _ := labels.Parse("env=production")
	selector2, _ := labels.Parse("team=platform")
	cfg.NamespaceSelectors = []labels.Selector{selector1, selector2}

	cache := controller.NewNamespaceCache(true)
	reconciler := &controller.NamespaceReconciler{
		Client: fakeClient,
		Log:    testr.New(t),
		Config: cfg,
		Cache:  cache,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-ns"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Should be added because it matches second selector (team=platform)
	if !cache.Contains("test-ns") {
		t.Error("Cache should contain test-ns (matches second selector)")
	}
}

func TestNamespaceReconciler_NoLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Namespace with no labels
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns).
		Build()

	cfg := config.NewDefault()
	selector, _ := labels.Parse("env=production")
	cfg.NamespaceSelectors = []labels.Selector{selector}

	cache := controller.NewNamespaceCache(true)
	reconciler := &controller.NamespaceReconciler{
		Client: fakeClient,
		Log:    testr.New(t),
		Config: cfg,
		Cache:  cache,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-ns"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if cache.Contains("test-ns") {
		t.Error("Cache should not contain test-ns (no labels)")
	}
}
