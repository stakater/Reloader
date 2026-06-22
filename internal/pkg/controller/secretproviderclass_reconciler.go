package controller

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/reload"
)

// SecretProviderClassReconciler watches SecretProviderClassPodStatus (the per-pod
// status the CSI driver rewrites on rotation) and reloads matching workloads,
// reusing the generic reconciler via a ResolveChange hook.
type SecretProviderClassReconciler = ResourceReconciler[*csiv1.SecretProviderClassPodStatus]

// NewSecretProviderClassReconciler builds the reconciler. apiReader (non-cached)
// looks up the parent SecretProviderClass without starting a second informer.
func NewSecretProviderClassReconciler(deps ResourceReconcilerDeps, apiReader client.Reader) *SecretProviderClassReconciler {
	deps.APIReader = apiReader
	return NewResourceReconciler(
		deps,
		ResourceConfig[*csiv1.SecretProviderClassPodStatus]{
			ResourceType:   reload.ResourceTypeSecretProviderClass,
			NewResource:    func() *csiv1.SecretProviderClassPodStatus { return &csiv1.SecretProviderClassPodStatus{} },
			ResolveChange:  resolveSecretProviderClassChange,
			SkipOnNotFound: true,
			BuildFilter:    secretProviderClassFilter,
		},
	)
}

// resolveSecretProviderClassChange builds the change from an SPCPS: it reads the
// SPC name from the status and looks up the SPC for its annotations. On any lookup
// error it proceeds with empty annotations so annotation-matched workloads still
// reload (master parity); an empty SPC name skips the event.
func resolveSecretProviderClassChange(
	ctx context.Context,
	reader client.Reader,
	log logr.Logger,
	spcps *csiv1.SecretProviderClassPodStatus,
) (reload.ResourceChange, bool) {
	spcName := spcps.Status.SecretProviderClassName
	if spcName == "" {
		return nil, false
	}

	annotations := map[string]string{}
	spc := &csiv1.SecretProviderClass{}
	if err := reader.Get(ctx, types.NamespacedName{Name: spcName, Namespace: spcps.GetNamespace()}, spc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("SecretProviderClass not found; proceeding without its annotations", "spc", spcName)
		} else {
			log.V(1).Error(err, "failed to get SecretProviderClass; proceeding without its annotations", "spc", spcName)
		}
	} else if a := spc.GetAnnotations(); a != nil {
		annotations = a
	}

	return reload.SecretProviderClassChange{
		Name:        spcName,
		Namespace:   spcps.GetNamespace(),
		Annotations: annotations,
		Status:      spcps.Status,
		EventType:   reload.EventTypeUpdate,
	}, true
}

// secretProviderClassFilter omits the label selector (driver-owned SPCPS can't
// carry user labels, so applying it would silently disable CSI reloads) and the
// namespace cache (checked in Reconcile to avoid a startup race).
func secretProviderClassFilter(cfg *config.Config, hasher *reload.Hasher) predicate.Predicate {
	return reload.CombinedPredicates(
		reload.NamespaceFilterPredicateWithCache(cfg, nil),
		reload.SecretProviderClassPodStatusPredicates(cfg, hasher),
	)
}

// SetupSecretProviderClassReconciler sets up the reconciler with the manager.
func SetupSecretProviderClassReconciler(mgr ctrl.Manager, r *SecretProviderClassReconciler) error {
	return r.SetupWithManager(mgr, &csiv1.SecretProviderClassPodStatus{})
}

var _ reconcile.Reconciler = &SecretProviderClassReconciler{}
