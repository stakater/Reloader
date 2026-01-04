package reload

import (
	"fmt"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/workload"
	appsv1 "k8s.io/api/apps/v1"
)

// PauseHandler handles pause deployment logic.
type PauseHandler struct {
	cfg *config.Config
}

// NewPauseHandler creates a new PauseHandler.
func NewPauseHandler(cfg *config.Config) *PauseHandler {
	return &PauseHandler{cfg: cfg}
}

// ShouldPause checks if a deployment should be paused after reload.
func (h *PauseHandler) ShouldPause(wl workload.Workload) bool {
	if wl.Kind() != workload.KindDeployment {
		return false
	}

	annotations := wl.GetAnnotations()
	if annotations == nil {
		return false
	}

	pausePeriod := annotations[h.cfg.Annotations.PausePeriod]
	return pausePeriod != ""
}

// GetPausePeriod returns the configured pause period for a workload.
func (h *PauseHandler) GetPausePeriod(wl workload.Workload) (time.Duration, error) {
	annotations := wl.GetAnnotations()
	if annotations == nil {
		return 0, fmt.Errorf("no annotations on workload")
	}

	pausePeriodStr := annotations[h.cfg.Annotations.PausePeriod]
	if pausePeriodStr == "" {
		return 0, fmt.Errorf("no pause period annotation")
	}

	return time.ParseDuration(pausePeriodStr)
}

// ApplyPause pauses a deployment and sets the paused-at annotation.
func (h *PauseHandler) ApplyPause(wl workload.Workload) error {
	deployWl, ok := wl.(*workload.DeploymentWorkload)
	if !ok {
		return fmt.Errorf("workload is not a deployment")
	}

	deploy := deployWl.GetDeployment()

	deploy.Spec.Paused = true

	if deploy.Annotations == nil {
		deploy.Annotations = make(map[string]string)
	}
	deploy.Annotations[h.cfg.Annotations.PausedAt] = time.Now().UTC().Format(time.RFC3339)

	return nil
}

// CheckPauseExpired checks if the pause period has expired for a deployment.
func (h *PauseHandler) CheckPauseExpired(deploy *appsv1.Deployment) (expired bool, remainingTime time.Duration, err error) {
	annotations := deploy.GetAnnotations()
	if annotations == nil {
		return false, 0, fmt.Errorf("no annotations on deployment")
	}

	pausePeriodStr := annotations[h.cfg.Annotations.PausePeriod]
	if pausePeriodStr == "" {
		return false, 0, fmt.Errorf("no pause period annotation")
	}

	pausedAtStr := annotations[h.cfg.Annotations.PausedAt]
	if pausedAtStr == "" {
		return false, 0, fmt.Errorf("no paused-at annotation")
	}

	pausePeriod, err := time.ParseDuration(pausePeriodStr)
	if err != nil {
		return false, 0, fmt.Errorf("invalid pause period %q: %w", pausePeriodStr, err)
	}

	pausedAt, err := time.Parse(time.RFC3339, pausedAtStr)
	if err != nil {
		return false, 0, fmt.Errorf("invalid paused-at time %q: %w", pausedAtStr, err)
	}

	elapsed := time.Since(pausedAt)
	if elapsed >= pausePeriod {
		return true, 0, nil
	}

	return false, pausePeriod - elapsed, nil
}

// ClearPause removes the pause from a deployment.
func (h *PauseHandler) ClearPause(deploy *appsv1.Deployment) {
	deploy.Spec.Paused = false
	delete(deploy.Annotations, h.cfg.Annotations.PausedAt)
}

// IsPausedByReloader checks if a deployment was paused by Reloader.
func (h *PauseHandler) IsPausedByReloader(deploy *appsv1.Deployment) bool {
	if !deploy.Spec.Paused {
		return false
	}

	annotations := deploy.GetAnnotations()
	if annotations == nil {
		return false
	}

	_, hasPausedAt := annotations[h.cfg.Annotations.PausedAt]
	_, hasPausePeriod := annotations[h.cfg.Annotations.PausePeriod]

	return hasPausedAt && hasPausePeriod
}
