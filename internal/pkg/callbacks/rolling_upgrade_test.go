package callbacks_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	fakeargoclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/fake"

	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/kube"
)

var (
	clients = setupTestClients()
)

type testFixtures struct {
	defaultContainers     []v1.Container
	defaultInitContainers []v1.Container
	defaultVolumes        []v1.Volume
	namespace             string
}

func newTestFixtures() testFixtures {
	return testFixtures{
		defaultContainers:     []v1.Container{{Name: "container1"}, {Name: "container2"}},
		defaultInitContainers: []v1.Container{{Name: "init-container1"}, {Name: "init-container2"}},
		defaultVolumes:        []v1.Volume{{Name: "volume1"}, {Name: "volume2"}},
		namespace:             "default",
	}
}

func setupTestClients() kube.Clients {
	return kube.Clients{
		KubernetesClient:  fake.NewSimpleClientset(),
		ArgoRolloutClient: fakeargoclientset.NewSimpleClientset(),
	}
}

// TestUpdateRollout test update rollout strategy annotation
func TestUpdateRollout(t *testing.T) {
	namespace := "test-ns"

	cases := map[string]struct {
		name      string
		strategy  string
		isRestart bool
	}{
		"test-without-strategy": {
			name:      "defaults to rollout strategy",
			strategy:  "",
			isRestart: false,
		},
		"test-with-restart-strategy": {
			name:      "triggers a restart strategy",
			strategy:  "restart",
			isRestart: true,
		},
		"test-with-rollout-strategy": {
			name:      "triggers a rollout strategy",
			strategy:  "rollout",
			isRestart: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rollout, err := testutil.CreateRollout(
				clients.ArgoRolloutClient, name, namespace,
				map[string]string{options.RolloutStrategyAnnotation: tc.strategy},
			)
			if err != nil {
				t.Errorf("creating rollout: %v", err)
			}
			modifiedChan := watchRollout(rollout.Name, namespace)

			err = callbacks.UpdateRollout(clients, namespace, rollout)
			if err != nil {
				t.Errorf("updating rollout: %v", err)
			}
			rollout, err = clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(
				namespace).Get(context.TODO(), rollout.Name, meta_v1.GetOptions{})

			if err != nil {
				t.Errorf("getting rollout: %v", err)
			}
			if isRestartStrategy(rollout) == tc.isRestart {
				t.Errorf("Should not be a restart strategy")
			}
			select {
			case <-modifiedChan:
				// object has been modified
			case <-time.After(1 * time.Second):
				t.Errorf("Rollout has not been updated")
			}
		})
	}
}

func TestResourceItems(t *testing.T) {
	fixtures := newTestFixtures()

	tests := []struct {
		name          string
		createFunc    func(kube.Clients, string) error
		getItemsFunc  func(kube.Clients, string) []runtime.Object
		expectedCount int
	}{
		{
			name:          "Deployments",
			createFunc:    createTestDeployments,
			getItemsFunc:  callbacks.GetDeploymentItems,
			expectedCount: 2,
		},
		{
			name:          "CronJobs",
			createFunc:    createTestCronJobs,
			getItemsFunc:  callbacks.GetCronJobItems,
			expectedCount: 2,
		},
		{
			name:          "Jobs",
			createFunc:    createTestJobs,
			getItemsFunc:  callbacks.GetJobItems,
			expectedCount: 2,
		},
		{
			name:          "DaemonSets",
			createFunc:    createTestDaemonSets,
			getItemsFunc:  callbacks.GetDaemonSetItems,
			expectedCount: 2,
		},
		{
			name:          "StatefulSets",
			createFunc:    createTestStatefulSets,
			getItemsFunc:  callbacks.GetStatefulSetItems,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createFunc(clients, fixtures.namespace)
			assert.NoError(t, err)

			items := tt.getItemsFunc(clients, fixtures.namespace)
			assert.Equal(t, tt.expectedCount, len(items))
		})
	}
}

func TestGetAnnotations(t *testing.T) {
	testAnnotations := map[string]string{"version": "1"}

	tests := []struct {
		name     string
		resource runtime.Object
		getFunc  func(runtime.Object) map[string]string
	}{
		{"Deployment", &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, callbacks.GetDeploymentAnnotations},
		{"CronJob", &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, callbacks.GetCronJobAnnotations},
		{"Job", &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, callbacks.GetJobAnnotations},
		{"DaemonSet", &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, callbacks.GetDaemonSetAnnotations},
		{"StatefulSet", &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, callbacks.GetStatefulSetAnnotations},
		{"Rollout", &argorolloutv1alpha1.Rollout{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, callbacks.GetRolloutAnnotations},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, testAnnotations, tt.getFunc(tt.resource))
		})
	}
}

func TestGetPodAnnotations(t *testing.T) {
	testAnnotations := map[string]string{"version": "1"}

	tests := []struct {
		name     string
		resource runtime.Object
		getFunc  func(runtime.Object) map[string]string
	}{
		{"Deployment", createResourceWithPodAnnotations(&appsv1.Deployment{}, testAnnotations), callbacks.GetDeploymentPodAnnotations},
		{"CronJob", createResourceWithPodAnnotations(&batchv1.CronJob{}, testAnnotations), callbacks.GetCronJobPodAnnotations},
		{"Job", createResourceWithPodAnnotations(&batchv1.Job{}, testAnnotations), callbacks.GetJobPodAnnotations},
		{"DaemonSet", createResourceWithPodAnnotations(&appsv1.DaemonSet{}, testAnnotations), callbacks.GetDaemonSetPodAnnotations},
		{"StatefulSet", createResourceWithPodAnnotations(&appsv1.StatefulSet{}, testAnnotations), callbacks.GetStatefulSetPodAnnotations},
		{"Rollout", createResourceWithPodAnnotations(&argorolloutv1alpha1.Rollout{}, testAnnotations), callbacks.GetRolloutPodAnnotations},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, testAnnotations, tt.getFunc(tt.resource))
		})
	}
}

func TestGetContainers(t *testing.T) {
	fixtures := newTestFixtures()

	tests := []struct {
		name     string
		resource runtime.Object
		getFunc  func(runtime.Object) []v1.Container
	}{
		{"Deployment", createResourceWithContainers(&appsv1.Deployment{}, fixtures.defaultContainers), callbacks.GetDeploymentContainers},
		{"DaemonSet", createResourceWithContainers(&appsv1.DaemonSet{}, fixtures.defaultContainers), callbacks.GetDaemonSetContainers},
		{"StatefulSet", createResourceWithContainers(&appsv1.StatefulSet{}, fixtures.defaultContainers), callbacks.GetStatefulSetContainers},
		{"CronJob", createResourceWithContainers(&batchv1.CronJob{}, fixtures.defaultContainers), callbacks.GetCronJobContainers},
		{"Job", createResourceWithContainers(&batchv1.Job{}, fixtures.defaultContainers), callbacks.GetJobContainers},
		{"Rollout", createResourceWithContainers(&argorolloutv1alpha1.Rollout{}, fixtures.defaultContainers), callbacks.GetRolloutContainers},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, fixtures.defaultContainers, tt.getFunc(tt.resource))
		})
	}
}

func TestGetInitContainers(t *testing.T) {
	fixtures := newTestFixtures()

	tests := []struct {
		name     string
		resource runtime.Object
		getFunc  func(runtime.Object) []v1.Container
	}{
		{"Deployment", createResourceWithInitContainers(&appsv1.Deployment{}, fixtures.defaultInitContainers), callbacks.GetDeploymentInitContainers},
		{"DaemonSet", createResourceWithInitContainers(&appsv1.DaemonSet{}, fixtures.defaultInitContainers), callbacks.GetDaemonSetInitContainers},
		{"StatefulSet", createResourceWithInitContainers(&appsv1.StatefulSet{}, fixtures.defaultInitContainers), callbacks.GetStatefulSetInitContainers},
		{"CronJob", createResourceWithInitContainers(&batchv1.CronJob{}, fixtures.defaultInitContainers), callbacks.GetCronJobInitContainers},
		{"Job", createResourceWithInitContainers(&batchv1.Job{}, fixtures.defaultInitContainers), callbacks.GetJobInitContainers},
		{"Rollout", createResourceWithInitContainers(&argorolloutv1alpha1.Rollout{}, fixtures.defaultInitContainers), callbacks.GetRolloutInitContainers},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, fixtures.defaultInitContainers, tt.getFunc(tt.resource))
		})
	}
}

func TestUpdateResources(t *testing.T) {
	fixtures := newTestFixtures()

	tests := []struct {
		name       string
		createFunc func(kube.Clients, string, string) (runtime.Object, error)
		updateFunc func(kube.Clients, string, runtime.Object) error
	}{
		{"Deployment", createTestDeploymentWithAnnotations, callbacks.UpdateDeployment},
		{"DaemonSet", createTestDaemonSetWithAnnotations, callbacks.UpdateDaemonSet},
		{"StatefulSet", createTestStatefulSetWithAnnotations, callbacks.UpdateStatefulSet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, err := tt.createFunc(clients, fixtures.namespace, "1")
			assert.NoError(t, err)

			err = tt.updateFunc(clients, fixtures.namespace, resource)
			assert.NoError(t, err)
		})
	}
}

func TestCreateJobFromCronjob(t *testing.T) {
	fixtures := newTestFixtures()

	cronJob, err := createTestCronJobWithAnnotations(clients, fixtures.namespace, "1")
	assert.NoError(t, err)

	err = callbacks.CreateJobFromCronjob(clients, fixtures.namespace, cronJob.(*batchv1.CronJob))
	assert.NoError(t, err)
}

func TestReCreateJobFromJob(t *testing.T) {
	fixtures := newTestFixtures()

	job, err := createTestJobWithAnnotations(clients, fixtures.namespace, "1")
	assert.NoError(t, err)

	err = callbacks.ReCreateJobFromjob(clients, fixtures.namespace, job.(*batchv1.Job))
	assert.NoError(t, err)
}

func TestGetVolumes(t *testing.T) {
	fixtures := newTestFixtures()

	tests := []struct {
		name     string
		resource runtime.Object
		getFunc  func(runtime.Object) []v1.Volume
	}{
		{"Deployment", createResourceWithVolumes(&appsv1.Deployment{}, fixtures.defaultVolumes), callbacks.GetDeploymentVolumes},
		{"CronJob", createResourceWithVolumes(&batchv1.CronJob{}, fixtures.defaultVolumes), callbacks.GetCronJobVolumes},
		{"Job", createResourceWithVolumes(&batchv1.Job{}, fixtures.defaultVolumes), callbacks.GetJobVolumes},
		{"DaemonSet", createResourceWithVolumes(&appsv1.DaemonSet{}, fixtures.defaultVolumes), callbacks.GetDaemonSetVolumes},
		{"StatefulSet", createResourceWithVolumes(&appsv1.StatefulSet{}, fixtures.defaultVolumes), callbacks.GetStatefulSetVolumes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, fixtures.defaultVolumes, tt.getFunc(tt.resource))
		})
	}
}

// Helper functions

func isRestartStrategy(rollout *argorolloutv1alpha1.Rollout) bool {
	return rollout.Spec.RestartAt == nil
}

func watchRollout(name, namespace string) chan interface{} {
	timeOut := int64(1)
	modifiedChan := make(chan interface{})
	watcher, _ := clients.ArgoRolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Watch(context.Background(), meta_v1.ListOptions{TimeoutSeconds: &timeOut})
	go watchModified(watcher, name, modifiedChan)
	return modifiedChan
}

func watchModified(watcher watch.Interface, name string, modifiedChan chan interface{}) {
	for event := range watcher.ResultChan() {
		item := event.Object.(*argorolloutv1alpha1.Rollout)
		if item.Name == name {
			switch event.Type {
			case watch.Modified:
				modifiedChan <- nil
			}
			return
		}
	}
}

func createTestDeployments(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		_, err := testutil.CreateDeployment(clients.KubernetesClient, fmt.Sprintf("test-deployment-%d", i), namespace, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestCronJobs(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		_, err := testutil.CreateCronJob(clients.KubernetesClient, fmt.Sprintf("test-cron-%d", i), namespace, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestJobs(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		_, err := testutil.CreateJob(clients.KubernetesClient, fmt.Sprintf("test-job-%d", i), namespace, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestDaemonSets(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		_, err := testutil.CreateDaemonSet(clients.KubernetesClient, fmt.Sprintf("test-daemonset-%d", i), namespace, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestStatefulSets(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		_, err := testutil.CreateStatefulSet(clients.KubernetesClient, fmt.Sprintf("test-statefulset-%d", i), namespace, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func createResourceWithPodAnnotations(obj runtime.Object, annotations map[string]string) runtime.Object {
	switch v := obj.(type) {
	case *appsv1.Deployment:
		v.Spec.Template.ObjectMeta.Annotations = annotations
	case *appsv1.DaemonSet:
		v.Spec.Template.ObjectMeta.Annotations = annotations
	case *appsv1.StatefulSet:
		v.Spec.Template.ObjectMeta.Annotations = annotations
	case *batchv1.CronJob:
		v.Spec.JobTemplate.Spec.Template.ObjectMeta.Annotations = annotations
	case *batchv1.Job:
		v.Spec.Template.ObjectMeta.Annotations = annotations
	case *argorolloutv1alpha1.Rollout:
		v.Spec.Template.ObjectMeta.Annotations = annotations
	}
	return obj
}

func createResourceWithContainers(obj runtime.Object, containers []v1.Container) runtime.Object {
	switch v := obj.(type) {
	case *appsv1.Deployment:
		v.Spec.Template.Spec.Containers = containers
	case *appsv1.DaemonSet:
		v.Spec.Template.Spec.Containers = containers
	case *appsv1.StatefulSet:
		v.Spec.Template.Spec.Containers = containers
	case *batchv1.CronJob:
		v.Spec.JobTemplate.Spec.Template.Spec.Containers = containers
	case *batchv1.Job:
		v.Spec.Template.Spec.Containers = containers
	case *argorolloutv1alpha1.Rollout:
		v.Spec.Template.Spec.Containers = containers
	}
	return obj
}

func createResourceWithInitContainers(obj runtime.Object, initContainers []v1.Container) runtime.Object {
	switch v := obj.(type) {
	case *appsv1.Deployment:
		v.Spec.Template.Spec.InitContainers = initContainers
	case *appsv1.DaemonSet:
		v.Spec.Template.Spec.InitContainers = initContainers
	case *appsv1.StatefulSet:
		v.Spec.Template.Spec.InitContainers = initContainers
	case *batchv1.CronJob:
		v.Spec.JobTemplate.Spec.Template.Spec.InitContainers = initContainers
	case *batchv1.Job:
		v.Spec.Template.Spec.InitContainers = initContainers
	case *argorolloutv1alpha1.Rollout:
		v.Spec.Template.Spec.InitContainers = initContainers
	}
	return obj
}

func createResourceWithVolumes(obj runtime.Object, volumes []v1.Volume) runtime.Object {
	switch v := obj.(type) {
	case *appsv1.Deployment:
		v.Spec.Template.Spec.Volumes = volumes
	case *batchv1.CronJob:
		v.Spec.JobTemplate.Spec.Template.Spec.Volumes = volumes
	case *batchv1.Job:
		v.Spec.Template.Spec.Volumes = volumes
	case *appsv1.DaemonSet:
		v.Spec.Template.Spec.Volumes = volumes
	case *appsv1.StatefulSet:
		v.Spec.Template.Spec.Volumes = volumes
	}
	return obj
}

func createTestDeploymentWithAnnotations(clients kube.Clients, namespace, version string) (runtime.Object, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-deployment",
			Namespace:   namespace,
			Annotations: map[string]string{"version": version},
		},
	}
	return clients.KubernetesClient.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
}

func createTestDaemonSetWithAnnotations(clients kube.Clients, namespace, version string) (runtime.Object, error) {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-daemonset",
			Namespace:   namespace,
			Annotations: map[string]string{"version": version},
		},
	}
	return clients.KubernetesClient.AppsV1().DaemonSets(namespace).Create(context.TODO(), daemonSet, metav1.CreateOptions{})
}

func createTestStatefulSetWithAnnotations(clients kube.Clients, namespace, version string) (runtime.Object, error) {
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-statefulset",
			Namespace:   namespace,
			Annotations: map[string]string{"version": version},
		},
	}
	return clients.KubernetesClient.AppsV1().StatefulSets(namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
}

func createTestCronJobWithAnnotations(clients kube.Clients, namespace, version string) (runtime.Object, error) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cronjob",
			Namespace:   namespace,
			Annotations: map[string]string{"version": version},
		},
	}
	return clients.KubernetesClient.BatchV1().CronJobs(namespace).Create(context.TODO(), cronJob, metav1.CreateOptions{})
}

func createTestJobWithAnnotations(clients kube.Clients, namespace, version string) (runtime.Object, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-job",
			Namespace:   namespace,
			Annotations: map[string]string{"version": version},
		},
	}
	return clients.KubernetesClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
}
