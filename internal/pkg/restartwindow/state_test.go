package restartwindow

import (
	"context"
	"encoding/json"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func fixture() (*fake.Clientset, Target) {
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "sample-app", Namespace: "test", UID: "original", Annotations: map[string]string{PolicyAnnotation: nightly, "secret.reloader.stakater.com/reload": "tls,other"}}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "test"}}}}}}
	return fake.NewClientset(d, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "test"}, Data: map[string][]byte{"tls.crt": []byte("old")}}, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "test"}, Data: map[string][]byte{"tls.crt": []byte("other")}}), Target{"Deployment", "test", "sample-app"}
}
func request(t *testing.T, c *fake.Clientset, target Target, name string) {
	t.Helper()
	s, e := c.CoreV1().Secrets("test").Get(context.Background(), name, metav1.GetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	config := common.GetSecretConfig(s)
	if e = Request(context.Background(), c, target, "original", Source{Type: constants.SecretEnvVarPostfix, Name: name, ObservedHash: config.SHAValue}); e != nil {
		t.Fatal(e)
	}
}
func reconcile(t *testing.T, c *fake.Clientset, target Target, when string) (time.Duration, error) {
	t.Helper()
	return Reconcile(context.Background(), c, target, func() time.Time { return at(when) }, labels.Everything(), func(string) bool { return true }, nil)
}
func deployment(t *testing.T, c *fake.Clientset) *appsv1.Deployment {
	t.Helper()
	d, e := c.AppsV1().Deployments("test").Get(context.Background(), "sample-app", metav1.GetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestPersistentCoalescingAndFreshSource(t *testing.T) {
	c, target := fixture()
	request(t, c, target, "tls")
	request(t, c, target, "other")
	if delay, e := reconcile(t, c, target, "2026-09-09T22:00:00Z"); e != nil || delay <= 0 {
		t.Fatal(delay, e)
	}
	d := deployment(t, c)
	if d.Spec.Template.Annotations[RestartAnnotation] != "" {
		t.Fatal("restarted outside window")
	}
	// Simulate controller replacement with only Kubernetes persisted state.
	s, _ := c.CoreV1().Secrets("test").Get(context.Background(), "tls", metav1.GetOptions{})
	s.Data["tls.crt"] = []byte("newest")
	c.CoreV1().Secrets("test").Update(context.Background(), s, metav1.UpdateOptions{})
	c.ClearActions()
	if _, e := reconcile(t, c, target, "2026-09-09T23:00:00Z"); e != nil {
		t.Fatal(e)
	}
	patches := 0
	for _, a := range c.Actions() {
		if a.GetVerb() == "patch" {
			patches++
		}
	}
	if patches != 1 {
		t.Fatalf("expected one combined patch; got %d", patches)
	}
	d = deployment(t, c)
	if d.Annotations[PendingAnnotation] != "" || d.Spec.Template.Annotations[RestartAnnotation] == "" {
		t.Fatal("pending state not consumed atomically")
	}
	var applied map[string]string
	json.Unmarshal([]byte(d.Annotations[AppliedAnnotation]), &applied)
	if applied[constants.SecretEnvVarPostfix+"/tls"] != common.GetSecretConfig(s).SHAValue {
		t.Fatal("used stale certificate hash")
	}
	token := d.Spec.Template.Annotations[RestartAnnotation]
	// A replayed event after a crash does not change the restart token.
	request(t, c, target, "tls")
	if _, e := reconcile(t, c, target, "2026-09-09T23:05:00Z"); e != nil {
		t.Fatal(e)
	}
	if deployment(t, c).Spec.Template.Annotations[RestartAnnotation] != token {
		t.Fatal("duplicate rollout on replay")
	}
}
func TestInvalidPolicyAndMissingSecretHold(t *testing.T) {
	c, target := fixture()
	request(t, c, target, "tls")
	d := deployment(t, c)
	d.Annotations[PolicyAnnotation] = "invalid"
	c.AppsV1().Deployments("test").Update(context.Background(), d, metav1.UpdateOptions{})
	if _, e := reconcile(t, c, target, "2026-09-09T23:00:00Z"); e == nil {
		t.Fatal("invalid policy allowed")
	}
	d = deployment(t, c)
	d.Annotations[PolicyAnnotation] = nightly
	c.AppsV1().Deployments("test").Update(context.Background(), d, metav1.UpdateOptions{})
	c.CoreV1().Secrets("test").Delete(context.Background(), "tls", metav1.DeleteOptions{})
	if _, e := reconcile(t, c, target, "2026-09-09T23:00:00Z"); e == nil {
		t.Fatal("missing Secret allowed")
	}
	if deployment(t, c).Annotations[PendingAnnotation] == "" {
		t.Fatal("lost pending state")
	}
}
func TestOptOutAndUIDProtection(t *testing.T) {
	c, target := fixture()
	if e := Request(context.Background(), c, target, "wrong-uid", Source{Type: constants.SecretEnvVarPostfix, Name: "tls"}); e != nil {
		t.Fatal(e)
	}
	if deployment(t, c).Annotations[PendingAnnotation] != "" {
		t.Fatal("transferred to different UID")
	}
	request(t, c, target, "tls")
	d := deployment(t, c)
	delete(d.Annotations, "secret.reloader.stakater.com/reload")
	c.AppsV1().Deployments("test").Update(context.Background(), d, metav1.UpdateOptions{})
	if _, e := reconcile(t, c, target, "2026-09-09T23:00:00Z"); e != nil {
		t.Fatal(e)
	}
	if deployment(t, c).Spec.Template.Annotations[RestartAnnotation] != "" {
		t.Fatal("ignored workload restarted")
	}
}
func TestAllSupportedWorkloads(t *testing.T) {
	for _, kind := range []string{"StatefulSet", "DaemonSet"} {
		t.Run(kind, func(t *testing.T) {
			c, _ := fixture()
			meta := metav1.ObjectMeta{Name: "app", Namespace: "test", UID: "original", Annotations: map[string]string{PolicyAnnotation: nightly, "secret.reloader.stakater.com/reload": "tls"}}
			spec := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}
			var obj runtime.Object
			if kind == "StatefulSet" {
				obj = &appsv1.StatefulSet{ObjectMeta: meta, Spec: appsv1.StatefulSetSpec{Template: spec}}
			} else {
				obj = &appsv1.DaemonSet{ObjectMeta: meta, Spec: appsv1.DaemonSetSpec{Template: spec}}
			}
			c.Tracker().Add(obj)
			target := Target{kind, "test", "app"}
			request(t, c, target, "tls")
			if _, e := reconcile(t, c, target, "2026-09-09T23:00:00Z"); e != nil {
				t.Fatal(e)
			}
			x, e := get(context.Background(), c, target)
			if e != nil || template(x).Annotations[RestartAnnotation] == "" {
				t.Fatal("did not restart", e)
			}
		})
	}
}

func TestPendingSourceUpdatesAndConflictRetry(t *testing.T) {
	c, target := fixture()
	request(t, c, target, "tls")
	s, _ := c.CoreV1().Secrets("test").Get(context.Background(), "tls", metav1.GetOptions{})
	s.Data["tls.crt"] = []byte("newer")
	c.CoreV1().Secrets("test").Update(context.Background(), s, metav1.UpdateOptions{})
	request(t, c, target, "tls")
	pending, _, e := state(deployment(t, c).Annotations)
	if e != nil {
		t.Fatal(e)
	}
	if pending[constants.SecretEnvVarPostfix+"/tls"].ObservedHash != common.GetSecretConfig(s).SHAValue {
		t.Fatal("newer event did not update pending state")
	}
	conflict := true
	c.PrependReactor("patch", "deployments", func(a clienttesting.Action) (bool, runtime.Object, error) {
		if !conflict {
			return false, nil, nil
		}
		conflict = false
		var body map[string]any
		json.Unmarshal(a.(clienttesting.PatchAction).GetPatch(), &body)
		metadata := body["metadata"].(map[string]any)
		if _, ok := metadata["resourceVersion"]; !ok {
			t.Error("missing optimistic concurrency precondition")
		}
		if metadata["uid"] != "original" {
			t.Error("missing UID precondition")
		}
		return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, "sample-app", fmt.Errorf("concurrent change"))
	})
	if _, e := reconcile(t, c, target, "2026-09-09T23:00:00Z"); e == nil {
		t.Fatal("conflict hidden")
	}
	if deployment(t, c).Annotations[PendingAnnotation] == "" {
		t.Fatal("lost pending on conflict")
	}
	if _, e := reconcile(t, c, target, "2026-09-09T23:00:01Z"); e != nil {
		t.Fatal(e)
	}
	if deployment(t, c).Spec.Template.Annotations[RestartAnnotation] == "" {
		t.Fatal("retry failed")
	}
}

func TestWindowClosesDuringSourceReads(t *testing.T) {
	c, target := fixture()
	request(t, c, target, "tls")
	calls := 0
	_, err := Reconcile(context.Background(), c, target, func() time.Time {
		calls++
		if calls == 1 {
			return at("2026-09-09T23:59:59Z")
		}
		return at("2026-09-10T00:00:00Z")
	}, labels.Everything(), func(string) bool { return true }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if deployment(t, c).Annotations[PendingAnnotation] == "" || deployment(t, c).Spec.Template.Annotations[RestartAnnotation] != "" {
		t.Fatal("crossed closing boundary")
	}
}
