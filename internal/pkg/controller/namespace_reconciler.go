package controller

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NamespaceCache provides thread-safe access to the set of namespaces
// that match the configured namespace label selector.
type NamespaceCache struct {
	mu         sync.RWMutex
	namespaces map[string]struct{}
	enabled    bool
}

// NewNamespaceCache creates a new NamespaceCache.
// If enabled is false, all namespace checks return true (allow all).
func NewNamespaceCache(enabled bool) *NamespaceCache {
	return &NamespaceCache{
		namespaces: make(map[string]struct{}),
		enabled:    enabled,
	}
}

// Add adds a namespace to the cache.
func (c *NamespaceCache) Add(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.namespaces[name] = struct{}{}
}

// Remove removes a namespace from the cache.
func (c *NamespaceCache) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.namespaces, name)
}

// Contains checks if a namespace is in the cache.
// If namespace selectors are not enabled, always returns true.
func (c *NamespaceCache) Contains(name string) bool {
	if !c.enabled {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.namespaces[name]
	return ok
}

// List returns a copy of all namespace names in the cache.
func (c *NamespaceCache) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0, len(c.namespaces))
	for name := range c.namespaces {
		result = append(result, name)
	}
	return result
}

// IsEnabled returns whether namespace selector filtering is enabled.
func (c *NamespaceCache) IsEnabled() bool {
	return c.enabled
}

// NamespaceReconciler watches Namespace objects and maintains a cache
// of namespaces that match the configured label selector.
type NamespaceReconciler struct {
	client.Client
	Log    logr.Logger
	Config *config.Config
	Cache  *NamespaceCache
}

// Reconcile handles Namespace events and updates the namespace cache.
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.Name)

	var ns corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &ns); err != nil {
		if errors.IsNotFound(err) {
			// Namespace was deleted - remove from cache
			r.Cache.Remove(req.Name)
			log.V(1).Info("removed namespace from cache (deleted)")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get Namespace")
		return ctrl.Result{}, err
	}

	// Check if namespace matches any of the configured selectors
	if r.matchesSelectors(&ns) {
		r.Cache.Add(ns.Name)
		log.V(1).Info("added namespace to cache")
	} else {
		// Labels might have changed, remove from cache if no longer matches
		r.Cache.Remove(ns.Name)
		log.V(1).Info("removed namespace from cache (labels no longer match)")
	}

	return ctrl.Result{}, nil
}

// matchesSelectors checks if the namespace matches any configured label selector.
func (r *NamespaceReconciler) matchesSelectors(ns *corev1.Namespace) bool {
	if len(r.Config.NamespaceSelectors) == 0 {
		// No selectors configured - should not happen since reconciler is only
		// set up when selectors are configured, but handle gracefully
		return true
	}

	nsLabels := ns.GetLabels()
	if nsLabels == nil {
		nsLabels = make(map[string]string)
	}

	for _, selector := range r.Config.NamespaceSelectors {
		if selector.Matches(nsLabelsSet(nsLabels)) {
			return true
		}
	}

	return false
}

// nsLabelsSet implements labels.Labels interface for a map.
type nsLabelsSet map[string]string

func (ls nsLabelsSet) Has(key string) bool {
	_, ok := ls[key]
	return ok
}

func (ls nsLabelsSet) Get(key string) string {
	return ls[key]
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}

// Ensure NamespaceReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &NamespaceReconciler{}
