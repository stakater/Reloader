package controller_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
)

const raNamespace = "default"

// reloadAfterEnv wires a secret reconciler and the Deployment reload-after
// reconciler onto one fake client, the way SetupReconcilers does.
type reloadAfterEnv struct {
	t         *testing.T
	cfg       *config.Config
	client    client.Client
	secret    *controller.SecretReconciler
	dependent *controller.DependentReloadReconciler
}

func newReloadAfterEnv(t *testing.T, objects ...client.Object) *reloadAfterEnv {
	t.Helper()
	cfg := config.NewDefault()
	log := testr.New(t)
	c := fake.NewClientBuilder().WithScheme(testutil.NewScheme()).WithObjects(objects...).Build()
	svc := reload.NewService(cfg, log)
	registry := workload.NewRegistry(workload.RegistryOptions{})
	collectors := metrics.NewCollectors()
	recorder := events.NewRecorder(nil)
	webhookClient := webhook.NewClient("", log)
	alerter := &alerting.NoOpAlerter{}

	return &reloadAfterEnv{
		t:      t,
		cfg:    cfg,
		client: c,
		secret: controller.NewSecretReconciler(
			c, log, cfg, svc, registry, &collectors, recorder, webhookClient, alerter, nil, nil,
		),
		dependent: &controller.DependentReloadReconciler{
			Client:        c,
			Log:           log,
			Config:        cfg,
			Kind:          workload.KindDeployment,
			Registry:      registry,
			ReloadService: svc,
			Handler: &controller.ReloadHandler{
				Client:        c,
				ReloadService: svc,
				WebhookClient: webhookClient,
				Collectors:    &collectors,
				EventRecorder: recorder,
				Alerter:       alerter,
			},
			EventRecorder: recorder,
			Collectors:    &collectors,
		},
	}
}

// changeSecret reconciles the "db" secret every fixture uses.
func (e *reloadAfterEnv) changeSecret() {
	e.t.Helper()
	if _, err := e.secret.Reconcile(context.Background(), reconcileRequest("db", raNamespace)); err != nil {
		e.t.Fatalf("secret Reconcile() error = %v", err)
	}
}

func (e *reloadAfterEnv) reconcileDependent(name string) time.Duration {
	e.t.Helper()
	result, err := e.dependent.Reconcile(context.Background(), reconcileRequest(name, raNamespace))
	if err != nil {
		e.t.Fatalf("dependent Reconcile() error = %v", err)
	}
	return result.RequeueAfter
}

func (e *reloadAfterEnv) deployment(name string) *appsv1.Deployment {
	e.t.Helper()
	var d appsv1.Deployment
	if err := e.client.Get(context.Background(), types.NamespacedName{Name: name, Namespace: raNamespace}, &d); err != nil {
		e.t.Fatalf("get deployment %s: %v", name, err)
	}
	return &d
}

func (e *reloadAfterEnv) pending(name string) reload.PendingReloads {
	e.t.Helper()
	p, err := reload.ParsePendingReloads(e.deployment(name).Annotations[e.cfg.Annotations.PendingReload])
	if err != nil {
		e.t.Fatalf("parse pending reloads: %v", err)
	}
	return p
}

// editPending rewrites every pending entry of a deployment.
func (e *reloadAfterEnv) editPending(name string, edit func(*reload.PendingReload)) {
	e.t.Helper()
	d := e.deployment(name)
	p := e.pending(name)
	for k, entry := range p {
		edit(&entry)
		p[k] = entry
	}
	value, err := p.Encode()
	if err != nil {
		e.t.Fatal(err)
	}
	d.Annotations[e.cfg.Annotations.PendingReload] = value
	if err := e.client.Update(context.Background(), d); err != nil {
		e.t.Fatal(err)
	}
}

func (e *reloadAfterEnv) ownedJob(cronJobName string) *batchv1.Job {
	e.t.Helper()
	var jobs batchv1.JobList
	if err := e.client.List(context.Background(), &jobs, client.InNamespace(raNamespace)); err != nil {
		e.t.Fatal(err)
	}
	for i := range jobs.Items {
		if strings.HasPrefix(jobs.Items[i].Name, cronJobName+"-") {
			return &jobs.Items[i]
		}
	}
	e.t.Fatalf("no job created from cronjob %s", cronJobName)
	return nil
}

func (e *reloadAfterEnv) finishJob(job *batchv1.Job, condition batchv1.JobConditionType) {
	e.t.Helper()
	job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{Type: condition, Status: corev1.ConditionTrue})
	if err := e.client.Status().Update(context.Background(), job); err != nil {
		e.t.Fatal(err)
	}
}

func hasReloaderEnv(d *appsv1.Deployment) bool {
	for _, c := range d.Spec.Template.Spec.Containers {
		for _, env := range c.Env {
			if strings.HasPrefix(env.Name, "STAKATER_") {
				return true
			}
		}
	}
	return false
}

func reloadAfterFixtures(cfg *config.Config, deployAnnotations map[string]string) []client.Object {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: raNamespace},
		Data:       map[string][]byte{"password": []byte("v2")},
	}
	cj := testutil.NewCronJobWithAnnotations("migrate", raNamespace, map[string]string{cfg.Annotations.SecretReload: "db"})
	annotations := map[string]string{cfg.Annotations.SecretReload: "db"}
	for k, v := range deployAnnotations {
		annotations[k] = v
	}
	return []client.Object{secret, cj, testutil.NewDeployment("api", raNamespace, annotations)}
}

func TestReloadAfter_DeploymentWaitsForCronJob(t *testing.T) {
	cfg := config.NewDefault()
	env := newReloadAfterEnv(t, reloadAfterFixtures(cfg, map[string]string{cfg.Annotations.ReloadAfter: "cronjob/migrate"})...)

	env.changeSecret()

	job := env.ownedJob("migrate")
	if hasReloaderEnv(env.deployment("api")) {
		t.Fatal("deployment reloaded before the cronjob's job finished")
	}
	pending := env.pending("api")
	if len(pending) != 1 {
		t.Fatalf("pending entries = %d, want 1", len(pending))
	}

	if requeue := env.reconcileDependent("api"); requeue == 0 {
		t.Error("expected requeue while the job is running")
	}
	if hasReloaderEnv(env.deployment("api")) {
		t.Fatal("deployment reloaded while the job is still running")
	}

	env.finishJob(job, batchv1.JobComplete)
	if requeue := env.reconcileDependent("api"); requeue != 0 {
		t.Errorf("requeue = %s after the last pending reload, want none", requeue)
	}

	d := env.deployment("api")
	if !hasReloaderEnv(d) {
		t.Fatal("deployment not reloaded after the job completed")
	}
	if _, ok := d.Annotations[cfg.Annotations.PendingReload]; ok {
		t.Error("pending-reload annotation not removed after reload")
	}
}

func TestReloadAfter_FailedDependencySkipsReload(t *testing.T) {
	cfg := config.NewDefault()
	env := newReloadAfterEnv(t, reloadAfterFixtures(cfg, map[string]string{cfg.Annotations.ReloadAfter: "cronjob/migrate"})...)

	env.changeSecret()
	env.finishJob(env.ownedJob("migrate"), batchv1.JobFailed)
	env.reconcileDependent("api")

	d := env.deployment("api")
	if hasReloaderEnv(d) {
		t.Fatal("deployment reloaded although its dependency failed")
	}
	if _, ok := d.Annotations[cfg.Annotations.PendingReload]; ok {
		t.Error("pending-reload annotation kept after the dependency failed")
	}
}

func TestReloadAfter_TimeoutSkipsReload(t *testing.T) {
	cfg := config.NewDefault()
	env := newReloadAfterEnv(t, reloadAfterFixtures(cfg, map[string]string{cfg.Annotations.ReloadAfter: "cronjob/migrate"})...)

	env.changeSecret()
	env.editPending("api", func(p *reload.PendingReload) { p.Deadline = time.Now().Add(-time.Second) })
	env.reconcileDependent("api")

	if hasReloaderEnv(env.deployment("api")) {
		t.Fatal("deployment reloaded after the reload-after timeout")
	}
	if len(env.pending("api")) != 0 {
		t.Error("pending entry kept after the reload-after timeout")
	}
}

func TestReloadAfter_DependencyThatNeverReloadsFails(t *testing.T) {
	cfg := config.NewDefault()
	env := newReloadAfterEnv(t, reloadAfterFixtures(cfg, map[string]string{cfg.Annotations.ReloadAfter: "cronjob/migrate"})...)

	env.changeSecret()
	// Drop the triggered Job and age the entry past the grace period: nothing
	// will ever carry the change, so waiting for the timeout would be pointless.
	if err := env.client.Delete(context.Background(), env.ownedJob("migrate")); err != nil {
		t.Fatal(err)
	}
	env.editPending("api", func(p *reload.PendingReload) { p.Since = time.Now().Add(-time.Minute) })
	env.reconcileDependent("api")

	if hasReloaderEnv(env.deployment("api")) {
		t.Fatal("deployment reloaded although its dependency never reloaded")
	}
	if len(env.pending("api")) != 0 {
		t.Error("pending entry kept for a dependency that never reloaded")
	}
}

func TestReloadAfter_DependencyNotInBatchReloadsImmediately(t *testing.T) {
	cfg := config.NewDefault()
	env := newReloadAfterEnv(t, reloadAfterFixtures(cfg, map[string]string{cfg.Annotations.ReloadAfter: "cronjob/unrelated"})...)

	env.changeSecret()

	d := env.deployment("api")
	if !hasReloaderEnv(d) {
		t.Fatal("deployment should reload immediately when its dependency is not reloaded by the change")
	}
	if _, ok := d.Annotations[cfg.Annotations.PendingReload]; ok {
		t.Error("unexpected pending-reload annotation")
	}
}

func TestReloadAfter_DeploymentChain(t *testing.T) {
	cfg := config.NewDefault()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: raNamespace}}
	first := testutil.NewDeployment("first", raNamespace, map[string]string{cfg.Annotations.SecretReload: "db"})
	second := testutil.NewDeployment("second", raNamespace, map[string]string{
		cfg.Annotations.SecretReload: "db",
		cfg.Annotations.ReloadAfter:  "deployment/first",
	})
	third := testutil.NewDeployment("third", raNamespace, map[string]string{
		cfg.Annotations.SecretReload: "db",
		cfg.Annotations.ReloadAfter:  "deployments/second",
	})
	env := newReloadAfterEnv(t, secret, first, second, third)

	env.changeSecret()
	if !hasReloaderEnv(env.deployment("first")) {
		t.Fatal("first deployment should reload immediately")
	}
	if hasReloaderEnv(env.deployment("second")) || hasReloaderEnv(env.deployment("third")) {
		t.Fatal("chained deployments reloaded too early")
	}

	// first has not rolled out yet: second keeps waiting.
	env.reconcileDependent("second")
	if hasReloaderEnv(env.deployment("second")) {
		t.Fatal("second reloaded before first rolled out")
	}

	markRolledOut(t, env, "first")
	env.reconcileDependent("second")
	if !hasReloaderEnv(env.deployment("second")) {
		t.Fatal("second not reloaded after first rolled out")
	}

	// third waits on second, which has applied the change but not rolled out.
	env.reconcileDependent("third")
	if hasReloaderEnv(env.deployment("third")) {
		t.Fatal("third reloaded before second rolled out")
	}
	markRolledOut(t, env, "second")
	env.reconcileDependent("third")
	if !hasReloaderEnv(env.deployment("third")) {
		t.Fatal("third not reloaded after second rolled out")
	}
}

func TestReloadAfter_CycleReloadsImmediately(t *testing.T) {
	cfg := config.NewDefault()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: raNamespace}}
	a := testutil.NewDeployment("a", raNamespace, map[string]string{
		cfg.Annotations.SecretReload: "db",
		cfg.Annotations.ReloadAfter:  "deployment/b",
	})
	b := testutil.NewDeployment("b", raNamespace, map[string]string{
		cfg.Annotations.SecretReload: "db",
		cfg.Annotations.ReloadAfter:  "deployment/a",
	})
	env := newReloadAfterEnv(t, secret, a, b)

	env.changeSecret()
	if !hasReloaderEnv(env.deployment("a")) || !hasReloaderEnv(env.deployment("b")) {
		t.Fatal("workloads in a reload-after cycle should reload immediately")
	}
}

func TestReloadAfter_InvalidAnnotationReloadsImmediately(t *testing.T) {
	cfg := config.NewDefault()
	env := newReloadAfterEnv(t, reloadAfterFixtures(cfg, map[string]string{cfg.Annotations.ReloadAfter: "pod/migrate"})...)

	env.changeSecret()
	if !hasReloaderEnv(env.deployment("api")) {
		t.Fatal("an unusable reload-after annotation should not hold the reload back")
	}
}

// markRolledOut sets the status a finished rollout reports.
func markRolledOut(t *testing.T, env *reloadAfterEnv, name string) {
	t.Helper()
	d := env.deployment(name)
	d.Status.ObservedGeneration = d.Generation
	d.Status.Replicas = 1
	d.Status.UpdatedReplicas = 1
	d.Status.AvailableReplicas = 1
	if err := env.client.Status().Update(context.Background(), d); err != nil {
		t.Fatal(err)
	}
}
