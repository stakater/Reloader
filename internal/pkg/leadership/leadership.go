package leadership

import (
	"context"
	"net/http"
	"sync"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/handler"
	"github.com/stakater/Reloader/internal/pkg/options"
)

var (
	// Used for liveness probe
	m       sync.Mutex
	healthy bool = true
)

func GetNewLock(client coordinationv1.CoordinationV1Interface, lockName, podname, namespace string) *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: v1.ObjectMeta{
			Name:      lockName,
			Namespace: namespace,
		},
		Client: client,
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podname,
		},
	}
}

// RunLeaderElection runs leadership election in a background goroutine and
// returns a channel that is closed once the goroutine has fully exited
// (i.e., OnStoppedLeading has run and all controller goroutines have returned).
// Any optional onStarted callbacks are launched when leadership is acquired with
// a context that is cancelled upon losing leadership.
func RunLeaderElection(lock *resourcelock.LeaseLock, ctx context.Context, cancel context.CancelFunc, id string, controllers []*controller.Controller, onStarted ...func(context.Context)) <-chan struct{} {
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		var stopChannels []chan struct{}
		for range controllers {
			stopChannels = append(stopChannels, make(chan struct{}))
		}

		leadCtx, leadCancel := context.WithCancel(ctx)
		defer leadCancel()

		// controllerWg tracks controller.Run and onStarted goroutines; wgMu and
		// stoppedLead prevent OnStartedLeading from calling Add after Wait starts.
		var (
			controllerWg sync.WaitGroup
			wgMu         sync.Mutex
			stoppedLead  bool
		)

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   options.LeaderElectionLeaseDuration,
			RenewDeadline:   options.LeaderElectionRenewDeadline,
			RetryPeriod:     options.LeaderElectionRetryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(c context.Context) {
					m.Lock()
					healthy = true
					m.Unlock()
					logrus.Info("became leader, starting controllers")
					wgMu.Lock()
					if stoppedLead {
						wgMu.Unlock()
						return
					}
					for i, ctrl := range controllers {
						controllerWg.Add(1)
						go func(ctrl *controller.Controller, stopCh chan struct{}) {
							defer controllerWg.Done()
							ctrl.Run(1, stopCh)
						}(ctrl, stopChannels[i])
					}
					for _, fn := range onStarted {
						if fn == nil {
							continue
						}
						controllerWg.Add(1)
						go func(fn func(context.Context)) {
							defer controllerWg.Done()
							fn(leadCtx)
						}(fn)
					}
					wgMu.Unlock()
				},
				OnStoppedLeading: func() {
					logrus.Info("no longer leader, shutting down")
					wgMu.Lock()
					stoppedLead = true
					wgMu.Unlock()
					leadCancel()
					stopControllers(stopChannels)
					// Wait for all controller.Run goroutines to fully exit.
					// controller.Run blocks until its informer and workers exit,
					// so this guarantees no controller goroutine is still running
					// when OnStoppedLeading returns.
					logrus.Info("waiting for all controller goroutines to exit")
					controllerWg.Wait()
					logrus.Info("all controller goroutines exited")
					handler.StopResumeTimers()
					cancel()
					m.Lock()
					defer m.Unlock()
					healthy = false
				},
				OnNewLeader: func(current_id string) {
					if current_id == id {
						logrus.Info("still the leader!")
						return
					}
					logrus.Infof("new leader is %s", current_id)
				},
			},
		})
	}()

	return stopped
}

func stopControllers(stopChannels []chan struct{}) {
	for _, c := range stopChannels {
		close(c)
	}
}

// Healthz sets up the liveness probe endpoint. If leadership election is
// enabled and a replica stops leading the liveness probe will fail and the
// kubelet will restart the container.
func SetupLivenessEndpoint() {
	SetupLivenessEndpointWithMux(http.DefaultServeMux)
}

// SetupLivenessEndpointWithMux sets up the liveness probe endpoint on the provided ServeMux.
func SetupLivenessEndpointWithMux(mux *http.ServeMux) {
	if mux != nil {
		mux.HandleFunc("/live", healthz)
	} else {
		http.HandleFunc("/live", healthz)
	}
}

func healthz(w http.ResponseWriter, req *http.Request) {
	m.Lock()
	defer m.Unlock()
	if healthy {
		if i, err := w.Write([]byte("alive")); err != nil {
			logrus.Infof("failed to write liveness response, wrote: %d bytes, got err: %s", i, err)
		}
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
}
