package reload

import (
	"github.com/stakater/Reloader/internal/pkg/workload"
)

// ReloadDecision contains the result of evaluating whether to reload a workload.
type ReloadDecision struct {
	// Workload is the workload accessor.
	Workload workload.Workload
	// ShouldReload indicates whether the workload should be reloaded.
	ShouldReload bool
	// AutoReload indicates if this is an auto-reload.
	AutoReload bool
	// Reason provides a human-readable explanation.
	Reason string
	// Hash is the computed hash of the resource content.
	Hash string
}

// FilterDecisions returns only decisions where ShouldReload is true.
func FilterDecisions(decisions []ReloadDecision) []ReloadDecision {
	var result []ReloadDecision
	for _, d := range decisions {
		if d.ShouldReload {
			result = append(result, d)
		}
	}
	return result
}
