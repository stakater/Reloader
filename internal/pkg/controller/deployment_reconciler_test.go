package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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

// v2 has no timer map to leak, so a failed resume must simply be retried: the
// reconciler returns the error, controller-runtime requeues, and the next pass
// unpauses. This is the equivalent of master's HandleMissingTimer recovery.
func TestDeploymentReconciler_RetriesAfterFailedUnpause(t *testing.T) {
	cfg := config.NewDefault()

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "test-ns",
			Annotations: map[string]string{
				cfg.Annotations.PausePeriod: "5m",
				cfg.Annotations.PausedAt:    time.Now().Add(-6 * time.Minute).UTC().Format(time.RFC3339),
			},
		},
		Spec: appsv1.DeploymentSpec{Paused: true},
	}

	failNextUpdate := true
	c := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithObjects(deploy).
		WithInterceptorFuncs(
			interceptor.Funcs{
				Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if failNextUpdate {
						failNextUpdate = false
						return apierrors.NewServiceUnavailable("api server is briefly unavailable")
					}
					return cl.Update(ctx, obj, opts...)
				},
			},
		).
		Build()

	r := &controller.DeploymentReconciler{
		Client:       c,
		Log:          testr.New(t),
		Config:       cfg,
		PauseHandler: reload.NewPauseHandler(cfg),
	}

	key := types.NamespacedName{Name: "test-deploy", Namespace: "test-ns"}
	req := ctrl.Request{NamespacedName: key}

	if _, err := r.Reconcile(context.Background(), req); err == nil {
		t.Fatal("first Reconcile() error = nil, want an error so controller-runtime requeues")
	}

	var stillPaused appsv1.Deployment
	if err := c.Get(context.Background(), key, &stillPaused); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !stillPaused.Spec.Paused {
		t.Fatal("deployment was unpaused despite the failed update")
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second Reconcile() error = %v, want nil", err)
	}

	var got appsv1.Deployment
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Spec.Paused {
		t.Error("deployment is still paused after the retry, want unpaused")
	}
}
