package controller_test

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/config"
)

// newSecretProviderClassReconcilerWithClient creates a SecretProviderClassReconciler for
// testing and returns both the reconciler and the fake client (for assertions).
func newSecretProviderClassReconcilerWithClient(t *testing.T, cfg *config.Config, objects ...client.Object) (*controller.SecretProviderClassReconciler, client.Client) {
	t.Helper()
	// Convert client.Object slice to runtime.Object slice for newTestDeps.
	// We create the fake client ourselves to hold the reference.
	deps := newTestDeps(t, cfg)
	for _, obj := range objects {
		deps.client = deps.client.WithObjects(obj)
	}
	cl := deps.client.Build()
	reconciler := controller.NewSecretProviderClassReconciler(
		controller.ResourceReconcilerDeps{
			Client:         cl,
			Log:            deps.log,
			Config:         deps.cfg,
			ReloadService:  deps.reloadService,
			Registry:       deps.registry,
			Collectors:     deps.collectors,
			EventRecorder:  deps.eventRecorder,
			WebhookClient:  deps.webhookClient,
			Alerter:        deps.alerter,
			PauseHandler:   reload.NewPauseHandler(cfg),
			NamespaceCache: nil,
		},
		cl, // APIReader = same fake client in tests
	)
	return reconciler, cl
}

// TestSecretProviderClassReconciler_NotFound ensures that reconciling a
// nonexistent SecretProviderClassPodStatus returns cleanly without error.
func TestSecretProviderClassReconciler_NotFound(t *testing.T) {
	cfg := config.NewDefault()
	reconciler, _ := newSecretProviderClassReconcilerWithClient(t, cfg)
	assertReconcileSuccess(t, reconciler, reconcileRequest("nonexistent", "default"))
}

// TestSecretProviderClassReconciler_MatchingDeployment_AutoAnnotation tests the
// core happy path: a SPCPS update resolves the SPC, creates a change event, and
// the deployment gets the STAKATER_MY_SPC_SECRETPROVIDERCLASS env var.
func TestSecretProviderClassReconciler_MatchingDeployment_AutoAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.SecretProviderClassAuto: "true",
	})

	spc := &csiv1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spc",
			Namespace: "default",
		},
	}

	spcps := &csiv1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-spcps",
			Namespace: "default",
		},
		Status: csiv1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "my-spc",
			Objects: []csiv1.SecretProviderClassObject{
				{ID: "a", Version: "1"},
			},
		},
	}

	reconciler, cl := newSecretProviderClassReconcilerWithClient(t, cfg, deployment, spc, spcps)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-pod-spcps", "default"))

	// Verify the deployment was updated with the expected env var.
	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "test-deployment", Namespace: "default"}, updated); err != nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}

	if len(updated.Spec.Template.Spec.Containers) == 0 {
		t.Fatal("deployment has no containers")
	}

	const expectedEnvVar = "STAKATER_MY_SPC_SECRETPROVIDERCLASS"
	found := false
	for _, env := range updated.Spec.Template.Spec.Containers[0].Env {
		if env.Name == expectedEnvVar && env.Value != "" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected env var %q to be set on deployment container; got envs: %v",
			expectedEnvVar, updated.Spec.Template.Spec.Containers[0].Env)
	}
}

// TestSecretProviderClassReconciler_SPCNotFound ensures that if the SPCPS
// references a SecretProviderClass that does not exist (e.g. it was deleted
// while secrets are still rotating), the reconciler still reloads any workload
// annotated with secretproviderclass auto/named annotations. The SPC object's
// annotations are only needed for ignore-check and search/match; auto and
// named-reload matching is driven by the workload's own annotations alone.
func TestSecretProviderClassReconciler_SPCNotFound(t *testing.T) {
	cfg := config.NewDefault()

	// A deployment annotated for auto-reload on any SPC change.
	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.SecretProviderClassAuto: "true",
	})

	// Only add the SPCPS; the SPC ("missing-spc") is intentionally absent from
	// the fake client to simulate deletion.
	spcps := &csiv1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orphaned-spcps",
			Namespace: "default",
		},
		Status: csiv1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "missing-spc",
			Objects: []csiv1.SecretProviderClassObject{
				{ID: "b", Version: "2"},
			},
		},
	}

	reconciler, cl := newSecretProviderClassReconcilerWithClient(t, cfg, deployment, spcps)

	// Reconcile must succeed with no error and no requeue.
	assertReconcileSuccess(t, reconciler, reconcileRequest("orphaned-spcps", "default"))

	// The deployment must have been reloaded: container[0].Env must contain
	// STAKATER_MISSING_SPC_SECRETPROVIDERCLASS with a non-empty SHA value.
	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "test-deployment", Namespace: "default"}, updated); err != nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}

	if len(updated.Spec.Template.Spec.Containers) == 0 {
		t.Fatal("deployment has no containers")
	}

	const expectedEnvVar = "STAKATER_MISSING_SPC_SECRETPROVIDERCLASS"
	found := false
	for _, env := range updated.Spec.Template.Spec.Containers[0].Env {
		if env.Name == expectedEnvVar && env.Value != "" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected env var %q to be set on deployment container after SPC-not-found reconcile; got envs: %v",
			expectedEnvVar, updated.Spec.Template.Spec.Containers[0].Env)
	}
}

func TestSecretProviderClassReconciler_EmptySPCName(t *testing.T) {
	cfg := config.NewDefault()
	// SPCPS whose Status carries no SecretProviderClassName must be skipped cleanly.
	spcps := &csiv1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "orphan-spcps", Namespace: "default"},
		Status:     csiv1.SecretProviderClassPodStatusStatus{},
	}
	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.SecretProviderClassAuto: "true",
	})
	reconciler, cl := newSecretProviderClassReconcilerWithClient(t, cfg, deployment, spcps)

	if _, err := reconciler.Reconcile(context.Background(), reconcileRequest("orphan-spcps", "default")); err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	// No reload should have happened (no SPC name to match against).
	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "test-deployment"}, updated); err != nil {
		t.Fatal(err)
	}
	for _, c := range updated.Spec.Template.Spec.Containers {
		if len(c.Env) != 0 {
			t.Errorf("expected no env vars injected for empty SPC name, got %+v", c.Env)
		}
	}
}
