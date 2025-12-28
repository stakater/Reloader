package events

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

const (
	// EventTypeNormal represents a normal event.
	EventTypeNormal = corev1.EventTypeNormal
	// EventTypeWarning represents a warning event.
	EventTypeWarning = corev1.EventTypeWarning

	// ReasonReloaded indicates a workload was successfully reloaded.
	ReasonReloaded = "Reloaded"
	// ReasonReloadFailed indicates a workload reload failed.
	ReasonReloadFailed = "ReloadFailed"
)

// Recorder wraps the Kubernetes event recorder.
type Recorder struct {
	recorder record.EventRecorder
}

// NewRecorder creates a new event Recorder.
func NewRecorder(recorder record.EventRecorder) *Recorder {
	if recorder == nil {
		return nil
	}
	return &Recorder{recorder: recorder}
}

// ReloadSuccess records a successful reload event.
func (r *Recorder) ReloadSuccess(object runtime.Object, resourceType, resourceName string) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Event(
		object,
		EventTypeNormal,
		ReasonReloaded,
		fmt.Sprintf("Reloaded due to %s %s change", resourceType, resourceName),
	)
}

// ReloadFailed records a failed reload event.
func (r *Recorder) ReloadFailed(object runtime.Object, resourceType, resourceName string, err error) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Event(
		object,
		EventTypeWarning,
		ReasonReloadFailed,
		fmt.Sprintf("Failed to reload due to %s %s change: %v", resourceType, resourceName, err),
	)
}
