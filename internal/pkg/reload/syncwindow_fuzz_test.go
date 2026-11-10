package reload

import (
	"testing"
	"time"

	"github.com/go-logr/logr"

	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
	"github.com/stakater/Reloader/pkg/matcher"
)

// FuzzSyncWindowDeferral feeds arbitrary sync-windows and pending-reload annotations through
// the deferral logic. Whatever users write must neither panic nor take long, and a deferral
// must never be due in the past.
func FuzzSyncWindowDeferral(f *testing.F) {
	for _, seed := range []struct{ windows, pending string }{
		{`[{"kind":"allow","schedule":"0 2 * * *","duration":"1h","timeZone":"Europe/Berlin"}]`, ``},
		{`- kind: deny
  schedule: "0 22 * * *"
  duration: 2h
  spread: 30m`, `[{"kind":"configmap","name":"test-cm","hash":"h","notBefore":"2026-11-10T10:05:00Z"}]`},
		{`[{"kind":"deny","schedule":"* * * * *","duration":"2562047h"}]`, ``},
		{`[{"kind":"allow","schedule":"0 0 29 2 1","duration":"1m"}]`, ``},
		{`[{"kind":"deny","schedule":"@every 1s","duration":"8760h"}]`, ``},
		{`[{"kind":"allow","schedule":"@yearly","duration":"1s","spread":"2562047h"}]`, ``},
		{`[{"kind":"deny","schedule":"0 * * * *","duration":"90m","timeZone":"../../etc/passwd"}]`, `not json`},
		{`&a [*a, *a, *a]`, `[{"kind":"configmap","name":"test-cm","hash":"h","notBefore":"9999-12-31T23:59:59Z"}]`},
		{`null`, `[]`},
		{``, ``},
	} {
		f.Add(seed.windows, seed.pending)
	}

	cfg := config.NewDefault()
	now := time.Date(2026, time.November, 10, 10, 0, 0, 0, time.UTC)
	svc := NewService(cfg, logr.Discard())
	svc.now = func() time.Time { return now }

	f.Fuzz(func(t *testing.T, windows, pending string) {
		deploy := testutil.NewDeployment("test-deploy", "default", map[string]string{
			cfg.Annotations.SyncWindows:   windows,
			cfg.Annotations.PendingReload: pending,
		})
		wl := workload.NewDeploymentWorkload(deploy)

		start := time.Now()
		due, _ := svc.syncWindowDeferral(wl, matcher.ResourceTypeConfigMap, "test-cm", "h")
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("evaluating %q took %s", windows, elapsed)
		}
		if !due.IsZero() && !due.After(now) {
			t.Errorf("deferral for %q is due at %s, not after now (%s)", windows, due, now)
		}

		obj := deploy.DeepCopy()
		SetPendingReload(obj, cfg.Annotations.PendingReload, PendingReload{Kind: matcher.ResourceTypeConfigMap, Name: "test-cm", Hash: "h", NotBefore: now})
		ClearPendingReload(obj, cfg.Annotations.PendingReload, matcher.ResourceTypeConfigMap, "test-cm")
	})
}
