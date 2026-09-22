package controller

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	alert "github.com/stakater/Reloader/internal/pkg/alerts"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/restartwindow"
	"github.com/stakater/Reloader/internal/pkg/util"
)

var sendWindowAlert = alert.SendWebhookAlert

// NewRestartWindowController shares the normal controller/leader lifecycle.
func NewRestartWindowController(client kubernetes.Interface, namespace string, ignored util.List, namespaceSelector, resourceSelector string, recorder record.EventRecorder, collectors metrics.Collectors) *Controller {
	return &Controller{windowRunner: func(stop chan struct{}) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if recorder == nil {
			broadcaster := record.NewBroadcaster()
			defer broadcaster.Shutdown()
			broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
			recorder = broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "reloader-restart-windows"})
		}
		go func() {
			select {
			case <-stop:
				cancel()
			case <-ctx.Done():
			}
		}()
		queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[restartwindow.Target]())
		defer queue.ShutDown()
		selector, err := labels.Parse(resourceSelector)
		if err != nil {
			logrus.Error(err)
			return
		}
		nsSelector, err := labels.Parse(namespaceSelector)
		if err != nil {
			logrus.Error(err)
			return
		}
		factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace(namespace))
		informersByKind := map[string]cache.SharedIndexInformer{
			"Deployment":  factory.Apps().V1().Deployments().Informer(),
			"StatefulSet": factory.Apps().V1().StatefulSets().Informer(),
			"DaemonSet":   factory.Apps().V1().DaemonSets().Informer(),
		}
		blockedReasons := map[restartwindow.Target]string{}
		for kind, informer := range informersByKind {
			add := func(obj any) {
				m, e := meta.Accessor(obj)
				if e == nil && m.GetAnnotations()[restartwindow.PendingAnnotation] != "" {
					queue.Add(restartwindow.Target{Kind: kind, Namespace: m.GetNamespace(), Name: m.GetName()})
				}
			}
			_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: add, UpdateFunc: func(_, obj any) { add(obj) }})
			if err != nil {
				logrus.Error(err)
				return
			}
		}
		factory.Start(stop)
		defer factory.Shutdown()
		for _, synced := range factory.WaitForCacheSync(stop) {
			if !synced {
				return
			}
		}
		var workers sync.WaitGroup
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				target, quit := queue.Get()
				if quit {
					return
				}
				func() {
					defer queue.Done(target)
					if ctx.Err() != nil {
						return
					}
					if ignored.Contains(target.Namespace) {
						queue.Forget(target)
						return
					}
					informer := informersByKind[target.Kind]
					cached, exists, cacheErr := informer.GetStore().GetByKey(target.Namespace + "/" + target.Name)
					if cacheErr != nil || !exists {
						queue.Forget(target)
						delete(blockedReasons, target)
						return
					}
					cachedObject, ok := cached.(runtime.Object)
					if !ok {
						queue.Forget(target)
						logrus.Errorf("Cached %s %s/%s does not implement runtime.Object", target.Kind, target.Namespace, target.Name)
						return
					}
					delay, pending, preflightErr := restartwindow.Preflight(cachedObject, time.Now())
					if preflightErr != nil {
						message := preflightErr.Error()
						logrus.WithError(preflightErr).Warnf("Restart window reconciliation blocked for %s %s/%s", target.Kind, target.Namespace, target.Name)
						if recorder != nil && blockedReasons[target] != message {
							recorder.Event(&corev1.ObjectReference{Kind: target.Kind, APIVersion: "apps/v1", Namespace: target.Namespace, Name: target.Name}, corev1.EventTypeWarning, "RestartWindowBlocked", message)
						}
						blockedReasons[target] = message
						queue.AddRateLimited(target)
						return
					}
					if !pending {
						queue.Forget(target)
						delete(blockedReasons, target)
						return
					}
					if delay > 0 {
						queue.Forget(target)
						delete(blockedReasons, target)
						queue.AddAfter(target, delay)
						return
					}
					if namespaceSelector != "" {
						ns, e := client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
						if e != nil {
							queue.AddAfter(target, time.Minute)
							return
						}
						if !nsSelector.Matches(labels.Set(ns.Labels)) {
							queue.AddAfter(target, time.Minute)
							return
						}
					}
					allowed := func(kind string) bool {
						name := "configmaps"
						if kind == constants.SecretEnvVarPostfix {
							name = "secrets"
						}
						return !slices.Contains(options.ResourcesToIgnore, name)
					}
					actionStart := time.Now()
					result, e := restartwindow.Reconcile(ctx, client, target, time.Now, selector, allowed, recorder)
					if e != nil {
						logrus.WithError(e).Warnf("Restart window reconciliation failed for %s %s/%s", target.Kind, target.Namespace, target.Name)
						message := e.Error()
						if recorder != nil && blockedReasons[target] != message {
							recorder.Event(&corev1.ObjectReference{Kind: target.Kind, APIVersion: "apps/v1", Namespace: target.Namespace, Name: target.Name}, corev1.EventTypeWarning, "RestartWindowBlocked", message)
						}
						blockedReasons[target] = message
						if result.Attempted {
							collectors.RecordReload(false, target.Namespace)
							collectors.RecordAction(target.Kind, "error", time.Since(actionStart))
						}
						queue.AddRateLimited(target)
						return
					}
					queue.Forget(target)
					delete(blockedReasons, target)
					if result.Executed {
						collectors.RecordReload(true, target.Namespace)
						collectors.RecordAction(target.Kind, "success", time.Since(actionStart))
						if os.Getenv("ALERT_ON_RELOAD") == "true" {
							sendWindowAlert(fmt.Sprintf("Reloader applied pending configuration changes and reloaded *%s* of type *%s* in namespace *%s*", target.Name, target.Kind, target.Namespace))
						}
					}
					if result.RequeueAfter > 0 {
						queue.AddAfter(target, result.RequeueAfter)
					}
				}()
			}
		}()
		<-stop
		cancel()
		queue.ShutDown()
		workers.Wait()
	}}
}
