package controller_test

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/testutil"
)

func TestConfigMapReconciler_NotFound(t *testing.T) {
	cfg := config.NewDefault()
	reconciler := newConfigMapReconciler(t, cfg)
	assertReconcileSuccess(t, reconciler, reconcileRequest("nonexistent-cm", "default"))
}

func TestConfigMapReconciler_NotFound_ReloadOnDelete(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnDelete = true

	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "deleted-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("deleted-cm", "default"))
}

func TestConfigMapReconciler_IgnoredNamespace(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}

	cm := testutil.NewConfigMap("test-cm", "kube-system")
	reconciler := newConfigMapReconciler(t, cfg, cm)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "kube-system"))
}

func TestConfigMapReconciler_NoMatchingWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	deployment := testutil.NewDeployment("test-deployment", "default", nil)
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
}

func TestConfigMapReconciler_MatchingDeployment_AutoAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	cm := testutil.NewConfigMap("test-cm", "default")
	deployment := testutil.NewDeploymentWithEnvFrom("test-deployment", "default", "test-cm", "")
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
}

func TestConfigMapReconciler_MatchingDeployment_ExplicitAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
}

func TestConfigMapReconciler_WorkloadInDifferentNamespace(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "namespace-a")
	deployment := testutil.NewDeployment("test-deployment", "namespace-b", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "namespace-a"))
}

func TestConfigMapReconciler_IgnoredWorkloadType(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredWorkloads = []string{"deployment"}

	cm := testutil.NewConfigMap("test-cm", "default")
	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
}

func TestConfigMapReconciler_DaemonSet(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	daemonset := testutil.NewDaemonSet("test-daemonset", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, daemonset)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
}

func TestConfigMapReconciler_StatefulSet(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("test-cm", "default")
	statefulset := testutil.NewStatefulSet("test-statefulset", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, statefulset)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
}

func TestConfigMapReconciler_MultipleWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMap("shared-cm", "default")
	deployment1 := testutil.NewDeployment("deployment-1", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "shared-cm",
	})
	deployment2 := testutil.NewDeployment("deployment-2", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "shared-cm",
	})
	daemonset := testutil.NewDaemonSet("daemonset-1", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "shared-cm",
	})

	reconciler := newConfigMapReconciler(t, cfg, cm, deployment1, deployment2, daemonset)
	assertReconcileSuccess(t, reconciler, reconcileRequest("shared-cm", "default"))
}

func TestConfigMapReconciler_VolumeMount(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	cm := testutil.NewConfigMap("volume-cm", "default")
	deployment := testutil.NewDeploymentWithVolume("test-deployment", "default", "volume-cm", "")
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("volume-cm", "default"))
}

func TestConfigMapReconciler_ProjectedVolume(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	cm := testutil.NewConfigMap("projected-cm", "default")
	deployment := testutil.NewDeploymentWithProjectedVolume("test-deployment", "default", "projected-cm", "")
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("projected-cm", "default"))
}

func TestConfigMapReconciler_SearchAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	cm := testutil.NewConfigMapWithAnnotations("test-cm", "default", map[string]string{
		cfg.Annotations.Match: "true",
	})
	deployment := testutil.NewDeployment("test-deployment", "default", map[string]string{
		cfg.Annotations.Search: "true",
	})
	reconciler := newConfigMapReconciler(t, cfg, cm, deployment)
	assertReconcileSuccess(t, reconciler, reconcileRequest("test-cm", "default"))
}
