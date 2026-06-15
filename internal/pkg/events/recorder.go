package events

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
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

	// actionReloading is the action reported on reload events.
	actionReloading = "Reloading"
)

// Recorder wraps the Kubernetes event recorder.
type Recorder struct {
	recorder events.EventRecorder
}

// NewRecorder creates a new event Recorder.
func NewRecorder(recorder events.EventRecorder) *Recorder {
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
	r.recorder.Eventf(
		object,
		nil,
		EventTypeNormal,
		ReasonReloaded,
		actionReloading,
		"Reloaded due to %s %s change", resourceType, resourceName,
	)
}

// ReloadFailed records a failed reload event.
func (r *Recorder) ReloadFailed(object runtime.Object, resourceType, resourceName string, err error) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(
		object,
		nil,
		EventTypeWarning,
		ReasonReloadFailed,
		actionReloading,
		"Failed to reload due to %s %s change: %v", resourceType, resourceName, err,
	)
}
