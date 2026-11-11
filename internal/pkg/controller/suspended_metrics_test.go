package controller_test

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/config"
)

// A change that matches only suspended CronJobs is skipped once, as "suspended", however many
// CronJobs there are, and is not also counted as "no_match". A change that matches a running
// workload as well is not skipped at all.
func TestConfigMapReconciler_SuspendedSkipCountedOncePerEvent(t *testing.T) {
	suspendedCronJob := func(cfg *config.Config, name string) runtime.Object {
		cj := testutil.NewCronJobWithAnnotations(name, "default", map[string]string{cfg.Annotations.ConfigmapReload: "test-cm"})
		suspend := true
		cj.Spec.Suspend = &suspend
		return cj
	}
	for name, c := range map[string]struct {
		extra                      func(*config.Config) []runtime.Object
		wantSuspended, wantNoMatch float64
	}{
		"three suspended CronJobs": {func(cfg *config.Config) []runtime.Object {
			return []runtime.Object{suspendedCronJob(cfg, "a"), suspendedCronJob(cfg, "b"), suspendedCronJob(cfg, "c")}
		}, 1, 0},
		"a suspended CronJob next to a Deployment": {func(cfg *config.Config) []runtime.Object {
			return []runtime.Object{suspendedCronJob(cfg, "a"),
				testutil.NewDeployment("web", "default", map[string]string{cfg.Annotations.ConfigmapReload: "test-cm"})}
		}, 0, 0},
		"nothing matches": {func(cfg *config.Config) []runtime.Object { return nil }, 0, 1},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := config.NewDefault()
			objects := append([]runtime.Object{testutil.NewConfigMap("test-cm", "default")}, c.extra(cfg)...)
			deps := newTestDeps(t, cfg, objects...)
			reconciler := controller.NewConfigMapReconciler(deps.client.Build(), deps.log, deps.cfg, deps.reloadService,
				deps.registry, deps.collectors, deps.eventRecorder, deps.webhookClient, deps.alerter, nil, nil)
			assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))

			skipped := func(reason string) float64 {
				m := &dto.Metric{}
				if err := deps.collectors.SkippedTotal.WithLabelValues(reason).Write(m); err != nil {
					t.Fatal(err)
				}
				return m.GetCounter().GetValue()
			}
			if got := skipped("suspended"); got != c.wantSuspended {
				t.Errorf("skipped_total{reason=suspended} = %v, want %v", got, c.wantSuspended)
			}
			if got := skipped("no_match"); got != c.wantNoMatch {
				t.Errorf("skipped_total{reason=no_match} = %v, want %v", got, c.wantNoMatch)
			}
		})
	}
}
