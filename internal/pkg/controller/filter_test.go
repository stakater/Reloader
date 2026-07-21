package controller

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/stakater/Reloader/pkg/config"
)

func TestCreateEventPredicate_CreateEvent(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		reloadOnCreate   bool
		syncAfterRestart bool
		// createdAfterStart controls the resource's creation timestamp relative
		// to the controller start time: true => created after start (a genuine
		// post-startup create), false => created before start (initial-sync replay).
		createdAfterStart bool
		expectedResult    bool
	}{
		{
			// Regression: a genuine create after startup must be honored even
			// when it is the very first event the controller sees (no prior
			// reconcile). This is the reloadOnCreate e2e scenario.
			name:              "reload on create enabled, created after start",
			reloadOnCreate:    true,
			syncAfterRestart:  false,
			createdAfterStart: true,
			expectedResult:    true,
		},
		{
			// Pre-existing resources replayed during initial sync must not
			// trigger reloads on startup.
			name:              "reload on create enabled, created before start (initial sync replay)",
			reloadOnCreate:    true,
			syncAfterRestart:  false,
			createdAfterStart: false,
			expectedResult:    false,
		},
		{
			name:              "reload on create disabled",
			reloadOnCreate:    false,
			syncAfterRestart:  false,
			createdAfterStart: true,
			expectedResult:    false,
		},
		{
			// SyncAfterRestart processes every create, including initial-sync
			// replays of pre-existing resources.
			name:              "sync after restart honors pre-existing create",
			reloadOnCreate:    true,
			syncAfterRestart:  true,
			createdAfterStart: false,
			expectedResult:    true,
		},
		{
			name:              "sync after restart but reload on create disabled",
			reloadOnCreate:    false,
			syncAfterRestart:  true,
			createdAfterStart: true,
			expectedResult:    false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := &config.Config{
					ReloadOnCreate:   tt.reloadOnCreate,
					SyncAfterRestart: tt.syncAfterRestart,
				}

				pred := createEventPredicate(cfg, startTime)

				creationTime := startTime.Add(-time.Hour)
				if tt.createdAfterStart {
					creationTime = startTime.Add(time.Hour)
				}

				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test",
						Namespace:         "default",
						CreationTimestamp: metav1.NewTime(creationTime),
					},
				}

				e := event.CreateEvent{Object: cm}
				result := pred.Create(e)

				if result != tt.expectedResult {
					t.Errorf("CreateFunc() = %v, want %v", result, tt.expectedResult)
				}
			},
		)
	}
}

func TestCreateEventPredicate_UpdateEvent(t *testing.T) {
	cfg := &config.Config{}

	pred := createEventPredicate(cfg, time.Now())

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	e := event.UpdateEvent{ObjectOld: cm, ObjectNew: cm}
	result := pred.Update(e)

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
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := &config.Config{
					ReloadOnDelete: tt.reloadOnDelete,
				}

				pred := createEventPredicate(cfg, time.Now())

				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				}

				e := event.DeleteEvent{Object: cm}
				result := pred.Delete(e)

				if result != tt.expectedResult {
					t.Errorf("DeleteFunc() = %v, want %v", result, tt.expectedResult)
				}
			},
		)
	}
}

func TestCreateEventPredicate_GenericEvent(t *testing.T) {
	cfg := &config.Config{}

	pred := createEventPredicate(cfg, time.Now())

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	e := event.GenericEvent{Object: cm}
	result := pred.Generic(e)

	if result {
		t.Error("GenericFunc() should always return false")
	}
}

func TestBuildEventFilter(t *testing.T) {
	cfg := &config.Config{
		ReloadOnCreate: true,
		ReloadOnDelete: true,
	}

	resourcePred := &alwaysTruePredicate{}

	filter := BuildEventFilter(resourcePred, cfg, time.Now())

	if filter == nil {
		t.Fatal("BuildEventFilter() should return a non-nil predicate")
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	e := event.UpdateEvent{ObjectOld: cm, ObjectNew: cm}
	result := filter.Update(e)

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
