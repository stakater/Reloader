package controller_test

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/config"
)

// newPauseTestReconciler returns a ConfigMap reconciler together with the client backing it,
// so tests can inspect workloads after reconciling.
func newPauseTestReconciler(t *testing.T, cfg *config.Config, objects ...runtime.Object) (*controller.ConfigMapReconciler, client.Client) {
	t.Helper()
	deps := newTestDeps(t, cfg, objects...)
	c := deps.client.Build()
	r := controller.NewConfigMapReconciler(
		c, deps.log, deps.cfg, deps.reloadService, deps.registry, deps.collectors,
		deps.eventRecorder, deps.webhookClient, deps.alerter, nil, nil,
	)
	return r, c
}

func pauseTestConfigMap(annotations map[string]string) *corev1.ConfigMap {
	cm := testutil.NewConfigMapWithAnnotations("test-cm", "default", annotations)
	cm.Data = map[string]string{"key": "value"}
	return cm
}

func pauseTestNamespace(paused string) *corev1.Namespace {
	ns := testutil.NewNamespace("default", nil)
	if paused != "" {
		ns.Annotations = map[string]string{config.DefaultAnnotations().Pause: paused}
	}
	return ns
}

func pauseTestDeployment(cfg *config.Config) *appsv1.Deployment {
	return testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
}

func hasReloaderEnvVar(t *testing.T, c client.Client) bool {
	t.Helper()
	var d appsv1.Deployment
	if err := c.Get(context.Background(), types.NamespacedName{Name: "test-deployment", Namespace: "default"}, &d); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	for _, env := range d.Spec.Template.Spec.Containers[0].Env {
		if strings.HasPrefix(env.Name, "STAKATER_") {
			return true
		}
	}
	return false
}

func TestResourceReconciler_PausedNamespace_HoldsAndAppliesAfterResume(t *testing.T) {
	cfg := config.NewDefault()
	ns := pauseTestNamespace("true")
	r, c := newPauseTestReconciler(t, cfg, ns, pauseTestConfigMap(nil), pauseTestDeployment(cfg))
	req := reconcileRequest("test-cm", "default")

	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.RequeueAfter != controller.PausedRequeueInterval {
		t.Errorf("RequeueAfter = %v, want %v", result.RequeueAfter, controller.PausedRequeueInterval)
	}
	if hasReloaderEnvVar(t, c) {
		t.Fatal("deployment was reloaded while the namespace was paused")
	}

	// Lifting the pause lets the requeued reconcile apply the held change.
	var current corev1.Namespace
	if err := c.Get(context.Background(), client.ObjectKey{Name: "default"}, &current); err != nil {
		t.Fatalf("get namespace: %v", err)
	}
	delete(current.Annotations, cfg.Annotations.Pause)
	if err := c.Update(context.Background(), &current); err != nil {
		t.Fatalf("update namespace: %v", err)
	}

	assertReconcileSuccess(t, r, req)
	if !hasReloaderEnvVar(t, c) {
		t.Error("deployment was not reloaded after the pause was lifted")
	}
}

func TestResourceReconciler_PausedResource(t *testing.T) {
	cfg := config.NewDefault()
	cm := pauseTestConfigMap(map[string]string{cfg.Annotations.Pause: "true"})
	r, c := newPauseTestReconciler(t, cfg, pauseTestNamespace(""), cm, pauseTestDeployment(cfg))

	result, err := r.Reconcile(context.Background(), reconcileRequest("test-cm", "default"))
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.RequeueAfter != controller.PausedRequeueInterval {
		t.Errorf("RequeueAfter = %v, want %v", result.RequeueAfter, controller.PausedRequeueInterval)
	}
	if hasReloaderEnvVar(t, c) {
		t.Error("deployment was reloaded while the configmap was paused")
	}
}

func TestResourceReconciler_PauseNotTrue_Reloads(t *testing.T) {
	cfg := config.NewDefault()
	cm := pauseTestConfigMap(map[string]string{cfg.Annotations.Pause: "false"})
	r, c := newPauseTestReconciler(t, cfg, pauseTestNamespace("false"), cm, pauseTestDeployment(cfg))

	assertReconcileSuccess(t, r, reconcileRequest("test-cm", "default"))
	if !hasReloaderEnvVar(t, c) {
		t.Error("deployment was not reloaded")
	}
}

func TestResourceReconciler_CustomPauseAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Annotations.Pause = "example.com/hold"
	ns := testutil.NewNamespace("default", nil)
	ns.Annotations = map[string]string{"example.com/hold": "true"}
	r, c := newPauseTestReconciler(t, cfg, ns, pauseTestConfigMap(nil), pauseTestDeployment(cfg))

	result, err := r.Reconcile(context.Background(), reconcileRequest("test-cm", "default"))
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.RequeueAfter != controller.PausedRequeueInterval {
		t.Errorf("RequeueAfter = %v, want %v", result.RequeueAfter, controller.PausedRequeueInterval)
	}
	if hasReloaderEnvVar(t, c) {
		t.Error("deployment was reloaded while the namespace was paused")
	}
}

func TestResourceReconciler_PausedNamespace_HoldsDelete(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnDelete = true
	r, _ := newPauseTestReconciler(t, cfg, pauseTestNamespace("true"), pauseTestDeployment(cfg))

	result, err := r.Reconcile(context.Background(), reconcileRequest("test-cm", "default"))
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.RequeueAfter != controller.PausedRequeueInterval {
		t.Errorf("RequeueAfter = %v, want %v", result.RequeueAfter, controller.PausedRequeueInterval)
	}
}

func namespaceGetError(err error) interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Namespace); ok {
				return err
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}
}

func TestResourceReconciler_NamespaceForbidden_Reloads(t *testing.T) {
	cfg := config.NewDefault()
	r, c := newPauseTestReconciler(t, cfg, pauseTestNamespace("true"), pauseTestConfigMap(nil), pauseTestDeployment(cfg))
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "default", nil)
	r.APIReader = newTestDeps(t, cfg, pauseTestNamespace("true")).client.WithInterceptorFuncs(namespaceGetError(forbidden)).Build()

	assertReconcileSuccess(t, r, reconcileRequest("test-cm", "default"))
	if !hasReloaderEnvVar(t, c) {
		t.Error("deployment was not reloaded when namespaces cannot be read")
	}
}

func TestResourceReconciler_NamespaceGetError_Retries(t *testing.T) {
	cfg := config.NewDefault()
	r, c := newPauseTestReconciler(t, cfg, pauseTestConfigMap(nil), pauseTestDeployment(cfg))
	r.APIReader = newTestDeps(t, cfg).client.WithInterceptorFuncs(namespaceGetError(apierrors.NewServiceUnavailable("down"))).Build()

	if _, err := r.Reconcile(context.Background(), reconcileRequest("test-cm", "default")); err == nil {
		t.Error("expected the namespace lookup error to be returned")
	}
	if hasReloaderEnvVar(t, c) {
		t.Error("deployment was reloaded although the pause state was unknown")
	}
}
