package controller

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/reload"
)

// BuildEventFilter combines a resource-specific predicate with common filters.
//
// startTime is the moment the controller began watching; it is used to tell
// genuine post-startup creates apart from the initial-sync replay of
// pre-existing resources (which the informer delivers as create events).
func BuildEventFilter(resourcePredicate predicate.Predicate, cfg *config.Config, startTime time.Time) predicate.Predicate {
	return predicate.And(
		resourcePredicate,
		reload.NamespaceFilterPredicate(cfg),
		reload.LabelSelectorPredicate(cfg),
		reload.IgnoreAnnotationPredicate(cfg),
		createEventPredicate(cfg, startTime),
	)
}

func createEventPredicate(cfg *config.Config, startTime time.Time) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if !cfg.ReloadOnCreate {
				return false
			}
			// SyncAfterRestart processes every create, including the
			// initial-sync replay of resources that already existed.
			if cfg.SyncAfterRestart {
				return true
			}
			// Otherwise only honor resources created after the controller
			// started. Resources replayed during the initial cache sync carry
			// an older creation timestamp and must not trigger reloads on
			// startup, but a genuine create that arrives afterwards must be
			// honored even if it is the very first event this controller sees.
			return e.Object.GetCreationTimestamp().Time.After(startTime)
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
