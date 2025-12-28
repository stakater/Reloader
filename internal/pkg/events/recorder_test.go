package events

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

func TestNewRecorder_NilInput(t *testing.T) {
	r := NewRecorder(nil)
	if r != nil {
		t.Error("NewRecorder(nil) should return nil")
	}
}

func TestNewRecorder_ValidInput(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	r := NewRecorder(fakeRecorder)
	if r == nil {
		t.Error("NewRecorder with valid recorder should not return nil")
	}
}

func TestReloadSuccess_RecordsEvent(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	r := NewRecorder(fakeRecorder)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	r.ReloadSuccess(pod, "ConfigMap", "my-config")

	select {
	case event := <-fakeRecorder.Events:
		if event == "" {
			t.Error("Expected event to be recorded")
		}
		// Event format: "Normal Reloaded Reloaded due to ConfigMap my-config change"
		expectedContains := []string{"Normal", "Reloaded", "ConfigMap", "my-config"}
		for _, expected := range expectedContains {
			if !contains(event, expected) {
				t.Errorf("Event %q should contain %q", event, expected)
			}
		}
	default:
		t.Error("Expected event to be recorded, but none was")
	}
}

func TestReloadFailed_RecordsWarningEvent(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	r := NewRecorder(fakeRecorder)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	testErr := errors.New("update conflict")
	r.ReloadFailed(pod, "Secret", "my-secret", testErr)

	select {
	case event := <-fakeRecorder.Events:
		if event == "" {
			t.Error("Expected event to be recorded")
		}
		// Event format: "Warning ReloadFailed Failed to reload due to Secret my-secret change: update conflict"
		expectedContains := []string{"Warning", "ReloadFailed", "Secret", "my-secret", "update conflict"}
		for _, expected := range expectedContains {
			if !contains(event, expected) {
				t.Errorf("Event %q should contain %q", event, expected)
			}
		}
	default:
		t.Error("Expected event to be recorded, but none was")
	}
}

func TestNilRecorder_NoPanic(t *testing.T) {
	var r *Recorder = nil

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	// These should not panic
	r.ReloadSuccess(pod, "ConfigMap", "my-config")
	r.ReloadFailed(pod, "Secret", "my-secret", errors.New("test error"))
}

func TestRecorder_NilInternalRecorder(t *testing.T) {
	// Create a Recorder with nil internal recorder (edge case)
	r := &Recorder{recorder: nil}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	// These should not panic
	r.ReloadSuccess(pod, "ConfigMap", "my-config")
	r.ReloadFailed(pod, "Secret", "my-secret", errors.New("test error"))
}

func TestEventConstants(t *testing.T) {
	if EventTypeNormal != corev1.EventTypeNormal {
		t.Errorf("EventTypeNormal = %q, want %q", EventTypeNormal, corev1.EventTypeNormal)
	}
	if EventTypeWarning != corev1.EventTypeWarning {
		t.Errorf("EventTypeWarning = %q, want %q", EventTypeWarning, corev1.EventTypeWarning)
	}
	if ReasonReloaded != "Reloaded" {
		t.Errorf("ReasonReloaded = %q, want %q", ReasonReloaded, "Reloaded")
	}
	if ReasonReloadFailed != "ReloadFailed" {
		t.Errorf("ReasonReloadFailed = %q, want %q", ReasonReloadFailed, "ReloadFailed")
	}
}

func TestReloadSuccess_DifferentObjectTypes(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	r := NewRecorder(fakeRecorder)

	tests := []struct {
		name   string
		object runtime.Object
	}{
		{
			name: "Pod",
			object: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
			},
		},
		{
			name: "ConfigMap",
			object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "default"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.ReloadSuccess(tt.object, "ConfigMap", "my-config")

			select {
			case event := <-fakeRecorder.Events:
				if event == "" {
					t.Error("Expected event to be recorded")
				}
			default:
				t.Error("Expected event to be recorded")
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
