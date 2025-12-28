package reload

import (
	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ConfigMapPredicates returns predicates for filtering ConfigMap events.
func ConfigMapPredicates(cfg *config.Config, hasher *Hasher) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Only process create events if ReloadOnCreate is enabled
			// or if SyncAfterRestart is enabled (for initial sync)
			return cfg.ReloadOnCreate || cfg.SyncAfterRestart
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Always process updates, but filter by content change
			oldCM, okOld := e.ObjectOld.(*corev1.ConfigMap)
			newCM, okNew := e.ObjectNew.(*corev1.ConfigMap)
			if !okOld || !okNew {
				return false
			}

			// Check if the data actually changed
			oldHash := hasher.HashConfigMap(oldCM)
			newHash := hasher.HashConfigMap(newCM)
			return oldHash != newHash
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Only process delete events if ReloadOnDelete is enabled
			return cfg.ReloadOnDelete
		},
		GenericFunc: func(e event.GenericEvent) bool {
			// Ignore generic events
			return false
		},
	}
}

// SecretPredicates returns predicates for filtering Secret events.
func SecretPredicates(cfg *config.Config, hasher *Hasher) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Only process create events if ReloadOnCreate is enabled
			// or if SyncAfterRestart is enabled (for initial sync)
			return cfg.ReloadOnCreate || cfg.SyncAfterRestart
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Always process updates, but filter by content change
			oldSecret, okOld := e.ObjectOld.(*corev1.Secret)
			newSecret, okNew := e.ObjectNew.(*corev1.Secret)
			if !okOld || !okNew {
				return false
			}

			// Check if the data actually changed
			oldHash := hasher.HashSecret(oldSecret)
			newHash := hasher.HashSecret(newSecret)
			return oldHash != newHash
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Only process delete events if ReloadOnDelete is enabled
			return cfg.ReloadOnDelete
		},
		GenericFunc: func(e event.GenericEvent) bool {
			// Ignore generic events
			return false
		},
	}
}

// NamespaceFilterPredicate returns a predicate that filters resources by namespace.
func NamespaceFilterPredicate(cfg *config.Config) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		namespace := obj.GetNamespace()

		// Check if namespace should be ignored
		if cfg.IsNamespaceIgnored(namespace) {
			return false
		}

		// Check namespace selectors
		// Note: For now, we pass through and let the controller handle selector matching
		// A more efficient implementation would check labels here
		return true
	})
}

// LabelSelectorPredicate returns a predicate that filters resources by labels.
func LabelSelectorPredicate(cfg *config.Config) predicate.Predicate {
	if len(cfg.ResourceSelectors) == 0 {
		// No selectors configured, allow all
		return predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return true
		})
	}

	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}

		// Check if any selector matches
		for _, selector := range cfg.ResourceSelectors {
			if selector.Matches(labelsSet(labels)) {
				return true
			}
		}

		return false
	})
}

// labelsSet implements labels.Labels interface for a map.
type labelsSet map[string]string

func (ls labelsSet) Has(key string) bool {
	_, ok := ls[key]
	return ok
}

func (ls labelsSet) Get(key string) string {
	return ls[key]
}

// IgnoreAnnotationPredicate returns a predicate that filters out resources with the ignore annotation.
func IgnoreAnnotationPredicate(cfg *config.Config) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			return true
		}

		// Check for ignore annotation
		return annotations[cfg.Annotations.Ignore] != "true"
	})
}

// CombinedPredicates combines multiple predicates with AND logic.
func CombinedPredicates(predicates ...predicate.Predicate) predicate.Predicate {
	return predicate.And(predicates...)
}
