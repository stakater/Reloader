package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
)

// SecretProviderClassReconciler watches SecretProviderClassPodStatus objects and
// triggers workload reloads when the secret versions they track change.
//
// It watches SecretProviderClassPodStatus (the per-pod status written by the CSI
// driver) rather than SecretProviderClass directly, because only the pod status
// carries the current object IDs and versions that indicate a secret rotation.
type SecretProviderClassReconciler struct {
	ResourceReconcilerDeps

	// apiReader is a direct API client (not cached) used to look up the parent
	// SecretProviderClass object. In tests this is set to the fake client.
	apiReader client.Reader

	handler *ReloadHandler
}

// NewSecretProviderClassReconciler creates a new SecretProviderClassReconciler.
func NewSecretProviderClassReconciler(deps ResourceReconcilerDeps, apiReader client.Reader) *SecretProviderClassReconciler {
	return &SecretProviderClassReconciler{
		ResourceReconcilerDeps: deps,
		apiReader:              apiReader,
	}
}

// Reconcile handles a SecretProviderClassPodStatus event.
func (r *SecretProviderClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	resourceType := string(reload.ResourceTypeSecretProviderClass)
	log := r.Log.WithValues("secretproviderclasspodstatus", req.NamespacedName)

	r.Collectors.RecordEventReceived("reconcile", resourceType)

	spcps := &csiv1.SecretProviderClassPodStatus{}
	if err := r.Client.Get(ctx, req.NamespacedName, spcps); err != nil {
		if errors.IsNotFound(err) {
			r.Collectors.RecordSkipped("not_found")
			r.Collectors.RecordReconcile("success", time.Since(startTime))
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get SecretProviderClassPodStatus")
		r.Collectors.RecordError("get_secretproviderclasspodstatus")
		r.Collectors.RecordReconcile("error", time.Since(startTime))
		return ctrl.Result{}, err
	}

	namespace := spcps.GetNamespace()
	if r.Config.IsNamespaceIgnored(namespace) {
		log.V(1).Info("skipping SecretProviderClassPodStatus in ignored namespace")
		r.Collectors.RecordSkipped("ignored_namespace")
		r.Collectors.RecordReconcile("success", time.Since(startTime))
		return ctrl.Result{}, nil
	}

	if r.NamespaceCache != nil && r.NamespaceCache.IsEnabled() && !r.NamespaceCache.Contains(namespace) {
		log.V(1).Info("skipping SecretProviderClassPodStatus in namespace not matching selector", "namespace", namespace)
		r.Collectors.RecordSkipped("namespace_selector")
		r.Collectors.RecordReconcile("success", time.Since(startTime))
		return ctrl.Result{}, nil
	}

	spcName, spcAnnotations := r.resolveSPCAnnotations(ctx, spcps)
	if spcName == "" {
		r.Collectors.RecordSkipped("no_spc_name")
		r.Collectors.RecordReconcile("success", time.Since(startTime))
		return ctrl.Result{}, nil
	}

	change := reload.SecretProviderClassChange{
		Name:        spcName,
		Namespace:   namespace,
		Annotations: spcAnnotations,
		Status:      spcps.Status,
		EventType:   reload.EventTypeUpdate,
	}

	result, err := r.reloadHandler().Process(
		ctx, namespace, spcName, reload.ResourceTypeSecretProviderClass,
		func(workloads []workload.Workload) []reload.ReloadDecision {
			return r.ReloadService.Process(change, workloads)
		}, log,
	)

	if err != nil {
		r.Collectors.RecordReconcile("error", time.Since(startTime))
	} else {
		r.Collectors.RecordReconcile("success", time.Since(startTime))
	}
	return result, err
}

// resolveSPCAnnotations looks up the SecretProviderClass referenced by the
// given pod status and returns its name and annotations. It never returns an
// error: on any Get failure it logs and returns the SPC name (from the pod
// status) with an empty annotations map, so callers can still process workloads
// that match via their own auto/named annotations. This matches master's
// behaviour in populateAnnotationsFromSecretProviderClass.
func (r *SecretProviderClassReconciler) resolveSPCAnnotations(
	ctx context.Context,
	spcps *csiv1.SecretProviderClassPodStatus,
) (string, map[string]string) {
	spcName := spcps.Status.SecretProviderClassName
	spc := &csiv1.SecretProviderClass{}
	if err := r.apiReader.Get(ctx, types.NamespacedName{
		Name:      spcName,
		Namespace: spcps.GetNamespace(),
	}, spc); err != nil {
		if errors.IsNotFound(err) {
			r.Log.WithValues("spc", spcName).Info("SecretProviderClass not found; proceeding without its annotations")
		} else {
			r.Log.V(1).Error(err, "failed to get SecretProviderClass; proceeding without its annotations", "spc", spcName)
		}
		return spcName, map[string]string{}
	}
	annotations := spc.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	return spc.Name, annotations
}

func (r *SecretProviderClassReconciler) reloadHandler() *ReloadHandler {
	if r.handler == nil {
		r.handler = &ReloadHandler{
			Client:        r.Client,
			Lister:        workload.NewLister(r.Client, r.Registry, r.Config),
			ReloadService: r.ReloadService,
			WebhookClient: r.WebhookClient,
			Collectors:    r.Collectors,
			EventRecorder: r.EventRecorder,
			Alerter:       r.Alerter,
			PauseHandler:  r.PauseHandler,
		}
	}
	return r.handler
}

// SetupWithManager wires the reconciler to watch SecretProviderClassPodStatus.
func (r *SecretProviderClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var nsChecker reload.NamespaceChecker
	if r.NamespaceCache != nil {
		nsChecker = r.NamespaceCache
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1.SecretProviderClassPodStatus{}).
		WithEventFilter(reload.CombinedPredicates(
			reload.NamespaceFilterPredicateWithCache(r.Config, nsChecker),
			reload.SecretProviderClassPodStatusPredicates(r.Config, r.ReloadService.Hasher()),
		)).
		Complete(r)
}

var _ reconcile.Reconciler = &SecretProviderClassReconciler{}
