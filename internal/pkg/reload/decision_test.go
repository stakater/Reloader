package reload

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/workload"
)

func TestFilterDecisions(t *testing.T) {
	wl1 := workload.NewDeploymentWorkload(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "deploy1", Namespace: "default"},
		},
	)
	wl2 := workload.NewDeploymentWorkload(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "deploy2", Namespace: "default"},
		},
	)
	wl3 := workload.NewDeploymentWorkload(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "deploy3", Namespace: "default"},
		},
	)

	tests := []struct {
		name      string
		decisions []ReloadDecision
		wantCount int
		wantNames []string
	}{
		{
			name:      "empty list",
			decisions: []ReloadDecision{},
			wantCount: 0,
			wantNames: nil,
		},
		{
			name: "all should reload",
			decisions: []ReloadDecision{
				{Workload: wl1, ShouldReload: true, Reason: "test"},
				{Workload: wl2, ShouldReload: true, Reason: "test"},
			},
			wantCount: 2,
			wantNames: []string{"deploy1", "deploy2"},
		},
		{
			name: "none should reload",
			decisions: []ReloadDecision{
				{Workload: wl1, ShouldReload: false, Reason: "test"},
				{Workload: wl2, ShouldReload: false, Reason: "test"},
			},
			wantCount: 0,
			wantNames: nil,
		},
		{
			name: "mixed - some should reload",
			decisions: []ReloadDecision{
				{Workload: wl1, ShouldReload: true, Reason: "test"},
				{Workload: wl2, ShouldReload: false, Reason: "test"},
				{Workload: wl3, ShouldReload: true, Reason: "test"},
			},
			wantCount: 2,
			wantNames: []string{"deploy1", "deploy3"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result := FilterDecisions(tt.decisions)

				if len(result) != tt.wantCount {
					t.Errorf("FilterDecisions() returned %d decisions, want %d", len(result), tt.wantCount)
				}

				if tt.wantNames != nil {
					for i, d := range result {
						if d.Workload.GetName() != tt.wantNames[i] {
							t.Errorf(
								"FilterDecisions()[%d].Workload.GetName() = %s, want %s",
								i, d.Workload.GetName(), tt.wantNames[i],
							)
						}
					}
				}
			},
		)
	}
}

func TestReloadDecision_Fields(t *testing.T) {
	wl := workload.NewDeploymentWorkload(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		},
	)

	decision := ReloadDecision{
		Workload:     wl,
		ShouldReload: true,
		AutoReload:   true,
		Reason:       "test reason",
		Hash:         "abc123",
	}

	if decision.Workload.GetName() != "test" {
		t.Errorf("ReloadDecision.Workload.GetName() = %v, want test", decision.Workload.GetName())
	}
	if !decision.ShouldReload {
		t.Error("ReloadDecision.ShouldReload should be true")
	}
	if !decision.AutoReload {
		t.Error("ReloadDecision.AutoReload should be true")
	}
	if decision.Reason != "test reason" {
		t.Errorf("ReloadDecision.Reason = %v, want 'test reason'", decision.Reason)
	}
	if decision.Hash != "abc123" {
		t.Errorf("ReloadDecision.Hash = %v, want 'abc123'", decision.Hash)
	}
}
