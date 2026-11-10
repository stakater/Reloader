package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// pendingReloadSource feeds workloads that record a pending reload into a controller
// through a channel. It registers on the workload informers without waiting for them to
// sync, unlike a regular watch: a workload kind whose CRD or RBAC is missing then only
// loses deferred reloads, instead of failing the controller and stopping Reloader for
// everyone, including clusters that never use sync windows.
type pendingReloadSource struct {
	informers cache.Informers
	objects   []client.Object
	predicate predicate.Predicate
	events    chan<- event.GenericEvent
	log       logr.Logger
}

// Start registers the event handlers and blocks until ctx is done. Registering replays
// every existing workload as an add, which is how deferred reloads resume after a restart
// or a change of leader. It only runs on the leader, like the controllers.
func (s *pendingReloadSource) Start(ctx context.Context) error {
	for _, obj := range s.objects {
		informer, err := s.informers.GetInformer(ctx, obj, cache.BlockUntilSynced(false))
		if err != nil {
			s.log.Error(err, "cannot watch workloads for deferred reloads; their sync windows are not resumed", "kind", fmt.Sprintf("%T", obj))
			continue
		}
		if _, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
			AddFunc: func(o any) {
				if obj, ok := o.(client.Object); ok && s.predicate.Create(event.CreateEvent{Object: obj}) {
					s.send(ctx, obj)
				}
			},
			UpdateFunc: func(oldObj, newObj any) {
				o, okOld := oldObj.(client.Object)
				n, okNew := newObj.(client.Object)
				if okOld && okNew && s.predicate.Update(event.UpdateEvent{ObjectOld: o, ObjectNew: n}) {
					s.send(ctx, n)
				}
			},
		}); err != nil {
			s.log.Error(err, "cannot watch workloads for deferred reloads; their sync windows are not resumed", "kind", fmt.Sprintf("%T", obj))
		}
	}
	<-ctx.Done()
	return nil
}

func (s *pendingReloadSource) send(ctx context.Context, obj client.Object) {
	select {
	case s.events <- event.GenericEvent{Object: obj}:
	case <-ctx.Done():
	}
}
