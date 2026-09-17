package reload

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
)

// PauseHandler handles pause deployment logic.
type PauseHandler struct {
	cfg *config.Config
}

// NewPauseHandler creates a new PauseHandler.
func NewPauseHandler(cfg *config.Config) *PauseHandler {
	return &PauseHandler{cfg: cfg}
}

// ShouldPause reports whether a deployment should be paused after reload. It returns
// a non-nil error when the pause-period annotation is present but unusable, in which
// case the caller reloads without pausing: pausing on a zero or negative period would
// unpause again on the next reconcile, and pausing on an unparseable one would leave
// the deployment paused with no expiry to hit.
func (h *PauseHandler) ShouldPause(wl workload.Workload) (bool, error) {
	if wl.Kind() != workload.KindDeployment {
		return false, nil
	}

	annotations := wl.GetAnnotations()
	if annotations == nil {
		return false, nil
	}

	pausePeriod := annotations[h.cfg.Annotations.PausePeriod]
	if pausePeriod == "" {
		return false, nil
	}

	if _, err := parsePausePeriod(pausePeriod); err != nil {
		return false, err
	}
	return true, nil
}

// parsePausePeriod parses a pause-period annotation value, accepting only positive
// durations.
func parsePausePeriod(value string) (time.Duration, error) {
	period, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid pause period %q: %w", value, err)
	}
	if period <= 0 {
		return 0, fmt.Errorf("pause period must be positive, got %q", value)
	}
	return period, nil
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

	return parsePausePeriod(pausePeriodStr)
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

	pausePeriod, err := parsePausePeriod(pausePeriodStr)
	if err != nil {
		return false, 0, err
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
