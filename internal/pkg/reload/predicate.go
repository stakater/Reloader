package reload

import (
	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// resourcePredicates returns predicates for filtering resource events.
// The hashFn computes a hash from old and new objects to detect content changes.
func resourcePredicates(cfg *config.Config, hashFn func(old, new client.Object) (string, string, bool)) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return cfg.ReloadOnCreate || cfg.SyncAfterRestart
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldHash, newHash, ok := hashFn(e.ObjectOld, e.ObjectNew)
			if !ok {
				return false
			}
			return oldHash != newHash
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return cfg.ReloadOnDelete
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// ConfigMapPredicates returns predicates for filtering ConfigMap events.
func ConfigMapPredicates(cfg *config.Config, hasher *Hasher) predicate.Predicate {
	return resourcePredicates(cfg, func(old, new client.Object) (string, string, bool) {
		oldCM, okOld := old.(*corev1.ConfigMap)
		newCM, okNew := new.(*corev1.ConfigMap)
		if !okOld || !okNew {
			return "", "", false
		}
		return hasher.HashConfigMap(oldCM), hasher.HashConfigMap(newCM), true
	})
}

// SecretPredicates returns predicates for filtering Secret events.
func SecretPredicates(cfg *config.Config, hasher *Hasher) predicate.Predicate {
	return resourcePredicates(cfg, func(old, new client.Object) (string, string, bool) {
		oldSecret, okOld := old.(*corev1.Secret)
		newSecret, okNew := new.(*corev1.Secret)
		if !okOld || !okNew {
			return "", "", false
		}
		return hasher.HashSecret(oldSecret), hasher.HashSecret(newSecret), true
	})
}

// NamespaceChecker defines the interface for checking if a namespace is allowed.
type NamespaceChecker interface {
	Contains(name string) bool
}

// NamespaceFilterPredicate returns a predicate that filters resources by namespace.
func NamespaceFilterPredicate(cfg *config.Config) predicate.Predicate {
	return NamespaceFilterPredicateWithCache(cfg, nil)
}

// NamespaceFilterPredicateWithCache returns a predicate that filters resources by namespace,
// using the provided NamespaceChecker for namespace selector filtering.
func NamespaceFilterPredicateWithCache(cfg *config.Config, nsCache NamespaceChecker) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		namespace := obj.GetNamespace()

		// Check if namespace should be ignored
		if cfg.IsNamespaceIgnored(namespace) {
			return false
		}

		// Check namespace selector cache if provided
		if nsCache != nil && !nsCache.Contains(namespace) {
			return false
		}

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
