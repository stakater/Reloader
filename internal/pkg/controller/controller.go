package controller

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	errorHandler "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)
// ResourceUpdated contains new or updated objects
type ResourceUpdated struct {
	newObj interface{}
	oldObj interface{}
}

// Controller for checking events
type Controller struct {
	client    kubernetes.Interface
	indexer   cache.Indexer
	queue     workqueue.RateLimitingInterface
	informer  cache.Controller
	resource  string
	namespace string
}

// NewController for initializing a Controller
func NewController(
	client kubernetes.Interface, resource string, namespace string) (*Controller, error) {

	c := Controller{
		client:    client,
		resource:  resource,
		namespace: namespace,
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	listWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), resource, namespace, fields.Everything())

	indexer, informer := cache.NewIndexerInformer(listWatcher, kube.ResourceMap[resource], 0, cache.ResourceEventHandlerFuncs{
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
	c.queue.Add(ResourceUpdated{
		newObj: obj,
	})
}

// Update function to add an 'update' event to the queue in case of updating a pod
func (c *Controller) Update(old interface{}, new interface{}) {
	c.queue.Add(ResourceUpdated{
		newObj: new,
		oldObj: old,
	})
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
	resourceUpdated, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two events with the same key are never processed in
	// parallel.
	defer c.queue.Done(resourceUpdated)

	// Invoke the method containing the business logic
	err := c.takeAction(resourceUpdated.(ResourceUpdated))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, resourceUpdated)
	return true
}

// main business logic that acts bassed on the event or key
func (c *Controller) takeAction(resourceUpdated ResourceUpdated) error {

	newObj := resourceUpdated.newObj
	oldObj := resourceUpdated.oldObj
	if newObj == nil {
		logrus.Infof("Error in Action")
	} else {
		logrus.Infof("Detected changes in object %s", newObj)
		// process events based on its type
		if(oldObj == nil){
			logrus.Infof("Performing 'Added' action for controller of type '%s'", c.resource)
		} else {
			logrus.Infof("Performing 'Updated' action for controller of type '%s'", c.resource)
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
