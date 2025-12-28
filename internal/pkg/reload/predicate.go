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
	return resourcePredicates(
		cfg, func(old, new client.Object) (string, string, bool) {
			oldCM, okOld := old.(*corev1.ConfigMap)
			newCM, okNew := new.(*corev1.ConfigMap)
			if !okOld || !okNew {
				return "", "", false
			}
			return hasher.HashConfigMap(oldCM), hasher.HashConfigMap(newCM), true
		},
	)
}

// SecretPredicates returns predicates for filtering Secret events.
func SecretPredicates(cfg *config.Config, hasher *Hasher) predicate.Predicate {
	return resourcePredicates(
		cfg, func(old, new client.Object) (string, string, bool) {
			oldSecret, okOld := old.(*corev1.Secret)
			newSecret, okNew := new.(*corev1.Secret)
			if !okOld || !okNew {
				return "", "", false
			}
			return hasher.HashSecret(oldSecret), hasher.HashSecret(newSecret), true
		},
	)
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
	return predicate.NewPredicateFuncs(
		func(obj client.Object) bool {
			namespace := obj.GetNamespace()

			if cfg.IsNamespaceIgnored(namespace) {
				return false
			}

			if nsCache != nil && !nsCache.Contains(namespace) {
				return false
			}

			return true
		},
	)
}

// LabelSelectorPredicate returns a predicate that filters resources by labels.
func LabelSelectorPredicate(cfg *config.Config) predicate.Predicate {
	if len(cfg.ResourceSelectors) == 0 {
		return predicate.NewPredicateFuncs(
			func(obj client.Object) bool {
				return true
			},
		)
	}

	return predicate.NewPredicateFuncs(
		func(obj client.Object) bool {
			labels := obj.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}

			for _, selector := range cfg.ResourceSelectors {
				if selector.Matches(LabelsSet(labels)) {
					return true
				}
			}

			return false
		},
	)
}

// LabelsSet implements the k8s.io/apimachinery/pkg/labels.Labels interface
// for a map[string]string. This allows using label maps with label selectors.
type LabelsSet map[string]string

// Has returns whether the provided label key exists in the set.
func (ls LabelsSet) Has(key string) bool {
	_, ok := ls[key]
	return ok
}

// Get returns the value for the provided label key.
func (ls LabelsSet) Get(key string) string {
	return ls[key]
}

// Lookup returns the value for the provided label key and whether it exists.
func (ls LabelsSet) Lookup(key string) (string, bool) {
	value, ok := ls[key]
	return value, ok
}

// IgnoreAnnotationPredicate returns a predicate that filters out resources with the ignore annotation.
func IgnoreAnnotationPredicate(cfg *config.Config) predicate.Predicate {
	return predicate.NewPredicateFuncs(
		func(obj client.Object) bool {
			annotations := obj.GetAnnotations()
			if annotations == nil {
				return true
			}

			return annotations[cfg.Annotations.Ignore] != "true"
		},
	)
}

// CombinedPredicates combines multiple predicates with AND logic.
func CombinedPredicates(predicates ...predicate.Predicate) predicate.Predicate {
	return predicate.And(predicates...)
}
