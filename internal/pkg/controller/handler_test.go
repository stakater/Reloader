package controller_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/config"
)

// recordingAlerter keeps every message it is asked to send.
type recordingAlerter struct {
	mu       sync.Mutex
	messages []alerting.AlertMessage
}

func (r *recordingAlerter) Send(_ context.Context, message alerting.AlertMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, message)
	return nil
}

// The alert sinks print the timestamp with a literal "UTC" label, so the
// handler has to hand them a UTC time even when the pod runs in another zone.
func TestReloadHandler_AlertTimestampIsUTC(t *testing.T) {
	original := time.Local
	time.Local = time.FixedZone("TEST", 5*60*60)
	t.Cleanup(func() { time.Local = original })

	cfg := config.NewDefault()
	cm := testutil.NewConfigMap("test-cm", "default")
	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	deps := newTestDeps(t, cfg, cm, deployment)
	alerter := &recordingAlerter{}

	reconciler := controller.NewConfigMapReconciler(
		deps.client.Build(),
		deps.log,
		deps.cfg,
		deps.reloadService,
		deps.registry,
		deps.collectors,
		deps.eventRecorder,
		deps.webhookClient,
		alerter,
		nil,
		nil,
	)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))

	if len(alerter.messages) != 1 {
		t.Fatalf("got %d alerts, want 1", len(alerter.messages))
	}
	if got := alerter.messages[0].Timestamp.Location(); got != time.UTC {
		t.Errorf("alert timestamp location = %v, want UTC", got)
	}
}
