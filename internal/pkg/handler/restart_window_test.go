package handler

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/restartwindow"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stakater/Reloader/pkg/kube"
)

func TestRestartWindowGateAndDefault(t *testing.T) {
	old := options.EnableRestartWindows
	defer func() { options.EnableRestartWindows = old }()
	for _, tc := range []struct {
		name                            string
		window, enabled, strategyCalled bool
		pending                         bool
	}{
		{"unannotated-default", false, false, true, false},
		{"window-enabled", true, true, false, true},
		{"window-disabled-skips-only-windowed-workload", true, false, false, false},
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
			if err != nil {
				t.Fatalf("error=%v", err)
			}
			if called != tc.strategyCalled {
				t.Fatalf("upstream strategy called=%v, want %v", called, tc.strategyCalled)
			}
			current, _ := c.AppsV1().Deployments("test").Get(context.Background(), "app", metav1.GetOptions{})
			if tc.pending && current.Annotations[restartwindow.PendingAnnotation] == "" {
				t.Fatal("change not persisted")
			}
			if current.Spec.Template.Annotations[restartwindow.RestartAnnotation] != "" {
				t.Fatal("gate started rollout")
			}
		})
	}

}

func TestPerformActionContinuesAfterInvalidWindowWorkload(t *testing.T) {
	old := options.EnableRestartWindows
	options.EnableRestartWindows = true
	defer func() { options.EnableRestartWindows = old }()

	policy := `{"timezone":"UTC","windows":[{"cron":"0 2 * * *","duration":"1h"}]}`
	annotations := map[string]string{"secret.reloader.stakater.com/reload": "tls"}
	invalid := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "invalid", Namespace: "test", UID: "invalid", Annotations: annotations},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{restartwindow.PolicyAnnotation: policy}},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
		}},
	}
	valid := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "valid", Namespace: "test", UID: "valid", Annotations: annotations},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}},
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "test"}, Data: map[string][]byte{"tls.crt": []byte("test")}}
	client := fake.NewClientset(invalid, valid, secret)
	called := 0
	strategy := func(callbacks.RollingUpgradeFuncs, runtime.Object, common.Config, bool) InvokeStrategyResult {
		called++
		return InvokeStrategyResult{Result: constants.NotUpdated}
	}
	if err := PerformAction(kube.Clients{KubernetesClient: client}, common.GetSecretConfig(secret), GetDeploymentRollingUpgradeFuncs(), metrics.NewCollectors(), nil, strategy); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("normal workload strategy calls=%d, want 1", called)
	}
}

func TestPendingStateWithoutPolicyDoesNotBlockWhenWindowsDisabled(t *testing.T) {
	old := options.EnableRestartWindows
	options.EnableRestartWindows = false
	defer func() { options.EnableRestartWindows = old }()

	annotations := map[string]string{
		"secret.reloader.stakater.com/reload": "tls",
		restartwindow.PendingAnnotation:       `{"SECRET/tls":{"type":"SECRET","name":"tls","observedHash":"old"}}`,
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test", UID: "app", Annotations: annotations},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}},
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "test"}, Data: map[string][]byte{"tls.crt": []byte("test")}}
	client := fake.NewClientset(deployment, secret)
	called := false
	strategy := func(callbacks.RollingUpgradeFuncs, runtime.Object, common.Config, bool) InvokeStrategyResult {
		called = true
		return InvokeStrategyResult{Result: constants.NotUpdated}
	}
	_, err := upgradeResource(kube.Clients{KubernetesClient: client}, common.GetSecretConfig(secret), GetDeploymentRollingUpgradeFuncs(), metrics.NewCollectors(), nil, strategy, deployment, false)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("leftover pending state blocked the normal reload path")
	}
}
