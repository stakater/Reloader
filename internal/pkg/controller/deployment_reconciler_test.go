package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/config"
)

// A pause period Reloader cannot use must still unpause the deployment. Requeueing the
// error instead would leave spec.paused set with no expiry left to reach.
func TestDeploymentReconciler_UnpausesOnUnusablePausePeriod(t *testing.T) {
	cfg := config.NewDefault()

	for _, pausePeriod := range []string{"0s", "-5m", "invalid"} {
		t.Run(pausePeriod, func(t *testing.T) {
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
					Namespace: "test-ns",
					Annotations: map[string]string{
						cfg.Annotations.PausePeriod: pausePeriod,
						cfg.Annotations.PausedAt:    time.Now().UTC().Format(time.RFC3339),
					},
				},
				Spec: appsv1.DeploymentSpec{Paused: true},
			}

			c := fake.NewClientBuilder().
				WithScheme(testutil.NewScheme()).
				WithObjects(deploy).
				Build()

			r := &controller.DeploymentReconciler{
				Client:       c,
				Log:          testr.New(t),
				Config:       cfg,
				PauseHandler: reload.NewPauseHandler(cfg),
			}

			key := types.NamespacedName{Name: "test-deploy", Namespace: "test-ns"}
			result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
			if err != nil {
				t.Fatalf("Reconcile() error = %v, want nil", err)
			}
			if result.RequeueAfter != 0 {
				t.Errorf("Reconcile() RequeueAfter = %v, want 0", result.RequeueAfter)
			}

			var got appsv1.Deployment
			if err := c.Get(context.Background(), key, &got); err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if got.Spec.Paused {
				t.Error("deployment is still paused, want unpaused")
			}
			if _, exists := got.Annotations[cfg.Annotations.PausedAt]; exists {
				t.Error("paused-at annotation is still set, want removed")
			}
		})
	}
}
