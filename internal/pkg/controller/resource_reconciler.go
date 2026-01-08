package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
)

// ResourceReconcilerDeps holds shared dependencies for resource reconcilers.
type ResourceReconcilerDeps struct {
	Client         client.Client
	Log            logr.Logger
	Config         *config.Config
	ReloadService  *reload.Service
	Registry       *workload.Registry
	Collectors     *metrics.Collectors
	EventRecorder  *events.Recorder
	WebhookClient  *webhook.Client
	Alerter        alerting.Alerter
	PauseHandler   *reload.PauseHandler
	NamespaceCache *NamespaceCache
}

// ResourceConfig provides type-specific configuration for a resource reconciler.
type ResourceConfig[T client.Object] struct {
	// ResourceType identifies the type of resource (configmap or secret).
	ResourceType reload.ResourceType

	// NewResource creates a new instance of the resource type.
	NewResource func() T

	// CreateChange creates a change event for the resource.
	CreateChange func(resource T, eventType reload.EventType) reload.ResourceChange

	// CreatePredicates creates the predicates for this resource type.
	CreatePredicates func(cfg *config.Config, hasher *reload.Hasher) predicate.Predicate
}

// ResourceReconciler is a generic reconciler for ConfigMaps and Secrets.
type ResourceReconciler[T client.Object] struct {
	ResourceReconcilerDeps
	ResourceConfig[T]

	handler     *ReloadHandler
	initialized bool
	initOnce    sync.Once
}

// NewResourceReconciler creates a new generic resource reconciler.
func NewResourceReconciler[T client.Object](
	deps ResourceReconcilerDeps,
	cfg ResourceConfig[T],
) *ResourceReconciler[T] {
	return &ResourceReconciler[T]{
		ResourceReconcilerDeps: deps,
		ResourceConfig:         cfg,
	}
}

// Reconcile handles resource events and triggers workload reloads as needed.
func (r *ResourceReconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	resourceType := string(r.ResourceType)
	log := r.Log.WithValues(resourceType, req.NamespacedName)

	r.initOnce.Do(
		func() {
			r.initialized = true
			log.Info(resourceType + " controller initialized")
		},
	)

	r.Collectors.RecordEventReceived("reconcile", resourceType)

	resource := r.NewResource()
	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			return r.handleNotFound(ctx, req, log, startTime)
		}
		log.Error(err, "failed to get "+resourceType)
		r.Collectors.RecordError("get_" + resourceType)
		r.Collectors.RecordReconcile("error", time.Since(startTime))
		return ctrl.Result{}, err
	}

	namespace := resource.GetNamespace()
	if r.Config.IsNamespaceIgnored(namespace) {
		log.V(1).Info("skipping " + resourceType + " in ignored namespace")
		r.Collectors.RecordSkipped("ignored_namespace")
		r.Collectors.RecordReconcile("success", time.Since(startTime))
		return ctrl.Result{}, nil
	}

	if r.NamespaceCache != nil && r.NamespaceCache.IsEnabled() && !r.NamespaceCache.Contains(namespace) {
		log.V(1).Info("skipping "+resourceType+" in namespace not matching selector", "namespace", namespace)
		r.Collectors.RecordSkipped("namespace_selector")
		r.Collectors.RecordReconcile("success", time.Since(startTime))
		return ctrl.Result{}, nil
	}

	result, err := r.reloadHandler().Process(
		ctx, req.Namespace, req.Name, r.ResourceType,
		func(workloads []workload.Workload) []reload.ReloadDecision {
			return r.ReloadService.Process(r.CreateChange(resource, reload.EventTypeUpdate), workloads)
		}, log,
	)

	r.recordReconcile(startTime, err)
	return result, err
}

func (r *ResourceReconciler[T]) handleNotFound(
	ctx context.Context,
	req ctrl.Request,
	log logr.Logger,
	startTime time.Time,
) (ctrl.Result, error) {
	if r.Config.ReloadOnDelete {
		r.Collectors.RecordEventReceived("delete", string(r.ResourceType))
		result, err := r.handleDelete(ctx, req, log)
		r.recordReconcile(startTime, err)
		return result, err
	}
	r.Collectors.RecordSkipped("not_found")
	r.Collectors.RecordReconcile("success", time.Since(startTime))
	return ctrl.Result{}, nil
}

func (r *ResourceReconciler[T]) handleDelete(
	ctx context.Context,
	req ctrl.Request,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Info("handling " + string(r.ResourceType) + " deletion")

	// Create a minimal resource with just name/namespace for the delete event
	resource := r.NewResource()
	resource.SetName(req.Name)
	resource.SetNamespace(req.Namespace)

	return r.reloadHandler().Process(
		ctx, req.Namespace, req.Name, r.ResourceType,
		func(workloads []workload.Workload) []reload.ReloadDecision {
			return r.ReloadService.Process(r.CreateChange(resource, reload.EventTypeDelete), workloads)
		}, log,
	)
}

func (r *ResourceReconciler[T]) recordReconcile(startTime time.Time, err error) {
	if err != nil {
		r.Collectors.RecordReconcile("error", time.Since(startTime))
	} else {
		r.Collectors.RecordReconcile("success", time.Since(startTime))
	}
}

func (r *ResourceReconciler[T]) reloadHandler() *ReloadHandler {
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

// Initialized returns whether the reconciler has been initialized.
func (r *ResourceReconciler[T]) Initialized() *bool {
	return &r.initialized
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceReconciler[T]) SetupWithManager(mgr ctrl.Manager, forObject T) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(forObject).
		WithEventFilter(
			BuildEventFilter(
				r.CreatePredicates(r.Config, r.ReloadService.Hasher()),
				r.Config, r.Initialized(),
			),
		).
		Complete(r)
}
