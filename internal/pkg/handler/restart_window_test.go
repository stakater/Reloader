package handler

import (
	"context"
	"testing"

	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/restartwindow"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stakater/Reloader/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRestartWindowGateAndDefault(t *testing.T) {
	old := options.EnableRestartWindows
	defer func() { options.EnableRestartWindows = old }()
	for _, tc := range []struct {
		name                       string
		window, enabled, wantError bool
	}{
		{"unannotated-default", false, false, false}, {"window-enabled", true, true, false}, {"window-disabled-blocks", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options.EnableRestartWindows = tc.enabled
			anns := map[string]string{"secret.reloader.stakater.com/reload": "tls"}
			if tc.window {
				anns[restartwindow.PolicyAnnotation] = `{"timezone":"UTC","windows":[{"cron":"0 2 * * *","duration":"1h"}]}`
			}
			d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test", UID: "original", Annotations: anns}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}}}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "test"}, Data: map[string][]byte{"tls.crt": []byte("test")}}
			c := fake.NewClientset(d, secret)
			// Suppress post-action metric use by having the default-path strategy report no change.
			called := false
			strategy := func(f callbacks.RollingUpgradeFuncs, o runtime.Object, c common.Config, a bool) InvokeStrategyResult {
				called = true
				return InvokeStrategyResult{Result: constants.NotUpdated}
			}
			_, err := upgradeResource(kube.Clients{KubernetesClient: c}, common.GetSecretConfig(secret), GetDeploymentRollingUpgradeFuncs(), metrics.NewCollectors(), nil, strategy, d, false)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if called == tc.window {
				t.Fatalf("upstream strategy called=%v", called)
			}
			current, _ := c.AppsV1().Deployments("test").Get(context.Background(), "app", metav1.GetOptions{})
			if tc.window && tc.enabled && current.Annotations[restartwindow.PendingAnnotation] == "" {
				t.Fatal("change not persisted")
			}
			if current.Spec.Template.Annotations[restartwindow.RestartAnnotation] != "" {
				t.Fatal("gate started rollout")
			}
		})
	}

}
