package controller_test

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
)

func TestSecretReconciler_NotFound(t *testing.T) {
	cfg := config.NewDefault()
	reconciler := newSecretReconciler(t, cfg)
	assertReconcileSuccess(t, reconciler, reconcileRequest("nonexistent-secret", "default"))
}

func TestSecretReconciler_NotFound_ReloadOnDelete(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnDelete = true

	deployment := testDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.SecretReload: "deleted-secret",
	})
	reconciler := newSecretReconciler(t, cfg, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("deleted-secret", "default"))
}

func TestSecretReconciler_IgnoredNamespace(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}

	secret := testSecret("test-secret", "kube-system")
	reconciler := newSecretReconciler(t, cfg, secret)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "kube-system"))
}

func TestSecretReconciler_NoMatchingWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	secret := testSecret("test-secret", "default")
	deployment := testDeployment("test-deployment", "default", nil)
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "default"))
}

func TestSecretReconciler_MatchingDeployment_AutoAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	secret := testSecret("test-secret", "default")
	deployment := testDeploymentWithEnvFrom("test-deployment", "default", "", "test-secret")
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "default"))
}

func TestSecretReconciler_MatchingDeployment_ExplicitAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	secret := testSecret("test-secret", "default")
	deployment := testDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.SecretReload: "test-secret",
	})
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "default"))
}

func TestSecretReconciler_WorkloadInDifferentNamespace(t *testing.T) {
	cfg := config.NewDefault()

	secret := testSecret("test-secret", "namespace-a")
	deployment := testDeployment("test-deployment", "namespace-b", map[string]string{
		cfg.Annotations.SecretReload: "test-secret",
	})
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "namespace-a"))
}

func TestSecretReconciler_IgnoredWorkloadType(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredWorkloads = []string{"deployment"}

	secret := testSecret("test-secret", "default")
	deployment := testDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.SecretReload: "test-secret",
	})
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "default"))
}

func TestSecretReconciler_DaemonSet(t *testing.T) {
	cfg := config.NewDefault()

	secret := testSecret("test-secret", "default")
	daemonset := testDaemonSet("test-daemonset", "default", map[string]string{
		cfg.Annotations.SecretReload: "test-secret",
	})
	reconciler := newSecretReconciler(t, cfg, secret, daemonset)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "default"))
}

func TestSecretReconciler_StatefulSet(t *testing.T) {
	cfg := config.NewDefault()

	secret := testSecret("test-secret", "default")
	statefulset := testStatefulSet("test-statefulset", "default", map[string]string{
		cfg.Annotations.SecretReload: "test-secret",
	})
	reconciler := newSecretReconciler(t, cfg, secret, statefulset)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "default"))
}

func TestSecretReconciler_MultipleWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	secret := testSecret("shared-secret", "default")
	deployment1 := testDeployment("deployment-1", "default", map[string]string{
		cfg.Annotations.SecretReload: "shared-secret",
	})
	deployment2 := testDeployment("deployment-2", "default", map[string]string{
		cfg.Annotations.SecretReload: "shared-secret",
	})
	daemonset := testDaemonSet("daemonset-1", "default", map[string]string{
		cfg.Annotations.SecretReload: "shared-secret",
	})

	reconciler := newSecretReconciler(t, cfg, secret, deployment1, deployment2, daemonset)
	assertReconcileSuccess(t, reconciler, reconcileRequest("shared-secret", "default"))
}

func TestSecretReconciler_VolumeMount(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	secret := testSecret("volume-secret", "default")
	deployment := testDeploymentWithVolume("test-deployment", "default", "", "volume-secret")
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("volume-secret", "default"))
}

func TestSecretReconciler_ProjectedVolume(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	secret := testSecret("projected-secret", "default")
	deployment := testDeploymentWithProjectedVolume("test-deployment", "default", "", "projected-secret")
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("projected-secret", "default"))
}

func TestSecretReconciler_SearchAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	secret := testSecretWithAnnotations("test-secret", "default", map[string]string{
		cfg.Annotations.Match: "true",
	})
	deployment := testDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.Search: "true",
	})
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-secret", "default"))
}

func TestSecretReconciler_ServiceAccountTokenIgnored(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	// Service account tokens should be ignored
	secret := testSecret("sa-token", "default")
	secret.Type = "kubernetes.io/service-account-token"

	deployment := testDeploymentWithEnvFrom("test-deployment", "default", "", "sa-token")
	reconciler := newSecretReconciler(t, cfg, secret, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("sa-token", "default"))
}
