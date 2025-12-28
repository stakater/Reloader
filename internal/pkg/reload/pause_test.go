package reload

import (
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/workload"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPauseHandler_ShouldPause(t *testing.T) {
	cfg := config.NewDefault()
	handler := NewPauseHandler(cfg)

	tests := []struct {
		name     string
		workload workload.WorkloadAccessor
		want     bool
	}{
		{
			name: "deployment with pause period",
			workload: workload.NewDeploymentWorkload(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "5m",
					},
				},
			}),
			want: true,
		},
		{
			name: "deployment without pause period",
			workload: workload.NewDeploymentWorkload(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{},
			}),
			want: false,
		},
		{
			name: "daemonset with pause period (ignored)",
			workload: workload.NewDaemonSetWorkload(&appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "5m",
					},
				},
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.ShouldPause(tt.workload)
			if got != tt.want {
				t.Errorf("ShouldPause() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPauseHandler_GetPausePeriod(t *testing.T) {
	cfg := config.NewDefault()
	handler := NewPauseHandler(cfg)

	tests := []struct {
		name        string
		workload    workload.WorkloadAccessor
		wantPeriod  time.Duration
		wantErr     bool
	}{
		{
			name: "valid pause period",
			workload: workload.NewDeploymentWorkload(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "5m",
					},
				},
			}),
			wantPeriod: 5 * time.Minute,
			wantErr:    false,
		},
		{
			name: "invalid pause period",
			workload: workload.NewDeploymentWorkload(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "invalid",
					},
				},
			}),
			wantErr: true,
		},
		{
			name: "no pause period annotation",
			workload: workload.NewDeploymentWorkload(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{},
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handler.GetPausePeriod(tt.workload)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPausePeriod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantPeriod {
				t.Errorf("GetPausePeriod() = %v, want %v", got, tt.wantPeriod)
			}
		})
	}
}

func TestPauseHandler_ApplyPause(t *testing.T) {
	cfg := config.NewDefault()
	handler := NewPauseHandler(cfg)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deploy",
		},
		Spec: appsv1.DeploymentSpec{
			Paused: false,
		},
	}

	wl := workload.NewDeploymentWorkload(deploy)
	err := handler.ApplyPause(wl)
	if err != nil {
		t.Fatalf("ApplyPause() error = %v", err)
	}

	if !deploy.Spec.Paused {
		t.Error("Expected deployment to be paused")
	}

	pausedAt := deploy.Annotations[cfg.Annotations.PausedAt]
	if pausedAt == "" {
		t.Error("Expected paused-at annotation to be set")
	}

	// Verify the timestamp is valid
	_, err = time.Parse(time.RFC3339, pausedAt)
	if err != nil {
		t.Errorf("Invalid paused-at timestamp: %v", err)
	}
}

func TestPauseHandler_CheckPauseExpired(t *testing.T) {
	cfg := config.NewDefault()
	handler := NewPauseHandler(cfg)

	tests := []struct {
		name        string
		deploy      *appsv1.Deployment
		wantExpired bool
		wantErr     bool
	}{
		{
			name: "pause expired",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "1ms",
						cfg.Annotations.PausedAt:    time.Now().Add(-time.Second).UTC().Format(time.RFC3339),
					},
				},
				Spec: appsv1.DeploymentSpec{Paused: true},
			},
			wantExpired: true,
			wantErr:     false,
		},
		{
			name: "pause not expired",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "1h",
						cfg.Annotations.PausedAt:    time.Now().UTC().Format(time.RFC3339),
					},
				},
				Spec: appsv1.DeploymentSpec{Paused: true},
			},
			wantExpired: false,
			wantErr:     false,
		},
		{
			name: "no paused-at annotation",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "5m",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid pause period",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "invalid",
						cfg.Annotations.PausedAt:    time.Now().UTC().Format(time.RFC3339),
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expired, _, err := handler.CheckPauseExpired(tt.deploy)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPauseExpired() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && expired != tt.wantExpired {
				t.Errorf("CheckPauseExpired() expired = %v, want %v", expired, tt.wantExpired)
			}
		})
	}
}

func TestPauseHandler_ClearPause(t *testing.T) {
	cfg := config.NewDefault()
	handler := NewPauseHandler(cfg)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				cfg.Annotations.PausePeriod: "5m",
				cfg.Annotations.PausedAt:    time.Now().UTC().Format(time.RFC3339),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Paused: true,
		},
	}

	handler.ClearPause(deploy)

	if deploy.Spec.Paused {
		t.Error("Expected deployment to be unpaused")
	}

	if _, exists := deploy.Annotations[cfg.Annotations.PausedAt]; exists {
		t.Error("Expected paused-at annotation to be removed")
	}

	// Pause period should be preserved (user's config)
	if deploy.Annotations[cfg.Annotations.PausePeriod] != "5m" {
		t.Error("Expected pause-period annotation to be preserved")
	}
}

func TestPauseHandler_IsPausedByReloader(t *testing.T) {
	cfg := config.NewDefault()
	handler := NewPauseHandler(cfg)

	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   bool
	}{
		{
			name: "paused by reloader",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "5m",
						cfg.Annotations.PausedAt:    time.Now().UTC().Format(time.RFC3339),
					},
				},
				Spec: appsv1.DeploymentSpec{Paused: true},
			},
			want: true,
		},
		{
			name: "paused but not by reloader (no paused-at)",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "5m",
					},
				},
				Spec: appsv1.DeploymentSpec{Paused: true},
			},
			want: false,
		},
		{
			name: "not paused",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: "5m",
						cfg.Annotations.PausedAt:    time.Now().UTC().Format(time.RFC3339),
					},
				},
				Spec: appsv1.DeploymentSpec{Paused: false},
			},
			want: false,
		},
		{
			name: "no annotations",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{Paused: true},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.IsPausedByReloader(tt.deploy)
			if got != tt.want {
				t.Errorf("IsPausedByReloader() = %v, want %v", got, tt.want)
			}
		})
	}
}
