package controller

import (
	"time"
	"fmt"
	"strings"
	"bytes"
	"sort"
	
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/util/runtime"
	informerruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	errorHandler "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"github.com/sirupsen/logrus"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	updateOnChangeAnnotation = "reloader.stakater.com/update-on-change"
	// AllNamespaces as our controller will be looking for events in all namespaces
	AllNamespaces = "temp-reloader"
)

// Event indicate the informerEvent
type Event struct {
	key          string
	eventType    string
	namespace    string
	resourceType string
}

// Controller for checking events
type Controller struct {
	client        kubernetes.Interface
	indexer       cache.Indexer
	queue         workqueue.RateLimitingInterface
	informer      cache.Controller
	resource	  string

	stopCh chan struct{}
}

// NewController for initializing a Controller
func NewController(
	client kubernetes.Interface,
	resource string, objType informerruntime.Object) (*Controller, error) {

	c := Controller{
		client:   client,
		resource: resource,
		stopCh:   make(chan struct{}),
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	listWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), resource, AllNamespaces, fields.Everything())

	indexer, informer := cache.NewIndexerInformer(listWatcher, objType, 0, cache.ResourceEventHandlerFuncs {
		AddFunc:    c.Add,
		UpdateFunc: c.Update,
		DeleteFunc: c.Delete,
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

// Delete function to add a 'delete' event to the queue in case of deleting a pod
func (c *Controller) Delete(obj interface{}) {
	//In current scenario, we dont need to do anything when a pod is deleted so it is empty now
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
		if c.resource == "configMaps" {
			switch event.eventType {
				case "create":
					ObjectCreated(obj, c.client)
				case "update":
					ObjectUpdated(obj, c.client)
				case "delete":
					ObjectDeleted(obj)
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

// ObjectCreated Do nothing for default handler
func ObjectCreated(obj interface{}, client kubernetes.Interface) {
	message := "Configmap: `" + obj.(*v1.ConfigMap).Name + "`has been created in Namespace: `" + obj.(*v1.ConfigMap).Namespace + "`"
	logrus.Infof(message)
	err := rollingUpgradeDeployments(obj, client)
	if err != nil {
		logrus.Errorf("failed to update Deployment: %v", err)
	}
}

// ObjectDeleted Do nothing for default handler
func ObjectDeleted(obj interface{}) {

}

// ObjectUpdated Do nothing for default handler
func ObjectUpdated(oldObj interface{}, client kubernetes.Interface) {
	message := "Configmap: `" + oldObj.(*v1.ConfigMap).Name + "`has been updated in Namespace: `" + oldObj.(*v1.ConfigMap).Namespace + "`"
	logrus.Infof(message)
	err := rollingUpgradeDeployments(oldObj, client)
	if err != nil {
		logrus.Errorf("failed to update Deployment: %v", err)
	}
}

// Implementation has been borrowed from fabric8io/configmapcontroller
// Method has been modified a little to use updated liberaries.
func rollingUpgradeDeployments(oldObj interface{}, client kubernetes.Interface) error {
	ns := oldObj.(*v1.ConfigMap).Namespace
	configMapName := oldObj.(*v1.ConfigMap).Name
	configMapVersion := convertConfigMapToToken(oldObj.(*v1.ConfigMap))

	deployments, err := client.Apps().Deployments(ns).List(meta_v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list deployments")
	}
	for _, d := range deployments.Items {
		containers := d.Spec.Template.Spec.Containers
		// match deployments with the correct annotation
		annotationValue, _ := d.ObjectMeta.Annotations[updateOnChangeAnnotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == configMapName {
					matches = true
					break
				}
			}
			if matches {
				updateContainers(containers, annotationValue, configMapVersion)

				// update the deployment
				_, err := client.Apps().Deployments(ns).Update(&d)
				if err != nil {
					return errors.Wrap(err, "update deployment failed")
				}
				logrus.Infof("Updated Deployment %s", d.Name)
			}
		}
	}
	return nil
}

func updateContainers(containers []v1.Container, annotationValue, configMapVersion string) bool {
	// we can have multiple configmaps to update
	answer := false
	configmaps := strings.Split(annotationValue, ",")
	for _, cmNameToUpdate := range configmaps {
		configmapEnvar := "STAKATER_" + convertToEnvVarName(cmNameToUpdate) + "_CONFIGMAP"

		for i := range containers {
			envs := containers[i].Env
			matched := false
			for j := range envs {
				if envs[j].Name == configmapEnvar {
					matched = true
					if envs[j].Value != configMapVersion {
						logrus.Infof("Updating %s to %s", configmapEnvar, configMapVersion)
						envs[j].Value = configMapVersion
						answer = true
					}
				}
			}
			// if no existing env var exists lets create one
			if !matched {
				e := v1.EnvVar{
					Name:  configmapEnvar,
					Value: configMapVersion,
				}
				containers[i].Env = append(containers[i].Env, e)
				answer = true
			}
		}
	}
	return answer
}

// convertToEnvVarName converts the given text into a usable env var
// removing any special chars with '_'
func convertToEnvVarName(text string) string {
	var buffer bytes.Buffer
	lower := strings.ToUpper(text)
	lastCharValid := false
	for i := 0; i < len(lower); i++ {
		ch := lower[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			buffer.WriteString(string(ch))
			lastCharValid = true
		} else {
			if lastCharValid {
				buffer.WriteString("_")
			}
			lastCharValid = false
		}
	}
	return buffer.String()
}

// lets convert the configmap into a unique token based on the data values
func convertConfigMapToToken(cm *v1.ConfigMap) string {
	values := []string{}
	for k, v := range cm.Data {
		values = append(values, k+"="+v)
	}
	sort.Strings(values)
	text := strings.Join(values, ";")
	// we could zip and base64 encode
	// but for now we could leave this easy to read so that its easier to diagnose when & why things changed
	return text
}