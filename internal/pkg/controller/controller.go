package controller

import (
	"bytes"
	"strings"
	"time"
	"fmt"

	"github.com/stakater/Reloader/internal/pkg/actions"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"k8s.io/kubernetes/pkg/api"

	clientset "k8s.io/client-go/kubernetes"
	informerruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/apimachinery/pkg/util/wait"
	errorHandler "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"sort"
)

const (
	updateOnChangeAnnotation = "configmap.fabric8.io/update-on-change"
	// AllNamespaces as our controller will be looking for events in all namespaces
	AllNamespaces = ""
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
	clientset        clientset.Interface
	indexer          cache.Indexer
	queue            workqueue.RateLimitingInterface
	informer         cache.Controller
	controllerConfig config.Controller
	Actions          []actions.Action

	stopCh chan struct{}
}

// NewController for initializing a Controller
func NewController(
	clientset clientset.Interface,
	resyncPeriod time.Duration, controllerConfig config.Controller, objType informerruntime.Object) (*Controller, error) {

	c := Controller{
		clientset:        clientset,
		controllerConfig: controllerConfig,
		stopCh: make(chan struct{}),
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	listWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), controllerConfig.Type, AllNamespaces, fields.Everything())

	indexer, informer := cache.NewIndexerInformer(listWatcher, objType, 0, cache.ResourceEventHandlerFuncs {
		AddFunc:    c.Add,
		UpdateFunc: c.Update,
		DeleteFunc: c.Delete,
	}, cache.Indexers{})

	/*c.cmLister.Store, c.cmController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  configMapListFunc(c.client, namespace),
			WatchFunc: configMapWatchFunc(c.client, namespace),
		},
		&api.ConfigMap{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				newCM := obj.(*api.ConfigMap)
				//typeOfMaster, err := util.TypeOfMaster(kubeClient)
				if err != nil {
					glog.Fatalf("failed to create REST client config: %s", err)
				}
				err = rollingUpgradeDeployments(newCM, kubeClient)
				if err != nil {
					glog.Errorf("failed to update Deployment: %v", err)
				}

			},
			UpdateFunc: func(oldObj interface{}, newObj interface{}) {
				oldM := oldObj.(*api.ConfigMap)
				newCM := newObj.(*api.ConfigMap)

				if oldM.ResourceVersion != newCM.ResourceVersion {
					//typeOfMaster, err := util.TypeOfMaster(kubeClient)
					if err != nil {
						glog.Fatalf("failed to create REST client config: %s", err)
					}
					err = rollingUpgradeDeployments(newCM, kubeClient)
					if err != nil {
						glog.Errorf("failed to update Deployment: %v", err)
					}
				}
			},
		},
	)*/
	return &c, nil
}

// Run starts the controller.
/*func (c *Controller) Run() {
	glog.Infof("starting reloader")

	<-c.stopCh
}*/

// Stop stops the controller.
/*func (c *Controller) Stop() {
	glog.Infof("stopping reloader")

	close(c.stopCh)
}*/

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
		c.queue.Add(event)
	}
}

// Delete function to add a 'delete' event to the queue in case of deleting a pod
func (c *Controller) Delete(obj interface{}) {
	//In current scenario, we dont need to do anything when a pod is deleted so it is empty now
}

//Run function for controller which handles the queue
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {

	glog.Infof("Starting Controller for type ", c.controllerConfig.Type)
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
	glog.Infof("Stopping Controller for type ", c.controllerConfig.Type)
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
		glog.Infof("Fetching object with key %s from store failed with %v", event.key, err)
	}
	if obj == nil {
		glog.Infof("Error in Action")
	} else {
		glog.Infof("Detected changes in object %s", obj)
		/*glog.Infof("Resource block not found, performing actions")
		// process events based on its type
		for index, action := range c.Actions {
			glog.Infof("Performing '%s' action for controller of type '%s'", c.controllerConfig.Actions[index].Name, c.controllerConfig.Type)
			switch event.eventType {
			case "create":
				action.ObjectCreated(obj)
			case "update":
				//TODO: Figure how to pass old and new object
				action.ObjectUpdated(obj, nil)
			case "delete":
				action.ObjectDeleted(obj)
			}
		}*/
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
		log.Printf("Error syncing events %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	log.Printf("Dropping the key %q out of the queue: %v", key, err)
}

/*func configMapListFunc(c *client.Client, ns string) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return c.ConfigMaps(ns).List(opts)
	}
}

func configMapWatchFunc(c *client.Client, ns string) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		return c.ConfigMaps(ns).Watch(options)
	}
}

func rollingUpgradeDeployments(cm *api.ConfigMap, c *client.Client) error {
	ns := cm.Namespace
	configMapName := cm.Name
	configMapVersion := convertConfigMapToToken(cm)

	deployments, err := c.Deployments(ns).List(api.ListOptions{})
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
				_, err := c.Deployments(ns).Update(&d)
				if err != nil {
					return errors.Wrap(err, "update deployment failed")
				}
				glog.Infof("Updated Deployment %s", d.Name)
			}
		}
	}
	return nil
}*/

// lets convert the configmap into a unique token based on the data values
func convertConfigMapToToken(cm *api.ConfigMap) string {
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

func updateContainers(containers []api.Container, annotationValue, configMapVersion string) bool {
	// we can have multiple configmaps to update
	answer := false
	configmaps := strings.Split(annotationValue, ",")
	for _, cmNameToUpdate := range configmaps {
		configmapEnvar := "FABRIC8_" + convertToEnvVarName(cmNameToUpdate) + "_CONFIGMAP"

		for i := range containers {
			envs := containers[i].Env
			matched := false
			for j := range envs {
				if envs[j].Name == configmapEnvar {
					matched = true
					if envs[j].Value != configMapVersion {
						glog.Infof("Updating %s to %s", configmapEnvar, configMapVersion)
						envs[j].Value = configMapVersion
						answer = true
					}
				}
			}
			// if no existing env var exists lets create one
			if !matched {
				e := api.EnvVar{
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
