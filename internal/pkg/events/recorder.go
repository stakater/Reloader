package events

import (
	"time"

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
	// ReasonReloadDeferred indicates a workload reload is waiting for a sync window.
	ReasonReloadDeferred = "ReloadDeferred"

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

// ReloadDeferred records that a reload is waiting for the workload's sync windows to allow it.
func (r *Recorder) ReloadDeferred(object runtime.Object, resourceType, resourceName string, until time.Time) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(
		object,
		nil,
		EventTypeNormal,
		ReasonReloadDeferred,
		actionReloading,
		"Reload due to %s %s change deferred by sync windows until %s",
		resourceType, resourceName, until.UTC().Format(time.RFC3339),
	)
}
