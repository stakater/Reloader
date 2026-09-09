// Copyright 2026 Doska. Licensed under the Apache License, Version 2.0.
package controller

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/restartwindow"
	"github.com/stakater/Reloader/internal/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// NewRestartWindowController shares the normal controller/leader lifecycle.
func NewRestartWindowController(client kubernetes.Interface, namespace string, ignored util.List, namespaceSelector, resourceSelector string, recorder record.EventRecorder) *Controller {
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
					delay, e := restartwindow.Reconcile(ctx, client, target, time.Now, selector, allowed, recorder)
					if e != nil {
						logrus.WithError(e).Warnf("Restart window reconciliation failed for %s %s/%s", target.Kind, target.Namespace, target.Name)
						if recorder != nil {
							recorder.Event(&corev1.ObjectReference{Kind: target.Kind, APIVersion: "apps/v1", Namespace: target.Namespace, Name: target.Name}, corev1.EventTypeWarning, "RestartWindowBlocked", fmt.Sprint(e))
						}
						delay = time.Minute
					}
					queue.Forget(target)
					// Revisit closed windows at least once a minute for clock changes and policy edits.
					if delay > time.Minute {
						delay = time.Minute
					}
					if delay > 0 {
						queue.AddAfter(target, delay)
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
