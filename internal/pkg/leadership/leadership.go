package leadership

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/stakater/Reloader/internal/pkg/controller"

	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
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
func RunLeaderElection(lock *resourcelock.LeaseLock, ctx context.Context, cancel context.CancelFunc, id string, controllers []*controller.Controller) <-chan struct{} {
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		var stopChannels []chan struct{}
		for range controllers {
			stopChannels = append(stopChannels, make(chan struct{}))
		}

		// controllerWg tracks the controller.Run goroutines so that
		// OnStoppedLeading can wait for them to fully exit before returning.
		var controllerWg sync.WaitGroup

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   15 * time.Second,
			RenewDeadline:   10 * time.Second,
			RetryPeriod:     2 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(c context.Context) {
					m.Lock()
					healthy = true
					m.Unlock()
					logrus.Info("became leader, starting controllers")
					for i, ctrl := range controllers {
						controllerWg.Add(1)
						go func(ctrl *controller.Controller, stopCh chan struct{}) {
							defer controllerWg.Done()
							ctrl.Run(1, stopCh)
						}(ctrl, stopChannels[i])
					}
				},
				OnStoppedLeading: func() {
					logrus.Info("no longer leader, shutting down")
					stopControllers(stopChannels)
					// Wait for all controller.Run goroutines to fully exit.
					// controller.Run blocks until its informer and workers exit,
					// so this guarantees no controller goroutine is still running
					// when OnStoppedLeading returns.
					logrus.Info("waiting for all controller goroutines to exit")
					controllerWg.Wait()
					logrus.Info("all controller goroutines exited")
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
	http.HandleFunc("/live", healthz)
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
