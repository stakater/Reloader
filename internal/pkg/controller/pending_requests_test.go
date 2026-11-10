package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/config"
)

func workloadWaitingOn(cfg *config.Config, pending string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:        "app",
		Namespace:   "team-a",
		Annotations: map[string]string{cfg.Annotations.PendingReload: pending},
	}}
}

func TestPendingReloadRequests_MapsEntriesOfTheReconcilerType(t *testing.T) {
	cfg := config.NewDefault()
	r := NewConfigMapReconciler(nil, logr.Discard(), cfg, reload.NewService(cfg, logr.Discard()), nil, nil, nil, nil, nil, nil, nil)

	obj := workloadWaitingOn(cfg, `[
		{"kind":"configmap","name":"app-cm"},
		{"kind":"secret","name":"app-secret"},
		{"kind":"configmap","name":"shared-cm"}]`)

	got := r.pendingReloadRequests(context.Background(), obj)
	want := []reconcile.Request{
		{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "app-cm"}},
		{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "shared-cm"}},
	}
	if len(got) != len(want) {
		t.Fatalf("pendingReloadRequests() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPendingReloadRequests_SecretProviderClassUsesAPodStatus(t *testing.T) {
	cfg := config.NewDefault()
	spcps := func(name, spc string) *csiv1.SecretProviderClassPodStatus {
		return &csiv1.SecretProviderClassPodStatus{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "team-a"},
			Status:     csiv1.SecretProviderClassPodStatusStatus{SecretProviderClassName: spc},
		}
	}
	c := fake.NewClientBuilder().WithScheme(testutil.NewScheme()).WithObjects(
		spcps("pod-b-vault", "vault"),
		spcps("pod-a-vault", "vault"),
		spcps("pod-c-other", "other"),
	).Build()

	r := NewSecretProviderClassReconciler(
		ResourceReconcilerDeps{Client: c, Config: cfg, Log: logr.Discard(), ReloadService: reload.NewService(cfg, logr.Discard())},
		nil,
	)

	got := r.pendingReloadRequests(context.Background(), workloadWaitingOn(cfg, `[
		{"kind":"secretproviderclass","name":"vault"},
		{"kind":"secretproviderclass","name":"missing"}]`))
	want := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "pod-a-vault"}}
	if len(got) != 1 || got[0] != want {
		t.Errorf("pendingReloadRequests() = %v, want [%v]", got, want)
	}
}
