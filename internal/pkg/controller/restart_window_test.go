package controller

import (
	"context"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/restartwindow"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"testing"
	"time"
)

func TestRestartWindowRecoversPendingOnStartup(t *testing.T) {
	// Every minute opens a 2-minute window, independent of wall-clock test time.
	policy := `{"timezone":"UTC","windows":[{"cron":"* * * * *","duration":"2m"}]}`
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test", UID: "original", Annotations: map[string]string{restartwindow.PolicyAnnotation: policy, "secret.reloader.stakater.com/reload": "tls"}}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "test"}, Data: map[string][]byte{"tls.crt": []byte("test-certificate")}}
	client := fake.NewClientset(d, secret)
	target := restartwindow.Target{Kind: "Deployment", Namespace: "test", Name: "app"}
	if err := restartwindow.Request(context.Background(), client, target, "original", restartwindow.Source{Type: constants.SecretEnvVarPostfix, Name: "tls", ObservedHash: "test"}); err != nil {
		t.Fatal(err)
	}
	ctrl := NewRestartWindowController(client, "test", nil, "", "", record.NewFakeRecorder(10))
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { defer close(done); ctrl.Run(1, stop) }()
	defer func() {
		close(stop)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("scheduler failed to stop")
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current, err := client.AppsV1().Deployments("test").Get(context.Background(), "app", metav1.GetOptions{})
		if err == nil && current.Spec.Template.Annotations[restartwindow.RestartAnnotation] != "" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("pending request did not execute without a new Secret event")
}
