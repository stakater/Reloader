package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/pkg/config"
	"github.com/stakater/Reloader/pkg/matcher"
)

// alwaysDenied is a deny window without gaps: hourly occurrences lasting 90 minutes.
const alwaysDenied = `[{"kind":"deny","schedule":"0 * * * *","duration":"90m"}]`

const envVar = "STAKATER_TEST_CM_CONFIGMAP"

// reconcilerFor builds a ConfigMapReconciler over an existing client, as a restarted
// Reloader would: nothing carries over but what is stored in the cluster.
func reconcilerFor(t *testing.T, cfg *config.Config, c client.Client, webhookURL string) *controller.ConfigMapReconciler {
	t.Helper()
	deps := newTestDeps(t, cfg)
	return controller.NewConfigMapReconciler(
		c, deps.log, deps.cfg, deps.reloadService, deps.registry, deps.collectors,
		deps.eventRecorder, webhook.NewClient(webhookURL, deps.log), deps.alerter, nil, nil,
	)
}

func reconcileForRequeue(t *testing.T, reconciler *controller.ConfigMapReconciler) bool {
	t.Helper()
	result, err := reconciler.Reconcile(context.Background(), reconcileRequest("test-cm", "default"))
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	return result.RequeueAfter > 0
}

func getObject[T client.Object](t *testing.T, c client.Client, name string, obj T) T {
	t.Helper()
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, obj); err != nil {
		t.Fatalf("Failed to get %s: %v", name, err)
	}
	return obj
}

func deploymentHasEnvVar(t *testing.T, c client.Client, name string) bool {
	t.Helper()
	deploy := getObject(t, c, name, &appsv1.Deployment{})
	for _, env := range deploy.Spec.Template.Spec.Containers[0].Env {
		if env.Name == envVar {
			return true
		}
	}
	return false
}

func pendingFor(t *testing.T, cfg *config.Config, obj client.Object) (reload.PendingReload, bool) {
	t.Helper()
	return reload.FindPendingReload(obj.GetAnnotations(), cfg.Annotations.PendingReload, matcher.ResourceTypeConfigMap, "test-cm")
}

func countJobs(t *testing.T, c client.Client) int {
	t.Helper()
	var jobs batchv1.JobList
	if err := c.List(context.Background(), &jobs, client.InNamespace("default")); err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	return len(jobs.Items)
}

func removeSyncWindows(t *testing.T, cfg *config.Config, c client.Client, obj client.Object) {
	t.Helper()
	obj = getObject(t, c, obj.GetName(), obj)
	annotations := obj.GetAnnotations()
	delete(annotations, cfg.Annotations.SyncWindows)
	obj.SetAnnotations(annotations)
	if err := c.Update(context.Background(), obj); err != nil {
		t.Fatalf("Failed to update %s: %v", obj.GetName(), err)
	}
}

func updateConfigMap(t *testing.T, c client.Client, value string) {
	t.Helper()
	cm := getObject(t, c, "test-cm", &corev1.ConfigMap{})
	cm.Data = map[string]string{"key": value}
	if err := c.Update(context.Background(), cm); err != nil {
		t.Fatalf("Failed to update configmap: %v", err)
	}
}

func TestConfigMapReconciler_SyncWindowDefersReload(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	deferred := testutil.NewDeployment("deferred", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     alwaysDenied,
	})
	cronJob := testutil.NewCronJobWithAnnotations("cronjob", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, deferred, cronJob)
	c := reconciler.Client

	if !reconcileForRequeue(t, reconciler) {
		t.Fatal("Expected a requeue while the sync window denies the reload")
	}
	if deploymentHasEnvVar(t, c, "deferred") {
		t.Error("Deployment was reloaded while its sync window denies it")
	}
	entry, ok := pendingFor(t, cfg, getObject(t, c, "deferred", &appsv1.Deployment{}))
	if !ok {
		t.Fatal("Deferred reload was not recorded on the Deployment")
	}
	if entry.Hash == "" || !entry.NotBefore.After(time.Now()) {
		t.Errorf("Recorded pending reload = %+v, want a hash and a due time in the future", entry)
	}
	if got := countJobs(t, c); got != 1 {
		t.Fatalf("Expected the CronJob without sync windows to create 1 job, got %d", got)
	}

	// A wake-up for the same content only retries the deferred workload.
	if !reconcileForRequeue(t, reconciler) {
		t.Fatal("Expected another requeue while the sync window still denies the reload")
	}
	if got := countJobs(t, c); got != 1 {
		t.Errorf("Wake-up reconcile reloaded the CronJob again: got %d jobs, want 1", got)
	}
	if again, _ := pendingFor(t, cfg, getObject(t, c, "deferred", &appsv1.Deployment{})); again != entry {
		t.Errorf("Wake-up changed the pending reload from %+v to %+v", entry, again)
	}

	// A restarted Reloader has only the cluster state, and still owes the same reload.
	restarted := reconcilerFor(t, cfg, c, "")
	if !reconcileForRequeue(t, restarted) {
		t.Fatal("Restarted Reloader lost the deferred reload")
	}
	if got := countJobs(t, c); got != 1 {
		t.Errorf("Restarted Reloader reloaded the CronJob again: got %d jobs, want 1", got)
	}

	// Once the window no longer blocks the reload, the deferred workload reloads.
	removeSyncWindows(t, cfg, c, &appsv1.Deployment{ObjectMeta: deferred.ObjectMeta})
	if reconcileForRequeue(t, restarted) {
		t.Error("Expected no requeue once nothing is deferred")
	}
	if !deploymentHasEnvVar(t, c, "deferred") {
		t.Error("Deferred deployment was not reloaded once its sync window allowed it")
	}
	if _, ok := pendingFor(t, cfg, getObject(t, c, "deferred", &appsv1.Deployment{})); ok {
		t.Error("Pending reload was not cleared after the reload")
	}
	if got := countJobs(t, c); got != 1 {
		t.Errorf("Retrying the deferred reload reloaded the CronJob again: got %d jobs, want 1", got)
	}

	// A leftover requeue timer for the same content does nothing.
	if reconcileForRequeue(t, restarted) {
		t.Error("Expected no requeue for a leftover timer")
	}
	if got := countJobs(t, c); got != 1 {
		t.Errorf("Leftover requeue reloaded the CronJob again: got %d jobs, want 1", got)
	}

	// A new change to the ConfigMap is processed for every workload again.
	updateConfigMap(t, c, "changed")
	if reconcileForRequeue(t, restarted) {
		t.Error("Expected no requeue for a change no sync window defers")
	}
	if got := countJobs(t, c); got != 2 {
		t.Errorf("Expected the new change to reload the CronJob: got %d jobs, want 2", got)
	}
}

func TestConfigMapReconciler_SyncWindowIgnoredForJobsAndCronJobs(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	cronJob := testutil.NewCronJobWithAnnotations("cronjob", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     alwaysDenied,
	})
	job := testutil.NewJobWithAnnotations("job", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     alwaysDenied,
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, cronJob, job)
	c := reconciler.Client

	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))

	// The CronJob started a new Job and the Job was recreated: neither waited.
	var jobs batchv1.JobList
	if err := c.List(context.Background(), &jobs, client.InNamespace("default")); err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	if len(jobs.Items) != 2 {
		t.Fatalf("Expected the recreated Job and one Job from the CronJob, got %d jobs", len(jobs.Items))
	}
	for _, j := range jobs.Items {
		if _, ok := pendingFor(t, cfg, &j); ok {
			t.Errorf("Job %s records a pending reload, want none", j.Name)
		}
	}
	if _, ok := pendingFor(t, cfg, getObject(t, c, "cronjob", &batchv1.CronJob{})); ok {
		t.Error("CronJob records a pending reload, want none")
	}
}

func TestConfigMapReconciler_SyncWindowRecordFailureRetriesOnlyDeferred(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	deferred := testutil.NewDeployment("deferred", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     alwaysDenied,
	})
	cronJob := testutil.NewCronJobWithAnnotations("cronjob", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})

	failUpdates := true
	c := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithRuntimeObjects(cm, deferred, cronJob).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if _, ok := obj.(*appsv1.Deployment); ok && failUpdates {
					return errors.New("injected update failure")
				}
				return c.Update(ctx, obj, opts...)
			},
		}).
		Build()
	reconciler := reconcilerFor(t, cfg, c, "")

	if _, err := reconciler.Reconcile(context.Background(), reconcileRequest("test-cm", "default")); err == nil {
		t.Fatal("Expected the failed pending-reload write to be returned as an error")
	}
	if got := countJobs(t, c); got != 1 {
		t.Fatalf("Expected the CronJob to create 1 job, got %d", got)
	}

	// The error retry writes the pending reload but does not reload the CronJob again.
	failUpdates = false
	if !reconcileForRequeue(t, reconciler) {
		t.Fatal("Expected a requeue for the deferred workload")
	}
	if _, ok := pendingFor(t, cfg, getObject(t, c, "deferred", &appsv1.Deployment{})); !ok {
		t.Error("Error retry did not record the deferred reload")
	}
	if got := countJobs(t, c); got != 1 {
		t.Errorf("Error retry reloaded the CronJob again: got %d jobs, want 1", got)
	}
}

func TestConfigMapReconciler_RepeatWithoutSyncWindowsUnchanged(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	cronJob := testutil.NewCronJobWithAnnotations("cronjob", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, cronJob)

	// Without sync windows a repeated reconcile behaves as before: nothing is remembered.
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
	if got := countJobs(t, reconciler.Client); got != 2 {
		t.Errorf("Expected each reconcile to reload the CronJob as before: got %d jobs, want 2", got)
	}
}

func TestConfigMapReconciler_SyncWindowGarbageReloadsNow(t *testing.T) {
	cfg := config.NewDefault()

	garbage := map[string]string{
		"not yaml":            `[{{{`,
		"not a list":          `kind: deny`,
		"unknown kind":        `[{"kind":"maybe","schedule":"0 2 * * *","duration":"1h"}]`,
		"bad schedule":        `[{"kind":"deny","schedule":"every night","duration":"1h"}]`,
		"bad duration":        `[{"kind":"deny","schedule":"0 2 * * *","duration":"forever"}]`,
		"bad time zone":       `[{"kind":"deny","schedule":"0 2 * * *","duration":"1h","timeZone":"Mars/Base"}]`,
		"bad spread":          `[{"kind":"deny","schedule":"0 2 * * *","duration":"1h","spread":"-1h"}]`,
		"interval schedule":   `[{"kind":"deny","schedule":"@every 1m","duration":"1h"}]`,
		"never fires":         `[{"kind":"allow","schedule":"0 0 31 2 *","duration":"1h"}]`,
		"garbage pending too": `[{"kind":"deny","schedule":"0 2 * * *","duration":"x"}]`,
	}
	for name, windows := range garbage {
		t.Run(name, func(t *testing.T) {
			cm := testutil.NewConfigMap("test-cm", "default")
			annotations := map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     windows,
			}
			if name == "garbage pending too" {
				annotations[cfg.Annotations.PendingReload] = `{not json`
			}
			reconciler := newConfigMapReconciler(t, cfg, cm, testutil.NewDeployment("app", "default", annotations))

			assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
			if !deploymentHasEnvVar(t, reconciler.Client, "app") {
				t.Error("Deployment with an unusable sync-windows annotation was not reloaded")
			}
		})
	}
}

func TestConfigMapReconciler_SyncWindowAllowsReload(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	deployment := testutil.NewDeployment("allowed", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		// An allow window without gaps always permits the reload.
		cfg.Annotations.SyncWindows: `[{"kind":"allow","schedule":"0 * * * *","duration":"90m"}]`,
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)

	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
	if !deploymentHasEnvVar(t, reconciler.Client, "allowed") {
		t.Error("Deployment was not reloaded inside its allow window")
	}
	if _, ok := pendingFor(t, cfg, getObject(t, reconciler.Client, "allowed", &appsv1.Deployment{})); ok {
		t.Error("A reload inside the window was recorded as pending")
	}
}

func TestConfigMapReconciler_SyncWindowClearedWhenResourceDeleted(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	deployment := testutil.NewDeployment("deferred", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     alwaysDenied,
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	c := reconciler.Client

	if !reconcileForRequeue(t, reconciler) {
		t.Fatal("Expected a requeue while the sync window denies the reload")
	}

	if err := c.Delete(context.Background(), cm); err != nil {
		t.Fatalf("Failed to delete configmap: %v", err)
	}
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
	if _, ok := pendingFor(t, cfg, getObject(t, c, "deferred", &appsv1.Deployment{})); ok {
		t.Error("Pending reload for a deleted ConfigMap was not cleared")
	}
}

func TestConfigMapReconciler_SyncWindowWebhookMode(t *testing.T) {
	cfg := config.NewDefault()

	var mu sync.Mutex
	var payloads []webhook.Payload
	failNext := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if failNext {
			failNext = false
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var payload webhook.Payload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode webhook payload: %v", err)
		}
		payloads = append(payloads, payload)
	}))
	defer server.Close()

	cm := testutil.NewConfigMap("test-cm", "default")
	deferred := testutil.NewDeployment("deferred", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     alwaysDenied,
	})
	ready := testutil.NewDeployment("ready", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	deps := newTestDeps(t, cfg, cm, deferred, ready)
	reconciler := reconcilerFor(t, cfg, deps.client.Build(), server.URL)
	c := reconciler.Client

	if _, err := reconciler.Reconcile(context.Background(), reconcileRequest("test-cm", "default")); err == nil {
		t.Fatal("Expected the failed webhook call to be returned as an error")
	}

	// The error retry must notify the workload that was ready, not only the deferred one.
	if !reconcileForRequeue(t, reconciler) {
		t.Fatal("Expected a requeue for the deferred workload")
	}
	mu.Lock()
	if len(payloads) != 1 {
		t.Fatalf("Expected 1 webhook payload, got %d", len(payloads))
	}
	if got := payloads[0].Workloads; len(got) != 1 || got[0].Name != "ready" {
		t.Errorf("Webhook payload workloads = %+v, want only the ready deployment", got)
	}
	mu.Unlock()

	// While the window still denies the reload, the wake-up sends nothing.
	if !reconcileForRequeue(t, reconciler) {
		t.Fatal("Expected another requeue for the deferred workload")
	}
	mu.Lock()
	if len(payloads) != 1 {
		t.Errorf("Wake-up reconcile sent another webhook: got %d payloads, want 1", len(payloads))
	}
	mu.Unlock()

	// Once allowed, the deferred workload is notified and its entry cleared.
	removeSyncWindows(t, cfg, c, &appsv1.Deployment{ObjectMeta: deferred.ObjectMeta})
	if reconcileForRequeue(t, reconciler) {
		t.Error("Expected no requeue once nothing is deferred")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 2 || len(payloads[1].Workloads) != 1 || payloads[1].Workloads[0].Name != "deferred" {
		t.Errorf("Webhook payloads = %+v, want a second one for the deferred deployment", payloads)
	}
	if _, ok := pendingFor(t, cfg, getObject(t, c, "deferred", &appsv1.Deployment{})); ok {
		t.Error("Pending reload was not cleared after the webhook notification")
	}
}
