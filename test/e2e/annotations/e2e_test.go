// Package annotations contains end-to-end tests for Reloader's Annotations Reload Strategy.
package annotations

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/go-logr/zerologr"
	openshiftclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/rs/zerolog"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/openshift"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	testNamespacePrefix = "test-reloader-e2e-"
	waitTimeout         = 30 * time.Second
	setupDelay          = 2 * time.Second
	negativeTestTimeout = 5 * time.Second
)

var (
	k8sClient                 kubernetes.Interface
	osClient                  openshiftclient.Interface
	cfg                       *config.Config
	namespace                 string
	skipE2ETests              bool
	skipDeploymentConfigTests bool
	cancelManager             context.CancelFunc
	restCfg                   *rest.Config
)

// testFixture provides a clean way to set up and tear down test resources.
type testFixture struct {
	t          *testing.T
	name       string
	configMaps []string
	secrets    []string
	workloads  []workloadInfo
}

type workloadInfo struct {
	name string
	kind string // "deployment", "daemonset", "statefulset", "cronjob"
}

// newFixture creates a new test fixture with a unique name prefix.
func newFixture(t *testing.T, prefix string) *testFixture {
	t.Helper()
	skipIfNoCluster(t)
	return &testFixture{
		t:    t,
		name: prefix + "-" + testutil.RandSeq(5),
	}
}

// createConfigMap creates a ConfigMap and registers it for cleanup.
func (f *testFixture) createConfigMap(name, data string) {
	f.t.Helper()
	_, err := testutil.CreateConfigMap(k8sClient, namespace, name, data)
	if err != nil {
		f.t.Fatalf("Failed to create ConfigMap %s: %v", name, err)
	}
	f.configMaps = append(f.configMaps, name)
}

// createSecret creates a Secret and registers it for cleanup.
func (f *testFixture) createSecret(name, data string) {
	f.t.Helper()
	_, err := testutil.CreateSecret(k8sClient, namespace, name, data)
	if err != nil {
		f.t.Fatalf("Failed to create Secret %s: %v", name, err)
	}
	f.secrets = append(f.secrets, name)
}

// createDeployment creates a Deployment and registers it for cleanup.
func (f *testFixture) createDeployment(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateDeployment(k8sClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create Deployment %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "deployment"})
}

// createDaemonSet creates a DaemonSet and registers it for cleanup.
func (f *testFixture) createDaemonSet(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateDaemonSet(k8sClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create DaemonSet %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "daemonset"})
}

// createStatefulSet creates a StatefulSet and registers it for cleanup.
func (f *testFixture) createStatefulSet(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateStatefulSet(k8sClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create StatefulSet %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "statefulset"})
}

// waitForReady waits for all workloads to be ready.
func (f *testFixture) waitForReady() {
	time.Sleep(setupDelay)
}

// updateConfigMap updates a ConfigMap's data.
func (f *testFixture) updateConfigMap(name, data string) {
	f.t.Helper()
	if err := testutil.UpdateConfigMapWithClient(k8sClient, namespace, name, "", data); err != nil {
		f.t.Fatalf("Failed to update ConfigMap %s: %v", name, err)
	}
}

// updateConfigMapLabel updates only a ConfigMap's label (not data).
func (f *testFixture) updateConfigMapLabel(name, label string) {
	f.t.Helper()
	// Get current data first
	cm, err := k8sClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		f.t.Fatalf("Failed to get ConfigMap %s: %v", name, err)
	}
	data := cm.Data["url"]
	if err := testutil.UpdateConfigMapWithClient(k8sClient, namespace, name, label, data); err != nil {
		f.t.Fatalf("Failed to update ConfigMap label %s: %v", name, err)
	}
}

// updateSecret updates a Secret's data.
func (f *testFixture) updateSecret(name, data string) {
	f.t.Helper()
	if err := testutil.UpdateSecretWithClient(k8sClient, namespace, name, "", data); err != nil {
		f.t.Fatalf("Failed to update Secret %s: %v", name, err)
	}
}

// updateSecretLabel updates only a Secret's label (not data).
func (f *testFixture) updateSecretLabel(name, label string) {
	f.t.Helper()
	secret, err := k8sClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		f.t.Fatalf("Failed to get Secret %s: %v", name, err)
	}
	var data string
	if secret.Data != nil {
		if d, ok := secret.Data["test"]; ok {
			data = string(d)
		}
	}
	if err := testutil.UpdateSecretWithClient(k8sClient, namespace, name, label, data); err != nil {
		f.t.Fatalf("Failed to update Secret label %s: %v", name, err)
	}
}

// assertDeploymentReloaded asserts that a deployment was reloaded.
func (f *testFixture) assertDeploymentReloaded(name string, testCfg *config.Config) {
	f.t.Helper()
	if testCfg == nil {
		testCfg = cfg
	}
	updated, err := testutil.WaitForDeploymentReloadedAnnotation(k8sClient, namespace, name, testCfg.Annotations.LastReloadedFrom, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for deployment %s update: %v", name, err)
	}
	if !updated {
		f.t.Errorf("Deployment %s was not updated after resource change", name)
	}
}

// assertDeploymentNotReloaded asserts that a deployment was NOT reloaded.
func (f *testFixture) assertDeploymentNotReloaded(name string, testCfg *config.Config) {
	f.t.Helper()
	if testCfg == nil {
		testCfg = cfg
	}
	time.Sleep(negativeTestTimeout)
	updated, _ := testutil.WaitForDeploymentReloadedAnnotation(k8sClient, namespace, name, testCfg.Annotations.LastReloadedFrom, negativeTestTimeout)
	if updated {
		f.t.Errorf("Deployment %s should not have been updated", name)
	}
}

// assertDaemonSetReloaded asserts that a daemonset was reloaded.
func (f *testFixture) assertDaemonSetReloaded(name string) {
	f.t.Helper()
	updated, err := testutil.WaitForDaemonSetReloadedAnnotation(k8sClient, namespace, name, cfg.Annotations.LastReloadedFrom, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for daemonset %s update: %v", name, err)
	}
	if !updated {
		f.t.Errorf("DaemonSet %s was not updated after resource change", name)
	}
}

// assertDaemonSetNotReloaded asserts that a daemonset was NOT reloaded.
func (f *testFixture) assertDaemonSetNotReloaded(name string) {
	f.t.Helper()
	time.Sleep(negativeTestTimeout)
	updated, _ := testutil.WaitForDaemonSetReloadedAnnotation(k8sClient, namespace, name, cfg.Annotations.LastReloadedFrom, negativeTestTimeout)
	if updated {
		f.t.Errorf("DaemonSet %s should not have been updated", name)
	}
}

// assertStatefulSetReloaded asserts that a statefulset was reloaded.
func (f *testFixture) assertStatefulSetReloaded(name string) {
	f.t.Helper()
	updated, err := testutil.WaitForStatefulSetReloadedAnnotation(k8sClient, namespace, name, cfg.Annotations.LastReloadedFrom, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for statefulset %s update: %v", name, err)
	}
	if !updated {
		f.t.Errorf("StatefulSet %s was not updated after resource change", name)
	}
}

// assertStatefulSetNotReloaded asserts that a statefulset was NOT reloaded.
func (f *testFixture) assertStatefulSetNotReloaded(name string) {
	f.t.Helper()
	time.Sleep(negativeTestTimeout)
	updated, _ := testutil.WaitForStatefulSetReloadedAnnotation(k8sClient, namespace, name, cfg.Annotations.LastReloadedFrom, negativeTestTimeout)
	if updated {
		f.t.Errorf("StatefulSet %s should not have been updated", name)
	}
}

// createDeploymentConfig creates a DeploymentConfig and registers it for cleanup.
func (f *testFixture) createDeploymentConfig(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateDeploymentConfig(osClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create DeploymentConfig %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "deploymentconfig"})
}

// assertDeploymentConfigReloaded asserts that a DeploymentConfig was reloaded.
func (f *testFixture) assertDeploymentConfigReloaded(name string) {
	f.t.Helper()
	updated, err := testutil.WaitForDeploymentConfigReloadedAnnotation(osClient, namespace, name, cfg.Annotations.LastReloadedFrom, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for DeploymentConfig %s update: %v", name, err)
	}
	if !updated {
		f.t.Errorf("DeploymentConfig %s was not updated after resource change", name)
	}
}

// assertDeploymentPaused asserts that a deployment is paused (spec.Paused=true).
func (f *testFixture) assertDeploymentPaused(name string) {
	f.t.Helper()
	paused, err := testutil.WaitForDeploymentPaused(k8sClient, namespace, name, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for deployment %s to be paused: %v", name, err)
	}
	if !paused {
		f.t.Errorf("Deployment %s was not paused after reload", name)
	}
}

// assertDeploymentUnpaused asserts that a deployment is unpaused (spec.Paused=false).
func (f *testFixture) assertDeploymentUnpaused(name string, timeout time.Duration) {
	f.t.Helper()
	unpaused, err := testutil.WaitForDeploymentUnpaused(k8sClient, namespace, name, timeout)
	if err != nil {
		f.t.Fatalf("Error waiting for deployment %s to be unpaused: %v", name, err)
	}
	if !unpaused {
		f.t.Errorf("Deployment %s was not unpaused after pause period", name)
	}
}

// assertDeploymentHasPausedAtAnnotation asserts that a deployment has the paused-at annotation.
func (f *testFixture) assertDeploymentHasPausedAtAnnotation(name string) {
	f.t.Helper()
	deploy, err := k8sClient.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		f.t.Fatalf("Failed to get deployment %s: %v", name, err)
	}
	if deploy.Annotations == nil {
		f.t.Errorf("Deployment %s has no annotations", name)
		return
	}
	if _, ok := deploy.Annotations[cfg.Annotations.PausedAt]; !ok {
		f.t.Errorf("Deployment %s does not have paused-at annotation", name)
	}
}

// cleanup removes all created resources.
func (f *testFixture) cleanup() {
	for _, w := range f.workloads {
		switch w.kind {
		case "deployment":
			_ = testutil.DeleteDeployment(k8sClient, namespace, w.name)
		case "daemonset":
			_ = testutil.DeleteDaemonSet(k8sClient, namespace, w.name)
		case "statefulset":
			_ = testutil.DeleteStatefulSet(k8sClient, namespace, w.name)
		case "deploymentconfig":
			if osClient != nil {
				_ = testutil.DeleteDeploymentConfig(osClient, namespace, w.name)
			}
		case "cronjob":
			_ = testutil.DeleteCronJob(k8sClient, namespace, w.name)
		}
	}
	for _, name := range f.configMaps {
		_ = testutil.DeleteConfigMap(k8sClient, namespace, name)
	}
	for _, name := range f.secrets {
		_ = testutil.DeleteSecret(k8sClient, namespace, name)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		os.Exit(0)
	}

	zl := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		Level(zerolog.WarnLevel).
		With().
		Timestamp().
		Logger()
	ctrllog.SetLogger(zerologr.New(&zl))

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	var err error
	restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		skipE2ETests = true
		os.Exit(0)
	}

	k8sClient, err = kubernetes.NewForConfig(restCfg)
	if err != nil {
		skipE2ETests = true
		os.Exit(0)
	}

	if _, err = k8sClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{}); err != nil {
		skipE2ETests = true
		os.Exit(0)
	}

	namespace = testNamespacePrefix + testutil.RandSeq(5)
	if err := testutil.CreateNamespace(namespace, k8sClient); err != nil {
		panic(err)
	}

	cfg = config.NewDefault()
	cfg.AutoReloadAll = false

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		skipDeploymentConfigTests = true
	} else {
		nopLog := ctrl.Log.WithName("dc-detection")
		if openshift.HasDeploymentConfigSupport(discoveryClient, nopLog) {
			cfg.DeploymentConfigEnabled = true
			osClient, err = testutil.NewOpenshiftClient(restCfg)
			if err != nil {
				skipDeploymentConfigTests = true
			}
		} else {
			skipDeploymentConfigTests = true
		}
	}

	_, cancelManager = startManagerWithConfig(cfg, restCfg)

	code := m.Run()

	if cancelManager != nil {
		cancelManager()
		time.Sleep(2 * time.Second)
	}

	_ = testutil.DeleteNamespace(namespace, k8sClient)
	os.Exit(code)
}

func skipIfNoCluster(t *testing.T) {
	if skipE2ETests {
		t.Skip("Skipping e2e test: no Kubernetes cluster available")
	}
}

func skipIfNoDeploymentConfig(t *testing.T) {
	skipIfNoCluster(t)
	if skipDeploymentConfigTests {
		t.Skip("Skipping DeploymentConfig test: cluster does not support DeploymentConfig API")
	}
}

// TestConfigMapUpdate tests that updating a ConfigMap triggers a workload reload.
func TestConfigMapUpdate(t *testing.T) {
	f := newFixture(t, "cm-update")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestSecretUpdate tests that updating a Secret triggers a workload reload.
func TestSecretUpdate(t *testing.T) {
	f := newFixture(t, "secret-update")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createDeployment(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestAutoReloadAll tests the auto-reload-all feature.
func TestAutoReloadAll(t *testing.T) {
	f := newFixture(t, "auto-reload")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.Auto: "true",
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestDaemonSetReload tests that DaemonSets are reloaded when ConfigMaps change.
func TestDaemonSetReload(t *testing.T) {
	f := newFixture(t, "ds-reload")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDaemonSet(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDaemonSetReloaded(f.name)
}

// TestStatefulSetReload tests that StatefulSets are reloaded when Secrets change.
func TestStatefulSetReload(t *testing.T) {
	f := newFixture(t, "sts-reload")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createStatefulSet(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertStatefulSetReloaded(f.name)
}

// TestLabelOnlyChange tests that label-only changes don't trigger reloads.
func TestLabelOnlyChange(t *testing.T) {
	f := newFixture(t, "label-only")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMapLabel(f.name, "new-label")
	f.assertDeploymentNotReloaded(f.name, nil)
}

// TestMultipleConfigMaps tests watching multiple ConfigMaps in a single annotation.
func TestMultipleConfigMaps(t *testing.T) {
	f := newFixture(t, "multi-cm")
	defer f.cleanup()

	cm1 := f.name + "-a"
	cm2 := f.name + "-b"

	f.createConfigMap(cm1, "data-a")
	f.createConfigMap(cm2, "data-b")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: cm1 + "," + cm2,
		},
	)
	f.waitForReady()

	f.updateConfigMap(cm1, "updated-data-a")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestAutoAnnotationDisabled tests that auto: "false" disables auto-reload.
func TestAutoAnnotationDisabled(t *testing.T) {
	f := newFixture(t, "auto-disabled")
	defer f.cleanup()

	testCfg := config.NewDefault()
	testCfg.AutoReloadAll = true

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			testCfg.Annotations.Auto: "false",
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentNotReloaded(f.name, testCfg)
}

// TestAutoWithExplicitConfigMapAnnotation tests that a deployment with auto=true
// also reloads when an explicitly annotated (non-referenced) ConfigMap changes.
func TestAutoWithExplicitConfigMapAnnotation(t *testing.T) {
	f := newFixture(t, "auto-explicit-cm")
	defer f.cleanup()

	referencedCM := f.name + "-ref"
	explicitCM := f.name + "-explicit"

	f.createConfigMap(referencedCM, "referenced-data")
	f.createConfigMap(explicitCM, "explicit-data")
	f.createDeployment(
		referencedCM, true, map[string]string{
			cfg.Annotations.Auto:            "true",
			cfg.Annotations.ConfigmapReload: explicitCM,
		},
	)
	f.waitForReady()

	f.updateConfigMap(explicitCM, "updated-explicit-data")
	f.assertDeploymentReloaded(referencedCM, nil)
}

// TestAutoWithExplicitSecretAnnotation tests that a deployment with auto=true
// also reloads when an explicitly annotated (non-referenced) Secret changes.
func TestAutoWithExplicitSecretAnnotation(t *testing.T) {
	f := newFixture(t, "auto-explicit-secret")
	defer f.cleanup()

	referencedSecret := f.name + "-ref"
	explicitSecret := f.name + "-explicit"

	f.createSecret(referencedSecret, "referenced-secret")
	f.createSecret(explicitSecret, "explicit-secret")
	f.createDeployment(
		referencedSecret, false, map[string]string{
			cfg.Annotations.Auto:         "true",
			cfg.Annotations.SecretReload: explicitSecret,
		},
	)
	f.waitForReady()

	f.updateSecret(explicitSecret, "updated-explicit-secret")
	f.assertDeploymentReloaded(referencedSecret, nil)
}

// TestAutoWithBothExplicitAndReferencedChange tests that auto + explicit annotations
// work correctly when the referenced resource changes.
func TestAutoWithBothExplicitAndReferencedChange(t *testing.T) {
	f := newFixture(t, "auto-both")
	defer f.cleanup()

	referencedCM := f.name + "-ref"
	explicitCM := f.name + "-explicit"

	f.createConfigMap(referencedCM, "referenced-data")
	f.createConfigMap(explicitCM, "explicit-data")
	f.createDeployment(
		referencedCM, true, map[string]string{
			cfg.Annotations.Auto:            "true",
			cfg.Annotations.ConfigmapReload: explicitCM,
		},
	)
	f.waitForReady()

	f.updateConfigMap(referencedCM, "updated-referenced-data")
	f.assertDeploymentReloaded(referencedCM, nil)
}

// newFixtureForDeploymentConfig creates a new test fixture for DeploymentConfig tests.
func newFixtureForDeploymentConfig(t *testing.T, prefix string) *testFixture {
	t.Helper()
	skipIfNoDeploymentConfig(t)
	return &testFixture{
		t:    t,
		name: prefix + "-" + testutil.RandSeq(5),
	}
}

// TestDeploymentConfigReloadConfigMap tests that updating a ConfigMap triggers a DeploymentConfig reload.
func TestDeploymentConfigReloadConfigMap(t *testing.T) {
	f := newFixtureForDeploymentConfig(t, "dc-cm-reload")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeploymentConfig(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentConfigReloaded(f.name)
}

// TestDeploymentConfigReloadSecret tests that updating a Secret triggers a DeploymentConfig reload.
func TestDeploymentConfigReloadSecret(t *testing.T) {
	f := newFixtureForDeploymentConfig(t, "dc-secret-reload")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createDeploymentConfig(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertDeploymentConfigReloaded(f.name)
}

// TestDeploymentConfigAutoReload tests the auto-reload annotation on DeploymentConfig.
func TestDeploymentConfigAutoReload(t *testing.T) {
	f := newFixtureForDeploymentConfig(t, "dc-auto-reload")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeploymentConfig(
		f.name, true, map[string]string{
			cfg.Annotations.Auto: "true",
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentConfigReloaded(f.name)
}

// TestDeploymentPausePeriod tests the pause-period annotation on Deployment.
// It verifies that after a reload, the deployment is paused and then unpaused after the period expires.
func TestDeploymentPausePeriod(t *testing.T) {
	f := newFixture(t, "pause-period")
	defer f.cleanup()

	pausePeriod := "10s"

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
			cfg.Annotations.PausePeriod:     pausePeriod,
		},
	)
	f.waitForReady()
	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentReloaded(f.name, nil)
	f.assertDeploymentPaused(f.name)
	f.assertDeploymentHasPausedAtAnnotation(f.name)
	t.Log("Waiting for pause period to expire...")
	f.assertDeploymentUnpaused(f.name, 20*time.Second)
}

// TestDeploymentPausePeriodWithAutoReload tests pause-period with auto reload annotation.
func TestDeploymentPausePeriodWithAutoReload(t *testing.T) {
	f := newFixture(t, "pause-auto")
	defer f.cleanup()

	pausePeriod := "10s"

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.Auto:        "true",
			cfg.Annotations.PausePeriod: pausePeriod,
		},
	)
	f.waitForReady()
	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentReloaded(f.name, nil)
	f.assertDeploymentPaused(f.name)
	t.Log("Waiting for pause period to expire...")
	f.assertDeploymentUnpaused(f.name, 20*time.Second)
}

// TestDeploymentNoPauseWithoutAnnotation tests that deployments without pause-period are not paused.
func TestDeploymentNoPauseWithoutAnnotation(t *testing.T) {
	f := newFixture(t, "no-pause")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()
	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentReloaded(f.name, nil)

	time.Sleep(3 * time.Second)
	deploy, err := k8sClient.AppsV1().Deployments(namespace).Get(context.Background(), f.name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}
	if deploy.Spec.Paused {
		t.Errorf("Deployment should NOT be paused without pause-period annotation")
	}
}

// TestDaemonSetSecretReload tests that DaemonSets are reloaded when Secrets change.
func TestDaemonSetSecretReload(t *testing.T) {
	f := newFixture(t, "ds-secret-reload")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createDaemonSet(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertDaemonSetReloaded(f.name)
}

// TestStatefulSetConfigMapReload tests that StatefulSets are reloaded when ConfigMaps change.
func TestStatefulSetConfigMapReload(t *testing.T) {
	f := newFixture(t, "sts-cm-reload")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createStatefulSet(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertStatefulSetReloaded(f.name)
}

// TestSecretLabelOnlyChange tests that Secret label-only changes don't trigger reloads.
func TestSecretLabelOnlyChange(t *testing.T) {
	f := newFixture(t, "secret-label-only")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createDeployment(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecretLabel(f.name, "new-label")
	f.assertDeploymentNotReloaded(f.name, nil)
}

// TestDaemonSetLabelOnlyChange tests that ConfigMap label-only changes don't trigger DaemonSet reloads.
func TestDaemonSetLabelOnlyChange(t *testing.T) {
	f := newFixture(t, "ds-label-only")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDaemonSet(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMapLabel(f.name, "new-label")
	f.assertDaemonSetNotReloaded(f.name)
}

// TestStatefulSetLabelOnlyChange tests that Secret label-only changes don't trigger StatefulSet reloads.
func TestStatefulSetLabelOnlyChange(t *testing.T) {
	f := newFixture(t, "sts-label-only")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createStatefulSet(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecretLabel(f.name, "new-label")
	f.assertStatefulSetNotReloaded(f.name)
}

// TestMultipleConfigMapUpdates tests that multiple updates to a ConfigMap all trigger reloads correctly.
func TestMultipleConfigMapUpdates(t *testing.T) {
	f := newFixture(t, "multi-update")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data-1")
	f.assertDeploymentReloaded(f.name, nil)

	time.Sleep(2 * time.Second)

	f.updateConfigMap(f.name, "updated-data-2")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestMultipleSecretUpdates tests that multiple updates to a Secret all trigger reloads correctly.
func TestMultipleSecretUpdates(t *testing.T) {
	f := newFixture(t, "multi-secret-update")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createDeployment(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret-1")
	f.assertDeploymentReloaded(f.name, nil)

	time.Sleep(2 * time.Second)

	f.updateSecret(f.name, "updated-secret-2")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestSecretOnlyAuto tests the secret-only auto annotation (secret.reloader.stakater.com/auto).
func TestSecretOnlyAuto(t *testing.T) {
	f := newFixture(t, "secret-auto")
	defer f.cleanup()

	secretName := f.name + "-secret"
	cmName := f.name + "-cm"

	f.createSecret(secretName, "initial-secret")
	f.createConfigMap(cmName, "initial-data")

	_, err := testutil.CreateDeploymentWithBoth(
		k8sClient, f.name, namespace, cmName, secretName, map[string]string{
			cfg.Annotations.SecretAuto: "true",
		},
	)
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: f.name, kind: "deployment"})
	f.waitForReady()

	f.updateSecret(secretName, "updated-secret")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestConfigMapOnlyAuto tests the configmap-only auto annotation (configmap.reloader.stakater.com/auto).
func TestConfigMapOnlyAuto(t *testing.T) {
	f := newFixture(t, "cm-auto")
	defer f.cleanup()

	secretName := f.name + "-secret"
	cmName := f.name + "-cm"

	f.createSecret(secretName, "initial-secret")
	f.createConfigMap(cmName, "initial-data")

	_, err := testutil.CreateDeploymentWithBoth(
		k8sClient, f.name, namespace, cmName, secretName, map[string]string{
			cfg.Annotations.ConfigmapAuto: "true",
		},
	)
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: f.name, kind: "deployment"})
	f.waitForReady()

	f.updateConfigMap(cmName, "updated-data")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestSearchMatchAnnotations tests the search + match annotation pattern.
func TestSearchMatchAnnotations(t *testing.T) {
	f := newFixture(t, "search-match")
	defer f.cleanup()

	cm, err := testutil.CreateConfigMapWithAnnotations(
		k8sClient, namespace, f.name, "initial-data", map[string]string{
			cfg.Annotations.Match: "true",
		},
	)
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}
	f.configMaps = append(f.configMaps, cm.Name)

	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.Search: "true",
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentReloaded(f.name, nil)
}

// TestSearchWithoutMatch tests that search annotation without match doesn't trigger reload.
func TestSearchWithoutMatch(t *testing.T) {
	f := newFixture(t, "search-no-match")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")

	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.Search: "true",
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentNotReloaded(f.name, nil)
}

// TestResourceIgnore tests the ignore annotation on ConfigMap/Secret.
func TestResourceIgnore(t *testing.T) {
	f := newFixture(t, "ignore")
	defer f.cleanup()

	cm, err := testutil.CreateConfigMapWithAnnotations(
		k8sClient, namespace, f.name, "initial-data", map[string]string{
			cfg.Annotations.Ignore: "true",
		},
	)
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}
	f.configMaps = append(f.configMaps, cm.Name)

	f.createDeployment(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertDeploymentNotReloaded(f.name, nil)
}

// createCronJob creates a CronJob and registers it for cleanup.
func (f *testFixture) createCronJob(name string, useConfigMap bool, annotations map[string]string) {
	f.t.Helper()
	_, err := testutil.CreateCronJob(k8sClient, name, namespace, useConfigMap, annotations)
	if err != nil {
		f.t.Fatalf("Failed to create CronJob %s: %v", name, err)
	}
	f.workloads = append(f.workloads, workloadInfo{name: name, kind: "cronjob"})
}

// assertCronJobTriggeredJob asserts that a CronJob triggered a new Job.
func (f *testFixture) assertCronJobTriggeredJob(name string) {
	f.t.Helper()
	triggered, err := testutil.WaitForCronJobTriggeredJob(k8sClient, namespace, name, waitTimeout)
	if err != nil {
		f.t.Fatalf("Error waiting for CronJob %s to trigger Job: %v", name, err)
	}
	if !triggered {
		f.t.Errorf("CronJob %s did not trigger a Job after resource change", name)
	}
}

// TestCronJobConfigMapReload tests that updating a ConfigMap triggers a CronJob to create a new Job.
func TestCronJobConfigMapReload(t *testing.T) {
	f := newFixture(t, "cj-cm")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createCronJob(
		f.name, true, map[string]string{
			cfg.Annotations.ConfigmapReload: f.name,
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertCronJobTriggeredJob(f.name)
}

// TestCronJobSecretReload tests that updating a Secret triggers a CronJob to create a new Job.
func TestCronJobSecretReload(t *testing.T) {
	f := newFixture(t, "cj-secret")
	defer f.cleanup()

	f.createSecret(f.name, "initial-secret")
	f.createCronJob(
		f.name, false, map[string]string{
			cfg.Annotations.SecretReload: f.name,
		},
	)
	f.waitForReady()

	f.updateSecret(f.name, "updated-secret")
	f.assertCronJobTriggeredJob(f.name)
}

// TestCronJobAutoReload tests that CronJob with auto annotation triggers a Job on ConfigMap update.
func TestCronJobAutoReload(t *testing.T) {
	f := newFixture(t, "cj-auto")
	defer f.cleanup()

	f.createConfigMap(f.name, "initial-data")
	f.createCronJob(
		f.name, true, map[string]string{
			cfg.Annotations.Auto: "true",
		},
	)
	f.waitForReady()

	f.updateConfigMap(f.name, "updated-data")
	f.assertCronJobTriggeredJob(f.name)
}

// startManagerWithConfig creates and starts a controller-runtime manager for e2e testing.
func startManagerWithConfig(cfg *config.Config, restConfig *rest.Config) (manager.Manager, context.CancelFunc) {
	collectors := metrics.NewCollectors()
	mgr, err := controller.NewManagerWithRestConfig(
		controller.ManagerOptions{
			Config:     cfg,
			Log:        ctrl.Log.WithName("test-manager"),
			Collectors: &collectors,
		}, restConfig,
	)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	if err := controller.SetupReconcilers(mgr, cfg, ctrl.Log.WithName("test-reconcilers"), &collectors); err != nil {
		log.Fatalf("Failed to setup reconcilers: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := controller.RunManager(ctx, mgr, ctrl.Log.WithName("test-runner")); err != nil {
			log.Printf("Manager exited: %v", err)
		}
	}()

	time.Sleep(3 * time.Second)
	return mgr, cancel
}
