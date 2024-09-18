package callbacks_test

import (
	"context"
	"testing"
	"time"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	argorollouts "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/fake"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"

	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/kube"
)

var (
	clients = kube.Clients{ArgoRolloutClient: argorollouts.NewSimpleClientset()}
)

// TestUpdateRollout test update rollout strategy annotation
func TestUpdateRollout(t *testing.T) {
	namespace := "test-ns"

	cases := map[string]struct {
		name      string
		strategy  string
		isRestart bool
	}{
		"test-without-strategy": {
			name:      "defaults to rollout strategy",
			strategy:  "",
			isRestart: false,
		},
		"test-with-restart-strategy": {
			name:      "triggers a restart strategy",
			strategy:  "restart",
			isRestart: true,
		},
		"test-with-rollout-strategy": {
			name:      "triggers a rollout strategy",
			strategy:  "rollout",
			isRestart: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rollout, err := testutil.CreateRollout(
				clients.ArgoRolloutClient, name, namespace,
				map[string]string{options.RolloutStrategyAnnotation: tc.strategy},
			)
			if err != nil {
				t.Errorf("creating rollout: %v", err)
			}
			modifiedChan := watchRollout(rollout.Name, namespace)

			err = callbacks.UpdateRollout(clients, namespace, rollout)
			if err != nil {
				t.Errorf("updating rollout: %v", err)
			}
			rollout, err = clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(
				namespace).Get(context.TODO(), rollout.Name, meta_v1.GetOptions{})

			if err != nil {
				t.Errorf("getting rollout: %v", err)
			}
			if isRestartStrategy(rollout) == tc.isRestart {
				t.Errorf("Should not be a restart strategy")
			}
			select {
			case <-modifiedChan:
				// object has been modified
			case <-time.After(1 * time.Second):
				t.Errorf("Rollout has not been updated")
			}
		})
	}
}

func isRestartStrategy(rollout *argorolloutv1alpha1.Rollout) bool {
	return rollout.Spec.RestartAt == nil
}

func watchRollout(name, namespace string) chan interface{} {
	timeOut := int64(1)
	modifiedChan := make(chan interface{})
	watcher, _ := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Watch(context.Background(), meta_v1.ListOptions{TimeoutSeconds: &timeOut})
	go watchModified(watcher, name, modifiedChan)
	return modifiedChan
}

func watchModified(watcher watch.Interface, name string, modifiedChan chan interface{}) {
	for event := range watcher.ResultChan() {
		item := event.Object.(*argorolloutv1alpha1.Rollout)
		if item.Name == name {
			switch event.Type {
			case watch.Modified:
				modifiedChan <- nil
			}
			return
		}
	}
}
