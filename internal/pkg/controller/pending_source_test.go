package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/config"
)

// missingDaemonSets fails to provide a DaemonSet informer, as when a kind's CRD or RBAC is
// missing.
type missingDaemonSets struct {
	*informertest.FakeInformers
}

func (m missingDaemonSets) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	if _, ok := obj.(*appsv1.DaemonSet); ok {
		return nil, errors.New("no matches for kind DaemonSet")
	}
	return m.FakeInformers.GetInformer(ctx, obj, opts...)
}

func TestPendingReloadSource(t *testing.T) {
	cfg := config.NewDefault()
	fakes := &informertest.FakeInformers{Scheme: testutil.NewScheme()}
	events := make(chan event.GenericEvent, 10)
	src := &pendingReloadSource{
		informers: missingDaemonSets{fakes},
		objects:   []client.Object{&appsv1.DaemonSet{}, &appsv1.Deployment{}},
		predicate: reload.PendingReloadPredicate(cfg),
		events:    events,
		log:       logr.Discard(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- src.Start(ctx) }()

	deployments, err := fakes.FakeInformerFor(ctx, &appsv1.Deployment{})
	if err != nil {
		t.Fatalf("FakeInformerFor() error = %v", err)
	}
	// Start registers its handler asynchronously, so the adds are repeated until it does.
	deadline := time.Now().Add(5 * time.Second)

	waiting := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: "waiting", Namespace: "default",
		Annotations: map[string]string{cfg.Annotations.PendingReload: `[{"kind":"configmap","name":"app-cm"}]`},
	}}
	idle := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "idle", Namespace: "default"}}

	var got event.GenericEvent
	for got.Object == nil && time.Now().Before(deadline) {
		deployments.Add(idle)
		deployments.Add(waiting)
		select {
		case got = <-events:
		case <-time.After(50 * time.Millisecond):
		}
	}
	if got.Object == nil || got.Object.GetName() != "waiting" {
		t.Fatalf("event = %v, want the Deployment waiting on a deferred reload", got.Object)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start() error = %v, want nil although one kind could not be watched", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start() did not return after its context was cancelled")
	}
}
