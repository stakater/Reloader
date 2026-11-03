package controller

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/matcher"
)

// deferredDecision is a reload that waits for some of its reload-after dependencies.
type deferredDecision struct {
	decision     reload.ReloadDecision
	dependencies []reload.DependencyRef
}

func refOf(wl workload.Workload) reload.DependencyRef {
	return reload.DependencyRef{Kind: wl.Kind(), Name: wl.GetName()}
}

// splitDeferred separates decisions that can reload now from those whose
// reload-after dependencies are reloaded by the same change. Deferred decisions
// come back with dependencies before dependents, so a chain's pending state is
// recorded in order. Workloads caught in a reload-after cycle reload immediately.
func splitDeferred(
	svc *reload.Service,
	decisions []reload.ReloadDecision,
	log logr.Logger,
) ([]reload.ReloadDecision, []deferredDecision) {
	inBatch := make(map[reload.DependencyRef]struct{}, len(decisions))
	for _, d := range decisions {
		inBatch[refOf(d.Workload)] = struct{}{}
	}

	var immediate []reload.ReloadDecision
	candidates := make(map[reload.DependencyRef]deferredDecision)
	var order []reload.DependencyRef
	for _, d := range decisions {
		refs, err := svc.ReloadAfter(d.Workload)
		if err != nil {
			svc.ReportReloadAfterError(err, d.Workload)
		}
		self := refOf(d.Workload)
		var deps []reload.DependencyRef
		for _, ref := range refs {
			if _, ok := inBatch[ref]; ok && ref != self {
				deps = append(deps, ref)
			}
		}
		if len(deps) == 0 {
			immediate = append(immediate, d)
			continue
		}
		candidates[self] = deferredDecision{decision: d, dependencies: deps}
		order = append(order, self)
	}

	// Topologically order the candidates; whatever cannot be ordered is in or behind a cycle.
	var deferred []deferredDecision
	placed := make(map[reload.DependencyRef]bool, len(candidates))
	for progress := true; progress; {
		progress = false
		for _, ref := range order {
			if placed[ref] {
				continue
			}
			ready := true
			for _, dep := range candidates[ref].dependencies {
				if _, isCandidate := candidates[dep]; isCandidate && !placed[dep] {
					ready = false
					break
				}
			}
			if ready {
				placed[ref] = true
				deferred = append(deferred, candidates[ref])
				progress = true
			}
		}
	}
	for _, ref := range order {
		if !placed[ref] {
			log.Info(
				"reload-after dependencies form a cycle, reloading without waiting",
				"workload", ref.Name, "kind", ref.Kind,
			)
			immediate = append(immediate, candidates[ref].decision)
		}
	}
	return immediate, deferred
}

// deferReloads records each deferred decision in its workload's pending-reload
// annotation. The annotation lives in metadata, so writing it starts no rollout.
func (h *ReloadHandler) deferReloads(
	ctx context.Context,
	resourceName, resourceNamespace string,
	resourceType matcher.ResourceType,
	deferred []deferredDecision,
	log logr.Logger,
) {
	annotationKey := h.ReloadService.PendingReloadAnnotation()
	now := time.Now().UTC()

	for _, dd := range deferred {
		wl := dd.decision.Workload
		entry := reload.PendingReload{
			ResourceType:      resourceType,
			ResourceName:      resourceName,
			ResourceNamespace: resourceNamespace,
			Hash:              dd.decision.Hash,
			AutoReload:        dd.decision.AutoReload,
			Dependencies:      dd.dependencies,
			Since:             now,
			Deadline:          now.Add(h.ReloadService.ReloadAfterTimeout(wl)),
		}

		obj := wl.GetObject()
		err := UpdateObjectWithRetry(
			ctx, h.Client, obj, func() (bool, error) {
				annotations := obj.GetAnnotations()
				pending, err := reload.ParsePendingReloads(annotations[annotationKey])
				if err != nil {
					log.Error(err, "discarding unreadable pending reloads", "workload", obj.GetName())
					pending = reload.PendingReloads{}
				}
				if existing, ok := pending[entry.Key()]; ok && existing.Hash == entry.Hash {
					return false, nil
				}
				pending[entry.Key()] = entry
				value, err := pending.Encode()
				if err != nil {
					return false, err
				}
				if annotations == nil {
					annotations = map[string]string{}
				}
				annotations[annotationKey] = value
				obj.SetAnnotations(annotations)
				return true, nil
			},
		)
		if err != nil {
			log.Error(err, "failed to defer reload", "workload", wl.GetName(), "kind", wl.Kind())
			h.EventRecorder.ReloadFailed(obj, resourceType.Kind(), resourceName, err)
			h.Collectors.RecordError("defer_reload")
			continue
		}

		deps := make([]string, len(dd.dependencies))
		for i, dep := range dd.dependencies {
			deps[i] = dep.String()
		}
		log.Info(
			"deferring workload reload until dependencies finish",
			"workload", wl.GetName(), "kind", wl.Kind(), "dependencies", deps,
		)
		h.EventRecorder.ReloadDeferred(obj, resourceType.Kind(), resourceName, strings.Join(deps, ", "))
		h.Collectors.RecordSkipped("deferred")
	}
}
