package leadership

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/controller"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

const healthPort string = ":9091"

var (
	// Used for liveness probe
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

// runLeaderElection runs leadership election. If an instance of the controller is the leader and stops leading it will shutdown.
func RunLeaderElection(lock *resourcelock.LeaseLock, ctx context.Context, cancel context.CancelFunc, id string, controllers []*controller.Controller) {
	// Construct channels for the controllers to use
	var stopChannels []chan struct{}
	for i := 0; i < len(controllers); i++ {
		stop := make(chan struct{})
		stopChannels = append(stopChannels, stop)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				logrus.Info("became leader, starting controllers")
				runControllers(controllers, stopChannels)
			},
			OnStoppedLeading: func() {
				logrus.Info("no longer leader, shutting down")
				stopControllers(stopChannels)
				cancel()
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
}

func runControllers(controllers []*controller.Controller, stopChannels []chan struct{}) {
	for i, c := range controllers {
		c := c
		go c.Run(1, stopChannels[i])
	}
}

func stopControllers(stopChannels []chan struct{}) {
	for _, c := range stopChannels {
		close(c)
	}
}

// Healthz serves the liveness probe endpoint. If leadership election is
// enabled and a replica stops leading the liveness probe will fail and the
// kubelet will restart the container.
func Healthz() error {
	http.HandleFunc("/live", healthz)
	return http.ListenAndServe(healthPort, nil)
}

func healthz(w http.ResponseWriter, req *http.Request) {
	if healthy {
		w.Write([]byte("alive"))
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
}
