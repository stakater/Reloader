package controller

import (
	"fmt"
	"slices"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/handler"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
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
)

// Controller for checking events
type Controller struct {
	client            kubernetes.Interface
	indexer           cache.Indexer
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

// controllerInitialized flag determines whether controlled is being initialized
var secretControllerInitialized bool = false
var configmapControllerInitialized bool = false
var selectedNamespacesCache []string

// NewController for initializing a Controller
func NewController(
	client kubernetes.Interface, resource string, namespace string, ignoredNamespaces []string, namespaceLabelSelector string, resourceLabelSelector string, collectors metrics.Collectors) (*Controller, error) {

	if options.SyncAfterRestart {
		secretControllerInitialized = true
		configmapControllerInitialized = true
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
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf("reloader-%s", resource)})

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]())

	optionsModifier := func(options *metav1.ListOptions) {
		if resource == "namespaces" {
			options.LabelSelector = c.namespaceSelector
		} else if len(c.resourceSelector) > 0 {
			options.LabelSelector = c.resourceSelector
		} else {
			options.FieldSelector = fields.Everything().String()
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
	// Record event received
	c.collectors.RecordEventReceived("add", c.resource)

	switch object := obj.(type) {
	case *v1.Namespace:
		c.addSelectedNamespaceToCache(*object)
		return
	case *csiv1.SecretProviderClassPodStatus:
		return
	}

	if options.ReloadOnCreate == "true" {
		if !c.resourceInIgnoredNamespace(obj) && c.resourceInSelectedNamespaces(obj) && secretControllerInitialized && configmapControllerInitialized {
			c.enqueue(handler.ResourceCreatedHandler{
				Resource:    obj,
				Collectors:  c.collectors,
				Recorder:    c.recorder,
				EnqueueTime: time.Now(), // Track when item was enqueued
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

	switch object := raw.(type) {
	case *v1.ConfigMap:
		if slices.Contains(selectedNamespacesCache, object.GetNamespace()) {
			return true
		}
	case *v1.Secret:
		if slices.Contains(selectedNamespacesCache, object.GetNamespace()) {
			return true
		}
	case *csiv1.SecretProviderClassPodStatus:
		if slices.Contains(selectedNamespacesCache, object.GetNamespace()) {
			return true
		}
	}
	return false
}

func (c *Controller) addSelectedNamespaceToCache(namespace v1.Namespace) {
	selectedNamespacesCache = append(selectedNamespacesCache, namespace.GetName())
	logrus.Infof("added namespace to be watched: %s", namespace.GetName())
}

func (c *Controller) removeSelectedNamespaceFromCache(namespace v1.Namespace) {
	for i, v := range selectedNamespacesCache {
		if v == namespace.GetName() {
			selectedNamespacesCache = append(selectedNamespacesCache[:i], selectedNamespacesCache[i+1:]...)
			logrus.Infof("removed namespace from watch: %s", namespace.GetName())
			return
		}
	}
}

// Update function to add an old object and a new object to the queue in case of updating a resource
func (c *Controller) Update(old interface{}, new interface{}) {
	// Record event received
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
			EnqueueTime: time.Now(), // Track when item was enqueued
		})
	} else {
		c.collectors.RecordSkipped("ignored_or_not_selected")
	}
}

// Delete function to add an object to the queue in case of deleting a resource
func (c *Controller) Delete(old interface{}) {
	// Record event received
	c.collectors.RecordEventReceived("delete", c.resource)

	if _, ok := old.(*csiv1.SecretProviderClassPodStatus); ok {
		return
	}

	if options.ReloadOnDelete == "true" {
		if !c.resourceInIgnoredNamespace(old) && c.resourceInSelectedNamespaces(old) && secretControllerInitialized && configmapControllerInitialized {
			c.enqueue(handler.ResourceDeleteHandler{
				Resource:    old,
				Collectors:  c.collectors,
				Recorder:    c.recorder,
				EnqueueTime: time.Now(), // Track when item was enqueued
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

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	logrus.Infof("Stopping Controller")
}

func (c *Controller) runWorker() {
	// At this point the controller is fully initialized and we can start processing the resources
	if c.resource == string(v1.ResourceSecrets) {
		secretControllerInitialized = true
	} else if c.resource == string(v1.ResourceConfigMaps) {
		configmapControllerInitialized = true
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

	// Update queue depth after getting item
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
	err := resourceHandler.(handler.ResourceHandler).Handle()

	duration := time.Since(startTime)

	// Record reconcile metrics
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

	// Record failed event processing
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
