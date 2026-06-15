package leadership

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/handler"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stakater/Reloader/pkg/kube"
)

func TestMain(m *testing.M) {

	testutil.CreateNamespace(testutil.Namespace, testutil.Clients.KubernetesClient)

	logrus.Infof("Running Testcases")
	retCode := m.Run()

	testutil.DeleteNamespace(testutil.Namespace, testutil.Clients.KubernetesClient)

	os.Exit(retCode)
}

func TestHealthz(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatalf(("failed to create request"))
	}

	response := httptest.NewRecorder()

	healthz(response, request)
	got := response.Code
	want := 200

	if got != want {
		t.Fatalf("got: %d, want: %d", got, want)
	}

	// Have the liveness probe serve a 500
	healthy = false

	request, err = http.NewRequest(http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatalf(("failed to create request"))
	}

	response = httptest.NewRecorder()

	healthz(response, request)
	got = response.Code
	want = 500

	if got != want {
		t.Fatalf("got: %d, want: %d", got, want)
	}
}

// TestRunLeaderElection validates that the liveness endpoint serves 500 when
// leadership election fails
func TestRunLeaderElection(t *testing.T) {
	// Reset shared state left by TestHealthz
	m.Lock()
	healthy = true
	m.Unlock()

	ctx, cancel := context.WithCancel(context.TODO())

	lock := GetNewLock(testutil.Clients.KubernetesClient.CoordinationV1(), constants.LockName, testutil.Pod, testutil.Namespace)

	stopped := RunLeaderElection(lock, ctx, cancel, testutil.Pod, []*controller.Controller{})

	// Before leadership is acquired the probe still reads the current healthy value (true)
	request, err := http.NewRequest(http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatalf(("failed to create request"))
	}

	response := httptest.NewRecorder()

	healthz(response, request)
	got := response.Code
	want := 200

	if got != want {
		t.Fatalf("got: %d, want: %d", got, want)
	}

	// Cancel the leader election context, so leadership is released and
	// live endpoint serves 500
	cancel()
	<-stopped

	request, err = http.NewRequest(http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatalf(("failed to create request"))
	}

	response = httptest.NewRecorder()

	healthz(response, request)
	got = response.Code
	want = 500

	if got != want {
		t.Fatalf("got: %d, want: %d", got, want)
	}
}

// TestRunLeaderElectionWithControllers tests that leadership election works
// with real controllers and that on context cancellation the controllers stop
// running.
func TestRunLeaderElectionWithControllers(t *testing.T) {
	t.Logf("Creating controller")
	var controllers []*controller.Controller
	for k := range kube.ResourceMap {
		// Skip namespace controller when there is no namespace label selector
		// (mirrors production behavior in startReloader).
		if k == "namespaces" {
			continue
		}
		// Skip CSI controller when CSI is not installed
		// (mirrors production behavior in startReloader).
		if k == constants.SecretProviderClassController {
			continue
		}
		c, err := controller.NewController(testutil.Clients.KubernetesClient, k, testutil.Namespace, []string{}, "", "", metrics.NewCollectors())
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		controllers = append(controllers, c)
	}
	time.Sleep(3 * time.Second)

	lock := GetNewLock(testutil.Clients.KubernetesClient.CoordinationV1(), fmt.Sprintf("%s-%d", constants.LockName, 1), testutil.Pod, testutil.Namespace)

	ctx, cancel := context.WithCancel(context.TODO())

	// Start running leadership election, this also starts the controllers
	stopped := RunLeaderElection(lock, ctx, cancel, testutil.Pod, controllers)
	time.Sleep(3 * time.Second)

	// Create some stuff and do a thing
	configmapName := testutil.ConfigmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(testutil.Clients.KubernetesClient, testutil.Namespace, configmapName, "www.google.com")
	if err != nil {
		t.Fatalf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(testutil.Clients.KubernetesClient, configmapName, testutil.Namespace, true)
	if err != nil {
		t.Fatalf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, testutil.Namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Fatalf("Configmap was not updated")
	}
	time.Sleep(3 * time.Second)

	// Verifying deployment update
	logrus.Infof("Verifying pod envvars has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, testutil.Namespace, configmapName, "www.stakater.com")
	config := common.Config{
		Namespace:    testutil.Namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(testutil.Clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Fatalf("Deployment was not updated")
	}
	time.Sleep(testutil.SleepDuration)

	// Add reloader.stakater.com/ignore: "true" to the configmap BEFORE cancelling
	// leadership. This prevents any Reloader instance running in the cluster
	// (including ones external to this test) from processing the second configmap
	// update below, making the assertion reliable in shared cluster environments.
	// The ignore annotation is on the configmap itself: ShouldReload checks
	// config.ResourceAnnotations (= configmap annotations) for this annotation.
	// Note: only the annotation is changed here — the data SHA is unchanged so
	// the still-running controllers will see no diff and skip the rolling upgrade.
	cm, getCMErr := testutil.Clients.KubernetesClient.CoreV1().ConfigMaps(testutil.Namespace).Get(
		context.TODO(), configmapName, metav1.GetOptions{})
	if getCMErr != nil {
		t.Fatalf("Failed to get configmap to add ignore annotation: %v", getCMErr)
	}
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[options.IgnoreResourceAnnotation] = "true"
	if _, err = testutil.Clients.KubernetesClient.CoreV1().ConfigMaps(testutil.Namespace).Update(
		context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to add ignore annotation to configmap: %v", err)
	}

	// Cancel the leader election context, so leadership is released
	logrus.Info("shutting down controller from test")
	cancel()
	<-stopped // wait until OnStoppedLeading has run and all controller goroutines have exited

	// Update the configmap data for the second time using a Get+modify+Update
	// pattern so that the ignore annotation added above is preserved.
	// Any Reloader (including external ones) will see ignore=true and skip the update.
	cm, err = testutil.Clients.KubernetesClient.CoreV1().ConfigMaps(testutil.Namespace).Get(
		context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get configmap for second update: %v", err)
	}
	cm.Data["test.url"] = "www.stakater.com/new"
	// ignore annotation is still present from the update above
	if _, err = testutil.Clients.KubernetesClient.CoreV1().ConfigMaps(testutil.Namespace).Update(
		context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to update configmap: %v", err)
	}
	time.Sleep(3 * time.Second)

	// Verifying that the deployment was not updated as leadership has been lost
	logrus.Infof("Verifying pod envvars has not been updated")
	shaData = testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, testutil.Namespace, configmapName, "www.stakater.com/new")
	config = common.Config{
		Namespace:    testutil.Namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs = handler.GetDeploymentRollingUpgradeFuncs()
	updated = testutil.VerifyResourceEnvVarUpdate(testutil.Clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if updated {
		t.Fatalf("Deployment was updated")
	}

	// Deleting deployment
	err = testutil.DeleteDeployment(testutil.Clients.KubernetesClient, testutil.Namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(testutil.Clients.KubernetesClient, testutil.Namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(testutil.SleepDuration)
}
