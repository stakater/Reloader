package controller

import (
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
)

// TestConfigMapReconciler_SetupWatchesWorkloads checks that the resource controller can be
// built with its workload watches, which wake it for deferred reloads.
func TestConfigMapReconciler_SetupWatchesWorkloads(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredWorkloads = []string{"Job"}

	mgr, err := ctrl.NewManager(&rest.Config{Host: "https://127.0.0.1:1"}, ctrl.Options{
		Scheme:  testutil.NewScheme(),
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	collectors := metrics.NewCollectors()
	r := NewConfigMapReconciler(
		mgr.GetClient(), logr.Discard(), cfg, reload.NewService(cfg, logr.Discard()),
		workload.NewRegistry(workload.RegistryOptions{}), &collectors, nil, nil, nil, nil, nil,
	)
	if err := SetupConfigMapReconciler(mgr, r); err != nil {
		t.Fatalf("SetupConfigMapReconciler() error = %v", err)
	}
}
