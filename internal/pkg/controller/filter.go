package controller

import (
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// BuildEventFilter combines a resource-specific predicate with common filters.
func BuildEventFilter(resourcePredicate predicate.Predicate, cfg *config.Config, initialized *bool) predicate.Predicate {
	return predicate.And(
		resourcePredicate,
		reload.NamespaceFilterPredicate(cfg),
		reload.LabelSelectorPredicate(cfg),
		reload.IgnoreAnnotationPredicate(cfg),
		createEventPredicate(cfg, initialized),
	)
}

func createEventPredicate(cfg *config.Config, initialized *bool) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if !*initialized && !cfg.SyncAfterRestart {
				return false
			}
			return cfg.ReloadOnCreate
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return cfg.ReloadOnDelete
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
