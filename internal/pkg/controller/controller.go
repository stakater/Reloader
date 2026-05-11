package controller

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/scheme"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/handler"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
)

// Controller for checking events
type Controller struct {
	client            kubernetes.Interface
	queue             workqueue.TypedRateLimitingInterface[any]
	informer          cache.Controller
	namespace         string
	resource          string
	ignoredNamespaces util.List
	collectors        metrics.Collectors
	recorder          record.EventRecorder
	namespaceSelector string
	resourceSelector  string
}

// controllerInitialized flags guard against processing Add/Delete events before
// the worker goroutines have started. Written by runWorker (in a goroutine) and
// read by the informer event handlers, so they must be atomic.
var secretControllerInitialized atomic.Bool
var configmapControllerInitialized atomic.Bool

// selectedNamespacesCache holds an immutable snapshot of the set of namespace
// names that match the namespace label selector. Written exclusively by the
// namespace controller's informer goroutine; read concurrently by configmap/
// secret controller informer goroutines. Using atomic.Value with an immutable
// map[string]struct{} snapshot avoids mutexes and prevents data races.
var selectedNamespacesCache atomic.Value // always stores map[string]struct{}

// loadSelectedNamespaces returns the current namespace snapshot (never nil).
func loadSelectedNamespaces() map[string]struct{} {
	if v := selectedNamespacesCache.Load(); v != nil {
		return v.(map[string]struct{})
	}
	return map[string]struct{}{}
}

// storeSelectedNamespaces replaces the current snapshot with one built from ns.
// It is the only mutator of selectedNamespacesCache and is called only from
// the namespace controller's informer goroutine (or from tests for setup).
func storeSelectedNamespaces(ns []string) {
	m := make(map[string]struct{}, len(ns))
	for _, n := range ns {
		m[n] = struct{}{}
	}
	selectedNamespacesCache.Store(m)
}

// loadSelectedNamespacesList returns the current namespace names as a slice.
// Intended for use in tests where slice-based assertions are more convenient.
func loadSelectedNamespacesList() []string {
	m := loadSelectedNamespaces()
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// NewController for initializing a Controller
func NewController(client kubernetes.Interface, resource string, namespace string, ignoredNamespaces []string, namespaceLabelSelector string, resourceLabelSelector string, collectors metrics.Collectors) (*Controller, error) {
	if options.SyncAfterRestart {
		secretControllerInitialized.Store(true)
		configmapControllerInitialized.Store(true)
	}

	c := Controller{
		client:            client,
		namespace:         namespace,
		ignoredNamespaces: ignoredNamespaces,
		namespaceSelector: namespaceLabelSelector,
		resourceSelector:  resourceLabelSelector,
		resource:          resource,
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: client.CoreV1().Events(""),
	})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme,
		v1.EventSource{Component: fmt.Sprintf("reloader-%s", resource)})

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]())

	optionsModifier := func(opts *metav1.ListOptions) {
		if resource == "namespaces" {
			opts.LabelSelector = c.namespaceSelector
		} else if len(c.resourceSelector) > 0 {
			opts.LabelSelector = c.resourceSelector
		} else {
			opts.FieldSelector = fields.Everything().String()
		}
	}

	getterRESTClient, err := getClientForResource(resource, client)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize REST client for %s: %w", resource, err)
	}

	listWatcher := cache.NewFilteredListWatchFromClient(getterRESTClient, resource, namespace, optionsModifier)

	_, informer := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: listWatcher,
		ObjectType:    kube.ResourceMap[resource],
		ResyncPeriod:  0,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.Add,
			UpdateFunc: c.Update,
			DeleteFunc: c.Delete,
		},
		Indexers: cache.Indexers{},
	})
	c.informer = informer
	c.queue = queue
	c.collectors = collectors
	c.recorder = recorder

	logrus.Infof("created controller for: %s", resource)
	return &c, nil
}

// Add function to add a new object to the queue in case of creating a resource
func (c *Controller) Add(obj interface{}) {
	c.collectors.RecordEventReceived("add", c.resource)

	switch object := obj.(type) {
	case *v1.Namespace:
		c.addSelectedNamespaceToCache(*object)
		return
	case *csiv1.SecretProviderClassPodStatus:
		return
	}

	if options.ReloadOnCreate == "true" {
		if !c.resourceInIgnoredNamespace(obj) && c.resourceInSelectedNamespaces(obj) && secretControllerInitialized.Load() && configmapControllerInitialized.Load() {
			c.enqueue(handler.ResourceCreatedHandler{
				Resource:    obj,
				Collectors:  c.collectors,
				Recorder:    c.recorder,
				EnqueueTime: time.Now(),
			})
		} else {
			c.collectors.RecordSkipped("ignored_or_not_selected")
		}
	}
}

func (c *Controller) resourceInIgnoredNamespace(raw interface{}) bool {
	switch obj := raw.(type) {
	case *v1.ConfigMap:
		return c.ignoredNamespaces.Contains(obj.Namespace)
	case *v1.Secret:
		return c.ignoredNamespaces.Contains(obj.Namespace)
	case *csiv1.SecretProviderClassPodStatus:
		return c.ignoredNamespaces.Contains(obj.Namespace)
	}
	return false
}

func (c *Controller) resourceInSelectedNamespaces(raw interface{}) bool {
	if len(c.namespaceSelector) == 0 {
		return true
	}

	namespaces := loadSelectedNamespaces()
	var ns string
	switch object := raw.(type) {
	case *v1.ConfigMap:
		ns = object.GetNamespace()
	case *v1.Secret:
		ns = object.GetNamespace()
	case *csiv1.SecretProviderClassPodStatus:
		ns = object.GetNamespace()
	default:
		return false
	}
	_, ok := namespaces[ns]
	return ok
}

func (c *Controller) addSelectedNamespaceToCache(namespace v1.Namespace) {
	old := loadSelectedNamespaces()
	next := make(map[string]struct{}, len(old)+1)
	for k := range old {
		next[k] = struct{}{}
	}
	next[namespace.GetName()] = struct{}{}
	selectedNamespacesCache.Store(next)
	logrus.Infof("added namespace to be watched: %s", namespace.GetName())
}

func (c *Controller) removeSelectedNamespaceFromCache(namespace v1.Namespace) {
	old := loadSelectedNamespaces()
	if _, ok := old[namespace.GetName()]; !ok {
		return
	}
	next := make(map[string]struct{}, len(old))
	for k := range old {
		next[k] = struct{}{}
	}
	delete(next, namespace.GetName())
	selectedNamespacesCache.Store(next)
	logrus.Infof("removed namespace from watch: %s", namespace.GetName())
}

// Update function to add an old object and a new object to the queue in case of updating a resource
func (c *Controller) Update(old interface{}, new interface{}) {
	c.collectors.RecordEventReceived("update", c.resource)

	switch new.(type) {
	case *v1.Namespace:
		return
	}

	if !c.resourceInIgnoredNamespace(new) && c.resourceInSelectedNamespaces(new) {
		c.enqueue(handler.ResourceUpdatedHandler{
			Resource:    new,
			OldResource: old,
			Collectors:  c.collectors,
			Recorder:    c.recorder,
			EnqueueTime: time.Now(),
		})
	} else {
		c.collectors.RecordSkipped("ignored_or_not_selected")
	}
}

// Delete function to add an object to the queue in case of deleting a resource
func (c *Controller) Delete(old interface{}) {
	c.collectors.RecordEventReceived("delete", c.resource)

	if _, ok := old.(*csiv1.SecretProviderClassPodStatus); ok {
		return
	}

	if options.ReloadOnDelete == "true" {
		if !c.resourceInIgnoredNamespace(old) && c.resourceInSelectedNamespaces(old) && secretControllerInitialized.Load() && configmapControllerInitialized.Load() {
			c.enqueue(handler.ResourceDeleteHandler{
				Resource:    old,
				Collectors:  c.collectors,
				Recorder:    c.recorder,
				EnqueueTime: time.Now(),
			})
		} else {
			c.collectors.RecordSkipped("ignored_or_not_selected")
		}
	}

	switch object := old.(type) {
	case *v1.Namespace:
		c.removeSelectedNamespaceFromCache(*object)
		return
	}
}

// enqueue adds an item to the queue and records metrics
func (c *Controller) enqueue(item interface{}) {
	c.queue.Add(item)
	c.collectors.RecordQueueAdd()
	c.collectors.SetQueueDepth(c.queue.Len())
}

// Run function for controller which handles the queue
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		c.informer.Run(stopCh)
	}()

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		c.queue.ShutDown()
		wg.Wait()
		return
	}

	for i := 0; i < threadiness; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.Until(c.runWorker, time.Second, stopCh)
		}()
	}

	<-stopCh
	logrus.Infof("Stopping Controller for %s", c.resource)
	c.queue.ShutDown() // unblock workers so they drain and exit
	logrus.Infof("Queue shut down for %s, waiting for goroutines", c.resource)
	wg.Wait() // block until informer and all workers have exited
	logrus.Infof("All goroutines exited for %s", c.resource)
}

func (c *Controller) runWorker() {
	// At this point the controller is fully initialized and we can start processing the resources
	if c.resource == string(v1.ResourceSecrets) {
		secretControllerInitialized.Store(true)
	} else if c.resource == string(v1.ResourceConfigMaps) {
		configmapControllerInitialized.Store(true)
	}

	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	resourceHandler, quit := c.queue.Get()
	if quit {
		return false
	}

	c.collectors.SetQueueDepth(c.queue.Len())

	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two events with the same key are never processed in
	// parallel.
	defer c.queue.Done(resourceHandler)

	// Record queue latency if the handler supports it
	if h, ok := resourceHandler.(handler.TimedHandler); ok {
		queueLatency := time.Since(h.GetEnqueueTime())
		c.collectors.RecordQueueLatency(queueLatency)
	}

	// Track reconcile/handler duration
	startTime := time.Now()

	// Invoke the method containing the business logic
	rh, ok := resourceHandler.(handler.ResourceHandler)
	if !ok {
		logrus.Errorf("Invalid resource handler type: %T", resourceHandler)
		// Clear rate-limiter state so the item doesn't leak memory in the queue.
		c.queue.Forget(resourceHandler)
		c.collectors.RecordError("invalid_handler_type")
		return true
	}
	err := rh.Handle()

	duration := time.Since(startTime)

	if err != nil {
		c.collectors.RecordReconcile("error", duration)
	} else {
		c.collectors.RecordReconcile("success", duration)
	}

	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, resourceHandler)
	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)

		// Record successful event processing
		c.collectors.RecordEventProcessed("unknown", c.resource, "success")
		return
	}

	// Record error
	c.collectors.RecordError("handler_error")

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		logrus.Errorf("Error syncing events: %v", err)

		// Record retry
		c.collectors.RecordRetry()

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		c.collectors.SetQueueDepth(c.queue.Len())
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	logrus.Errorf("Dropping key out of the queue: %v", err)
	logrus.Debugf("Dropping the key %q out of the queue: %v", key, err)

	c.collectors.RecordEventProcessed("unknown", c.resource, "dropped")
}

func getClientForResource(resource string, coreClient kubernetes.Interface) (cache.Getter, error) {
	if resource == constants.SecretProviderClassController {
		csiClient, err := kube.GetCSIClient()
		if err != nil {
			return nil, fmt.Errorf("failed to get CSI client: %w", err)
		}
		return csiClient.SecretsstoreV1().RESTClient(), nil
	}
	return coreClient.CoreV1().RESTClient(), nil
}
