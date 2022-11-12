package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
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
)

// Controller for checking events
type Controller struct {
	client            kubernetes.Interface
	indexer           cache.Indexer
	queue             workqueue.RateLimitingInterface
	informer          cache.Controller
	namespace         string
	ignoredNamespaces util.List
	collectors        metrics.Collectors
	recorder          record.EventRecorder
	namespaceSelector map[string]string
}

// controllerInitialized flag determines whether controlled is being initialized
var controllerInitialized bool = false

// NewController for initializing a Controller
func NewController(
	client kubernetes.Interface, resource string, namespace string, ignoredNamespaces []string, namespaceLabelSelector map[string]string, collectors metrics.Collectors) (*Controller, error) {

	c := Controller{
		client:            client,
		namespace:         namespace,
		ignoredNamespaces: ignoredNamespaces,
		namespaceSelector: namespaceLabelSelector,
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: client.CoreV1().Events(""),
	})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf("reloader-%s", resource)})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	listWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), resource, namespace, fields.Everything())

	indexer, informer := cache.NewIndexerInformer(listWatcher, kube.ResourceMap[resource], 0, cache.ResourceEventHandlerFuncs{
		AddFunc:    c.Add,
		UpdateFunc: c.Update,
		DeleteFunc: c.Delete,
	}, cache.Indexers{})
	c.indexer = indexer
	c.informer = informer
	c.queue = queue
	c.collectors = collectors
	c.recorder = recorder

	logrus.Infof("created controller for: %s", resource)
	return &c, nil
}

// Add function to add a new object to the queue in case of creating a resource
func (c *Controller) Add(obj interface{}) {
	if options.ReloadOnCreate == "true" {
		if !c.resourceInIgnoredNamespace(obj) && c.resourceInNamespaceSelector(obj) && controllerInitialized {
			c.queue.Add(handler.ResourceCreatedHandler{
				Resource:   obj,
				Collectors: c.collectors,
				Recorder:   c.recorder,
			})
		}
	}
}

func (c *Controller) resourceInIgnoredNamespace(raw interface{}) bool {
	switch object := raw.(type) {
	case *v1.ConfigMap:
		return c.ignoredNamespaces.Contains(object.ObjectMeta.Namespace)
	case *v1.Secret:
		return c.ignoredNamespaces.Contains(object.ObjectMeta.Namespace)
	}
	return false
}

func (c *Controller) resourceInNamespaceSelector(raw interface{}) bool {
	switch object := raw.(type) {
	case *v1.ConfigMap:
		namespace, _ := c.client.CoreV1().Namespaces().Get(context.Background(), object.ObjectMeta.Namespace, metav1.GetOptions{})
		for k, v := range c.namespaceSelector {
			if namespace.ObjectMeta.Labels[k] != v {
				return false
			}
		}
		return true
	case *v1.Secret:
		namespace, _ := c.client.CoreV1().Namespaces().Get(context.Background(), object.ObjectMeta.Namespace, metav1.GetOptions{})
		for k, v := range c.namespaceSelector {
			if namespace.ObjectMeta.Labels[k] != v {
				return false
			}
		}
		return true
	}
	return false
}

// Update function to add an old object and a new object to the queue in case of updating a resource
func (c *Controller) Update(old interface{}, new interface{}) {
	if !c.resourceInIgnoredNamespace(new) && c.resourceInNamespaceSelector(new) {
		c.queue.Add(handler.ResourceUpdatedHandler{
			Resource:    new,
			OldResource: old,
			Collectors:  c.collectors,
			Recorder:    c.recorder,
		})
	}
}

// Delete function to add an object to the queue in case of deleting a resource
func (c *Controller) Delete(old interface{}) {
	// Todo: Any future delete event can be handled here
}

// Run function for controller which handles the queue
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
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
	controllerInitialized = true

	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	resourceHandler, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two events with the same key are never processed in
	// parallel.
	defer c.queue.Done(resourceHandler)

	// Invoke the method containing the business logic
	err := resourceHandler.(handler.ResourceHandler).Handle()
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
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		logrus.Errorf("Error syncing events: %v", err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	logrus.Infof("Dropping the key %q out of the queue: %v", key, err)
}
