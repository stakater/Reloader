package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
}

// Process handles the reload workflow: list workloads, get decisions, webhook or apply.
func (h *ReloadHandler) Process(
	ctx context.Context,
	namespace, resourceName string,
	resourceType reload.ResourceType,
	getDecisions func([]workload.WorkloadAccessor) []reload.ReloadDecision,
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

	if h.WebhookClient.IsConfigured() && len(decisions) > 0 {
		return h.sendWebhook(ctx, resourceName, namespace, resourceType, decisions, log)
	}

	h.applyReloads(ctx, resourceName, namespace, resourceType, decisions, log)
	return ctrl.Result{}, nil
}

func (h *ReloadHandler) sendWebhook(
	ctx context.Context,
	resourceName, namespace string,
	resourceType reload.ResourceType,
	decisions []reload.ReloadDecision,
	log logr.Logger,
) (ctrl.Result, error) {
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
		return ctrl.Result{}, err
	}

	log.Info(
		"webhook notification sent",
		"resource", resourceName,
		"workloadCount", len(workloads),
	)
	h.Collectors.RecordReload(true, namespace)
	h.Collectors.RecordAction("webhook", "success", time.Since(actionStartTime))
	return ctrl.Result{}, nil
}

func (h *ReloadHandler) applyReloads(
	ctx context.Context,
	resourceName, resourceNamespace string,
	resourceType reload.ResourceType,
	decisions []reload.ReloadDecision,
	log logr.Logger,
) {
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
}
