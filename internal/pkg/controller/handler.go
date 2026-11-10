package controller

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
	"github.com/stakater/Reloader/pkg/matcher"
)

// ReloadHandler handles the common reload workflow.
type ReloadHandler struct {
	Client        client.Client
	Lister        *workload.Lister
	ReloadService *reload.Service
	WebhookClient *webhook.Client
	Collectors    *metrics.Collectors
	EventRecorder *events.Recorder
	Alerter       alerting.Alerter
	PauseHandler  *reload.PauseHandler

	// Config supplies the pending-reload annotation key.
	Config *config.Config

	// handled remembers, per resource, the content this process last handled while sync
	// windows were involved. Predicates
	// only pass real changes, so another reconcile for the same content is a repeat: a
	// leftover requeue timer (for example after a deferred reload already ran early because
	// its sync windows were removed) or an error retry. A repeat only handles the workloads
	// still owed a reload, never all of them again, which for a CronJob would start another
	// Job. What is owed lives in the pending-reload annotations; this only adds the deferred
	// workloads whose annotation could not be written, until a retry writes it.
	handledMu sync.Mutex
	handled   map[types.NamespacedName]handledChange
}

// handledChange is the content last handled for a resource.
type handledChange struct {
	hash string
	// unrecorded holds the deferred workloads whose pending-reload annotation failed to save.
	unrecorded map[string]bool
}

// workloadKey identifies a workload within its namespace.
func workloadKey(wl workload.Workload) string {
	return string(wl.Kind()) + "/" + wl.GetName()
}

// Process handles the reload workflow: list workloads, get decisions, webhook or apply.
func (h *ReloadHandler) Process(
	ctx context.Context,
	namespace, resourceName string,
	resourceType matcher.ResourceType,
	getDecisions func([]workload.Workload) []reload.ReloadDecision,
	log logr.Logger,
) (ctrl.Result, error) {
	workloads, err := h.Lister.List(ctx, namespace)
	if err != nil {
		log.Error(err, "failed to list workloads")
		h.Collectors.RecordError("list_workloads")
		return ctrl.Result{}, err
	}

	workloadsByKind := make(map[string]int)
	for _, w := range workloads {
		workloadsByKind[string(w.Kind())]++
	}
	for kind, count := range workloadsByKind {
		h.Collectors.RecordWorkloadsScanned(kind, count)
	}

	decisions := reload.FilterDecisions(getDecisions(workloads))

	matchedByKind := make(map[string]int)
	for _, d := range decisions {
		matchedByKind[string(d.Workload.Kind())]++
	}
	for kind, count := range matchedByKind {
		h.Collectors.RecordWorkloadsMatched(kind, count)
	}

	if len(decisions) == 0 {
		h.Collectors.RecordSkipped("no_match")
	}

	key := types.NamespacedName{Namespace: namespace, Name: resourceName}
	var hash string
	if len(decisions) > 0 {
		hash = decisions[0].Hash
	}
	decisions = h.selectOwed(decisions, h.handledFor(key, hash), resourceType, resourceName, log)

	// Read before reloading: a patched workload drops its entry in the same write.
	hadPending := h.workloadsWithPending(workloads, resourceType, resourceName)
	var ready, deferred []reload.ReloadDecision
	for _, d := range decisions {
		if d.IsDeferred() {
			deferred = append(deferred, d)
		} else {
			ready = append(ready, d)
		}
	}

	if h.WebhookClient.IsConfigured() && len(ready) > 0 {
		// Nothing is recorded before the webhook succeeds, so the error retry handles the
		// change again.
		if err := h.sendWebhook(ctx, resourceName, namespace, resourceType, ready, log); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		ready = h.applyReloads(ctx, resourceName, namespace, resourceType, ready, log)
	}

	unrecorded, err := h.recordPending(ctx, hadPending, decisions, ready, deferred, resourceType, resourceName, log)
	// Only content that sync windows touched can be repeated by a requeue timer or an error
	// retry, so only that is remembered. Without sync windows nothing is kept and the
	// behavior is unchanged.
	if hash != "" && (len(deferred) > 0 || len(hadPending) > 0 || len(unrecorded) > 0) {
		h.setHandled(key, handledChange{hash: hash, unrecorded: unrecorded})
	} else {
		h.forgetHandled(key)
	}
	return ctrl.Result{RequeueAfter: requeueAfter(deferred)}, err
}

// handledFor returns what this process recorded for the resource, if it was for this content.
func (h *ReloadHandler) handledFor(key types.NamespacedName, hash string) *handledChange {
	h.handledMu.Lock()
	defer h.handledMu.Unlock()
	if prev, ok := h.handled[key]; ok && hash != "" && prev.hash == hash {
		return &prev
	}
	return nil
}

func (h *ReloadHandler) setHandled(key types.NamespacedName, change handledChange) {
	h.handledMu.Lock()
	defer h.handledMu.Unlock()
	if h.handled == nil {
		h.handled = make(map[types.NamespacedName]handledChange)
	}
	h.handled[key] = change
}

func (h *ReloadHandler) forgetHandled(key types.NamespacedName) {
	h.handledMu.Lock()
	defer h.handledMu.Unlock()
	delete(h.handled, key)
}

// selectOwed narrows decisions to the workloads still owed a reload for this content. A
// reconcile is a wake-up rather than a new change when this process already handled the
// content (repeat) or when some workload records a pending reload for it (a requeue after
// a restart, or a change to the workload). The other workloads were then already handled
// for this content, and reloading them again would, for a CronJob, start another Job.
func (h *ReloadHandler) selectOwed(
	decisions []reload.ReloadDecision,
	repeat *handledChange,
	resourceType matcher.ResourceType,
	resourceName string,
	log logr.Logger,
) []reload.ReloadDecision {
	key := h.Config.Annotations.PendingReload
	wakeUp := repeat != nil
	for _, d := range decisions {
		if p, ok := reload.FindPendingReload(d.Workload.GetAnnotations(), key, resourceType, resourceName); ok && p.Hash == d.Hash {
			wakeUp = true
			break
		}
	}
	if !wakeUp {
		return decisions
	}

	var owed []reload.ReloadDecision
	for _, d := range decisions {
		_, pending := reload.FindPendingReload(d.Workload.GetAnnotations(), key, resourceType, resourceName)
		if pending || (repeat != nil && repeat.unrecorded[workloadKey(d.Workload)]) {
			owed = append(owed, d)
		}
	}
	if len(owed) == 0 {
		log.V(1).Info("nothing owed for content already handled")
		h.Collectors.RecordSkipped("already_handled")
	}
	return owed
}

// workloadsWithPending returns the workloads that record a pending reload for the resource.
func (h *ReloadHandler) workloadsWithPending(
	workloads []workload.Workload,
	resourceType matcher.ResourceType,
	resourceName string,
) []workload.Workload {
	var result []workload.Workload
	for _, wl := range workloads {
		if _, ok := reload.FindPendingReload(wl.GetAnnotations(), h.Config.Annotations.PendingReload, resourceType, resourceName); ok {
			result = append(result, wl)
		}
	}
	return result
}

// recordPending persists the pending-reload annotation. It records each deferred reload,
// and clears the entry of every workload in hadPending that has reloaded, or that no
// longer reloads for this resource at all. A workload whose reload failed keeps its entry.
// It returns the deferred workloads whose entry could not be written.
func (h *ReloadHandler) recordPending(
	ctx context.Context,
	hadPending []workload.Workload,
	decisions, reloaded, deferred []reload.ReloadDecision,
	resourceType matcher.ResourceType,
	resourceName string,
	log logr.Logger,
) (map[string]bool, error) {
	key := h.Config.Annotations.PendingReload
	var errs []error
	unrecorded := make(map[string]bool)

	for _, d := range deferred {
		entry := reload.PendingReload{Kind: resourceType, Name: resourceName, Hash: d.Hash, NotBefore: d.DeferUntil}
		changed := false
		obj := d.Workload.GetObject()
		if err := UpdateObjectWithRetry(ctx, h.Client, obj, func() (bool, error) {
			changed = reload.SetPendingReload(obj, key, entry)
			return changed, nil
		}); err != nil {
			log.Error(err, "failed to record deferred reload", "workload", d.Workload.GetName(), "kind", d.Workload.Kind())
			h.Collectors.RecordError("record_pending_reload")
			errs = append(errs, err)
			unrecorded[workloadKey(d.Workload)] = true
			continue
		}
		if !changed {
			continue
		}
		log.Info(
			"deferring reload until a sync window allows it",
			"workload", d.Workload.GetName(),
			"kind", d.Workload.Kind(),
			"until", d.DeferUntil.UTC().Format(time.RFC3339),
		)
		h.EventRecorder.ReloadDeferred(obj, resourceType.Kind(), resourceName, d.DeferUntil)
		h.Collectors.RecordSkipped("sync_window")
	}

	for _, wl := range hadPending {
		if !hasDecision(reloaded, wl) && hasDecision(decisions, wl) {
			// Still deferred, or its reload failed and should be tried again.
			continue
		}
		obj := wl.GetObject()
		if err := UpdateObjectWithRetry(ctx, h.Client, obj, func() (bool, error) {
			return reload.ClearPendingReload(obj, key, resourceType, resourceName), nil
		}); err != nil {
			log.Error(err, "failed to clear pending reload", "workload", wl.GetName(), "kind", wl.Kind())
			h.Collectors.RecordError("clear_pending_reload")
			errs = append(errs, err)
		}
	}

	return unrecorded, errors.Join(errs...)
}

func hasDecision(decisions []reload.ReloadDecision, wl workload.Workload) bool {
	for _, d := range decisions {
		if d.Workload.GetObject() == wl.GetObject() {
			return true
		}
	}
	return false
}

// requeueAfter returns how long until the earliest deferred reload is due.
func requeueAfter(deferred []reload.ReloadDecision) time.Duration {
	var after time.Duration
	for i, d := range deferred {
		wait := max(time.Until(d.DeferUntil), time.Second)
		if i == 0 || wait < after {
			after = wait
		}
	}
	return after
}

// ClearPending drops the pending reloads recorded for a resource that no longer exists, so
// workloads do not keep waiting on it.
func (h *ReloadHandler) ClearPending(
	ctx context.Context,
	namespace, resourceName string,
	resourceType matcher.ResourceType,
	log logr.Logger,
) error {
	h.forgetHandled(types.NamespacedName{Namespace: namespace, Name: resourceName})

	workloads, err := h.Lister.List(ctx, namespace)
	if err != nil {
		return err
	}
	_, err = h.recordPending(
		ctx, h.workloadsWithPending(workloads, resourceType, resourceName), nil, nil, nil, resourceType, resourceName, log,
	)
	return err
}

func (h *ReloadHandler) sendWebhook(
	ctx context.Context,
	resourceName, namespace string,
	resourceType matcher.ResourceType,
	decisions []reload.ReloadDecision,
	log logr.Logger,
) error {
	var workloads []webhook.WorkloadInfo
	var hash string
	for _, d := range decisions {
		workloads = append(
			workloads, webhook.WorkloadInfo{
				Kind:      string(d.Workload.Kind()),
				Name:      d.Workload.GetName(),
				Namespace: d.Workload.GetNamespace(),
			},
		)
		if hash == "" {
			hash = d.Hash
		}
	}

	payload := webhook.Payload{
		Kind:         string(resourceType),
		Namespace:    namespace,
		ResourceName: resourceName,
		ResourceType: string(resourceType),
		Hash:         hash,
		Timestamp:    time.Now().UTC(),
		Workloads:    workloads,
	}

	actionStartTime := time.Now()
	if err := h.WebhookClient.Send(ctx, payload); err != nil {
		log.Error(err, "failed to send webhook notification")
		h.Collectors.RecordReload(false, namespace)
		h.Collectors.RecordAction("webhook", "error", time.Since(actionStartTime))
		h.Collectors.RecordError("webhook_send")
		return err
	}

	log.Info(
		"webhook notification sent",
		"resource", resourceName,
		"workloadCount", len(workloads),
	)
	h.Collectors.RecordReload(true, namespace)
	h.Collectors.RecordAction("webhook", "success", time.Since(actionStartTime))
	return nil
}

func (h *ReloadHandler) applyReloads(
	ctx context.Context,
	resourceName, resourceNamespace string,
	resourceType matcher.ResourceType,
	decisions []reload.ReloadDecision,
	log logr.Logger,
) []reload.ReloadDecision {
	// Carry this reconcile's logger, and the resource context it already holds, into
	// the update helpers rather than letting them pick up the manager's own logger.
	ctx = logf.IntoContext(ctx, log)

	var succeeded []reload.ReloadDecision

	for _, decision := range decisions {
		log.Info(
			"reloading workload",
			"workload", decision.Workload.GetName(),
			"kind", decision.Workload.Kind(),
			"reason", decision.Reason,
		)

		actionStartTime := time.Now()
		updated, err := UpdateWorkloadWithRetry(
			ctx,
			h.Client,
			h.ReloadService,
			h.PauseHandler,
			decision.Workload,
			resourceName,
			resourceType,
			resourceNamespace,
			decision.Hash,
			decision.AutoReload,
		)
		actionLatency := time.Since(actionStartTime)

		if err != nil {
			log.Error(
				err, "failed to update workload",
				"workload", decision.Workload.GetName(),
				"kind", decision.Workload.Kind(),
			)
			h.EventRecorder.ReloadFailed(decision.Workload.GetObject(), resourceType.Kind(), resourceName, err)
			h.Collectors.RecordReload(false, resourceNamespace)
			h.Collectors.RecordAction(string(decision.Workload.Kind()), "error", actionLatency)
			h.Collectors.RecordError("update_workload")
			continue
		}
		succeeded = append(succeeded, decision)

		if updated {
			h.EventRecorder.ReloadSuccess(decision.Workload.GetObject(), resourceType.Kind(), resourceName)
			h.Collectors.RecordReload(true, resourceNamespace)
			h.Collectors.RecordAction(string(decision.Workload.Kind()), "success", actionLatency)
			log.Info(
				"workload reloaded successfully",
				"workload", decision.Workload.GetName(),
				"kind", decision.Workload.Kind(),
			)

			if err := h.Alerter.Send(
				ctx, alerting.AlertMessage{
					WorkloadKind:      string(decision.Workload.Kind()),
					WorkloadName:      decision.Workload.GetName(),
					WorkloadNamespace: decision.Workload.GetNamespace(),
					ResourceKind:      resourceType.Kind(),
					ResourceName:      resourceName,
					ResourceNamespace: resourceNamespace,
					Timestamp:         time.Now(),
				},
			); err != nil {
				log.Error(err, "failed to send alert")
			}
		} else {
			h.Collectors.RecordAction(string(decision.Workload.Kind()), "no_change", actionLatency)
		}
	}
	return succeeded
}
