package controller

import (
	"time"
	"fmt"
	
	
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	errorHandler "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/pkg/kube"
	"github.com/stakater/Reloader/internal/pkg/upgrader"
)

// Event indicate the informerEvent
type Event struct {
	key          string
	eventType    string
}

// Controller for checking events
type Controller struct {
	client        kubernetes.Interface
	indexer       cache.Indexer
	queue         workqueue.RateLimitingInterface
	informer      cache.Controller
	resource	  string
	namespace     string
}

// NewController for initializing a Controller
func NewController(
	client kubernetes.Interface, resource string, namespace string) (*Controller, error) {

	c := Controller{
		client:   client,
		resource: resource,
		namespace: namespace,
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	listWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), resource, namespace, fields.Everything())
	
	indexer, informer := cache.NewIndexerInformer(listWatcher, kube.ResourceMap[resource], 0, cache.ResourceEventHandlerFuncs {
		AddFunc:    c.Add,
		UpdateFunc: c.Update,
	}, cache.Indexers{})
	c.indexer = indexer
	c.informer = informer
	c.queue = queue
	return &c, nil
}

// Add function to add a 'create' event to the queue in case of creating a pod
func (c *Controller) Add(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	var event Event

	if err == nil {
		event.key = key
		event.eventType = "create"
		c.queue.Add(event)
	}
}

// Update function to add an 'update' event to the queue in case of updating a pod
func (c *Controller) Update(old interface{}, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	var event Event

	if err == nil {
		event.key = key
		event.eventType = "update"
		c.queue.Add(event)
	}
}

//Run function for controller which handles the queue
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {

	logrus.Infof("Starting Controller for type ", c.resource)
	defer errorHandler.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		errorHandler.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	logrus.Infof("Stopping Controller for type ", c.resource)
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	event, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two events with the same key are never processed in
	// parallel.
	defer c.queue.Done(event)

	// Invoke the method containing the business logic
	err := c.takeAction(event.(Event))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, event)
	return true
}

// main business logic that acts bassed on the event or key
func (c *Controller) takeAction(event Event) error {

	obj, _, err := c.indexer.GetByKey(event.key)
	if err != nil {
		logrus.Infof("Fetching object with key %s from store failed with %v", event.key, err)
	}
	if obj == nil {
		logrus.Infof("Error in Action")
	} else {
		logrus.Infof("Detected changes in object %s", obj)
		// process events based on its type
		logrus.Infof("Performing '%s' action for controller of type '%s'", event.eventType, c.resource)
		u, _ := upgrader.NewUpgrader(c.client, c.resource)
		if c.resource == "configMaps" {
			switch event.eventType {
				case "create":
					u.ObjectCreated(obj)
				case "update":
					u.ObjectUpdated(obj)
			}
		}
	}
	return nil
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
		logrus.Infof("Error syncing events %v: %v", key, err)

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