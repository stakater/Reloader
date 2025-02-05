package controller

import (
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
	"k8s.io/utils/strings/slices"
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

	getterRESTClient := client.CoreV1().RESTClient()
	if resource == "secretproviderclasspodstatuses" {
		csiClient, err := kube.GetCSIClient()
		if err != nil {
			logrus.Fatal(err)
		}
		getterRESTClient = csiClient.SecretsstoreV1().RESTClient()
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

	switch object := obj.(type) {
	case *v1.Namespace:
		c.addSelectedNamespaceToCache(*object)
		return
	case *csiv1.SecretProviderClassPodStatus:
		return
	}

	if options.ReloadOnCreate == "true" {
		if !c.resourceInIgnoredNamespace(obj) && c.resourceInSelectedNamespaces(obj) && secretControllerInitialized && configmapControllerInitialized {
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
	case *csiv1.SecretProviderClassPodStatus:
		return c.ignoredNamespaces.Contains(object.ObjectMeta.Namespace)
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
	switch new.(type) {
	case *v1.Namespace:
		return
	}

	if !c.resourceInIgnoredNamespace(new) && c.resourceInSelectedNamespaces(new) {
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
	switch object := old.(type) {
	case *v1.Namespace:
		c.removeSelectedNamespaceFromCache(*object)
		return
	case *csiv1.SecretProviderClassPodStatus:
		return
	}

	if options.ReloadOnDelete == "true" {
		if !c.resourceInIgnoredNamespace(old) && c.resourceInSelectedNamespaces(old) && secretControllerInitialized && configmapControllerInitialized {
			c.queue.Add(handler.ResourceDeleteHandler{
				Resource:   old,
				Collectors: c.collectors,
				Recorder:   c.recorder,
			})
		}
	}
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
	if c.resource == "secrets" {
		secretControllerInitialized = true
	} else if c.resource == "configMaps" {
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
	logrus.Errorf("Dropping key out of the queue: %v", err)
	logrus.Debugf("Dropping the key %q out of the queue: %v", key, err)
}
