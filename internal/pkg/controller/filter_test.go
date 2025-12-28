package controller

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestCreateEventPredicate_CreateEvent(t *testing.T) {
	tests := []struct {
		name              string
		reloadOnCreate    bool
		syncAfterRestart  bool
		initialized       bool
		expectedResult    bool
	}{
		{
			name:              "reload on create enabled, initialized",
			reloadOnCreate:    true,
			syncAfterRestart:  false,
			initialized:       true,
			expectedResult:    true,
		},
		{
			name:              "reload on create disabled, initialized",
			reloadOnCreate:    false,
			syncAfterRestart:  false,
			initialized:       true,
			expectedResult:    false,
		},
		{
			name:              "not initialized, sync after restart enabled",
			reloadOnCreate:    true,
			syncAfterRestart:  true,
			initialized:       false,
			expectedResult:    true,
		},
		{
			name:              "not initialized, sync after restart disabled",
			reloadOnCreate:    true,
			syncAfterRestart:  false,
			initialized:       false,
			expectedResult:    false,
		},
		{
			name:              "not initialized, sync after restart disabled, reload on create disabled",
			reloadOnCreate:    false,
			syncAfterRestart:  false,
			initialized:       false,
			expectedResult:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ReloadOnCreate:   tt.reloadOnCreate,
				SyncAfterRestart: tt.syncAfterRestart,
			}
			initialized := tt.initialized

			pred := createEventPredicate(cfg, &initialized)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}

			e := event.CreateEvent{Object: cm}
			result := pred.Create(e)

			if result != tt.expectedResult {
				t.Errorf("CreateFunc() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestCreateEventPredicate_UpdateEvent(t *testing.T) {
	cfg := &config.Config{}
	initialized := true

	pred := createEventPredicate(cfg, &initialized)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	e := event.UpdateEvent{ObjectOld: cm, ObjectNew: cm}
	result := pred.Update(e)

	// Update events should always return true
	if !result {
		t.Error("UpdateFunc() should always return true")
	}
}

func TestCreateEventPredicate_DeleteEvent(t *testing.T) {
	tests := []struct {
		name           string
		reloadOnDelete bool
		expectedResult bool
	}{
		{
			name:           "reload on delete enabled",
			reloadOnDelete: true,
			expectedResult: true,
		},
		{
			name:           "reload on delete disabled",
			reloadOnDelete: false,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ReloadOnDelete: tt.reloadOnDelete,
			}
			initialized := true

			pred := createEventPredicate(cfg, &initialized)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}

			e := event.DeleteEvent{Object: cm}
			result := pred.Delete(e)

			if result != tt.expectedResult {
				t.Errorf("DeleteFunc() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestCreateEventPredicate_GenericEvent(t *testing.T) {
	cfg := &config.Config{}
	initialized := true

	pred := createEventPredicate(cfg, &initialized)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	e := event.GenericEvent{Object: cm}
	result := pred.Generic(e)

	// Generic events should always return false
	if result {
		t.Error("GenericFunc() should always return false")
	}
}

func TestBuildEventFilter(t *testing.T) {
	cfg := &config.Config{
		ReloadOnCreate: true,
		ReloadOnDelete: true,
	}
	initialized := true

	// Create a simple always-true predicate as the resource predicate
	resourcePred := &alwaysTruePredicate{}

	filter := BuildEventFilter(resourcePred, cfg, &initialized)

	// The filter should be created without error
	if filter == nil {
		t.Fatal("BuildEventFilter() should return a non-nil predicate")
	}

	// Test update event passes (since resourcePred returns true and update always returns true)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	e := event.UpdateEvent{ObjectOld: cm, ObjectNew: cm}
	result := filter.Update(e)

	// Since namespace filter is empty (all namespaces allowed), this should pass
	if !result {
		t.Error("UpdateFunc() should return true when all predicates pass")
	}
}

// alwaysTruePredicate is a helper predicate for testing
type alwaysTruePredicate struct{}

func (p *alwaysTruePredicate) Create(_ event.CreateEvent) bool   { return true }
func (p *alwaysTruePredicate) Delete(_ event.DeleteEvent) bool   { return true }
func (p *alwaysTruePredicate) Update(_ event.UpdateEvent) bool   { return true }
func (p *alwaysTruePredicate) Generic(_ event.GenericEvent) bool { return true }
