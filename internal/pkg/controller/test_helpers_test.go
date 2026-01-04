package controller_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// testDeps holds shared test dependencies.
type testDeps struct {
	client        *fake.ClientBuilder
	log           logr.Logger
	cfg           *config.Config
	reloadService *reload.Service
	registry      *workload.Registry
	collectors    *metrics.Collectors
	eventRecorder *events.Recorder
	webhookClient *webhook.Client
	alerter       alerting.Alerter
}

// newTestDeps creates shared test dependencies for reconciler tests.
func newTestDeps(t *testing.T, cfg *config.Config, objects ...runtime.Object) testDeps {
	t.Helper()
	log := testr.New(t)
	collectors := metrics.NewCollectors()
	return testDeps{
		client: fake.NewClientBuilder().
			WithScheme(testutil.NewScheme()).
			WithRuntimeObjects(objects...),
		log:           log,
		cfg:           cfg,
		reloadService: reload.NewService(cfg, log),
		registry: workload.NewRegistry(workload.RegistryOptions{
			ArgoRolloutsEnabled:       cfg.ArgoRolloutsEnabled,
			DeploymentConfigEnabled:   cfg.DeploymentConfigEnabled,
			RolloutStrategyAnnotation: cfg.Annotations.RolloutStrategy,
		}),
		collectors:    &collectors,
		eventRecorder: events.NewRecorder(nil),
		webhookClient: webhook.NewClient("", log),
		alerter:       &alerting.NoOpAlerter{},
	}
}

// newConfigMapReconciler creates a ConfigMapReconciler for testing.
func newConfigMapReconciler(t *testing.T, cfg *config.Config, objects ...runtime.Object) *controller.ConfigMapReconciler {
	t.Helper()
	deps := newTestDeps(t, cfg, objects...)
	return controller.NewConfigMapReconciler(
		deps.client.Build(),
		deps.log,
		deps.cfg,
		deps.reloadService,
		deps.registry,
		deps.collectors,
		deps.eventRecorder,
		deps.webhookClient,
		deps.alerter,
		nil,
	)
}

// newSecretReconciler creates a SecretReconciler for testing.
func newSecretReconciler(t *testing.T, cfg *config.Config, objects ...runtime.Object) *controller.SecretReconciler {
	t.Helper()
	deps := newTestDeps(t, cfg, objects...)
	return controller.NewSecretReconciler(
		deps.client.Build(),
		deps.log,
		deps.cfg,
		deps.reloadService,
		deps.registry,
		deps.collectors,
		deps.eventRecorder,
		deps.webhookClient,
		deps.alerter,
		nil,
	)
}

// newNamespaceReconciler creates a NamespaceReconciler for testing.
func newNamespaceReconciler(t *testing.T, cfg *config.Config, cache *controller.NamespaceCache, objects ...runtime.Object) *controller.NamespaceReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		Build()

	return &controller.NamespaceReconciler{
		Client: fakeClient,
		Log:    testr.New(t),
		Config: cfg,
		Cache:  cache,
	}
}

// reconcileRequest creates a ctrl.Request for the given name and namespace.
func reconcileRequest(name, namespace string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// namespaceRequest creates a ctrl.Request for a namespace (no namespace field needed).
func namespaceRequest(name string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name},
	}
}

// assertReconcileSuccess runs reconcile and asserts no error and no requeue.
func assertReconcileSuccess(t *testing.T, reconciler interface {
	Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
}, req ctrl.Request) {
	t.Helper()
	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Should not requeue")
	}
}
