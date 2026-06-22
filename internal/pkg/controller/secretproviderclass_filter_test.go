package controller

import (
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/event"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/reload"
)

// TestSecretProviderClassReconciler_FilterIgnoresResourceLabelSelector pins the
// deliberate behavior that --resource-label-selector does NOT filter
// SecretProviderClassPodStatus events (they are CSI-driver-owned and cannot carry
// user labels). If the filter ever regressed to BuildEventFilter (which applies
// LabelSelectorPredicate), the changed-status event below would be dropped.
func TestSecretProviderClassReconciler_FilterIgnoresResourceLabelSelector(t *testing.T) {
	cfg := config.NewDefault()
	sel, err := labels.Parse("reloader=enabled")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ResourceSelectors = []labels.Selector{sel}

	r := NewSecretProviderClassReconciler(
		ResourceReconcilerDeps{
			Config:        cfg,
			Log:           logr.Discard(),
			ReloadService: reload.NewService(cfg, logr.Discard()),
		},
		nil,
	)

	// SPCPS with a changed status and NO matching label.
	oldObj := &csiv1.SecretProviderClassPodStatus{
		Status: csiv1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "spc",
			Objects:                 []csiv1.SecretProviderClassObject{{ID: "a", Version: "1"}},
		},
	}
	newObj := &csiv1.SecretProviderClassPodStatus{
		Status: csiv1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "spc",
			Objects:                 []csiv1.SecretProviderClassObject{{ID: "a", Version: "2"}},
		},
	}

	filter := r.BuildFilter(r.Config, r.ReloadService.Hasher())
	if !filter.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
		t.Fatal("SPCPS status change must pass the filter even when --resource-label-selector is set")
	}
}
