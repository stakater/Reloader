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
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/handler"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
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
		t.Fatalf("got: %q, want: %q", got, want)
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
		t.Fatalf("got: %q, want: %q", got, want)
	}
}

// TestRunLeaderElection validates that the liveness endpoint serves 500 when
// leadership election fails
func TestRunLeaderElection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())

	lock := GetNewLock(testutil.Clients.KubernetesClient.CoordinationV1(), constants.LockName, testutil.Pod, testutil.Namespace)

	go RunLeaderElection(lock, ctx, cancel, testutil.Pod, []*controller.Controller{})

	// Liveness probe should be serving OK
	request, err := http.NewRequest(http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatalf(("failed to create request"))
	}

	response := httptest.NewRecorder()

	healthz(response, request)
	got := response.Code
	want := 500

	if got != want {
		t.Fatalf("got: %q, want: %q", got, want)
	}

	// Cancel the leader election context, so leadership is released and
	// live endpoint serves 500
	cancel()

	request, err = http.NewRequest(http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatalf(("failed to create request"))
	}

	response = httptest.NewRecorder()

	healthz(response, request)
	got = response.Code
	want = 500

	if got != want {
		t.Fatalf("got: %q, want: %q", got, want)
	}
}

// TestRunLeaderElectionWithControllers tests that leadership election works
// wiht real controllers and that on context cancellation the controllers stop
// running.
func TestRunLeaderElectionWithControllers(t *testing.T) {
	t.Logf("Creating controller")
	var controllers []*controller.Controller
	for k := range kube.ResourceMap {
		c, err := controller.NewController(testutil.Clients.KubernetesClient, k, testutil.Namespace, []string{}, map[string]string{}, map[string]string{}, metrics.NewCollectors())
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		controllers = append(controllers, c)
	}
	time.Sleep(3 * time.Second)

	lock := GetNewLock(testutil.Clients.KubernetesClient.CoordinationV1(), fmt.Sprintf("%s-%d", constants.LockName, 1), testutil.Pod, testutil.Namespace)

	ctx, cancel := context.WithCancel(context.TODO())

	// Start running leadership election, this also starts the controllers
	go RunLeaderElection(lock, ctx, cancel, testutil.Pod, controllers)
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
	config := util.Config{
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

	// Cancel the leader election context, so leadership is released
	logrus.Info("shutting down controller from test")
	cancel()
	time.Sleep(5 * time.Second)

	// Updating configmap again
	updateErr = testutil.UpdateConfigMap(configmapClient, testutil.Namespace, configmapName, "", "www.stakater.com/new")
	if updateErr != nil {
		t.Fatalf("Configmap was not updated")
	}

	// Verifying that the deployment was not updated as leadership has been lost
	logrus.Infof("Verifying pod envvars has not been updated")
	shaData = testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, testutil.Namespace, configmapName, "www.stakater.com/new")
	config = util.Config{
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
