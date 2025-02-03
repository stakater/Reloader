package callbacks

import (
	"context"
	"fmt"
	"testing"

	"github.com/stakater/Reloader/pkg/kube"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	fakeargoclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/fake"
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

func TestResourceItems(t *testing.T) {
	fixtures := newTestFixtures()
	clients := setupTestClients()

	tests := []struct {
		name          string
		createFunc    func(kube.Clients, string) error
		getItemsFunc  func(kube.Clients, string) []runtime.Object
		expectedCount int
	}{
		{
			name:          "Deployments",
			createFunc:    createTestDeployments,
			getItemsFunc:  GetDeploymentItems,
			expectedCount: 2,
		},
		{
			name:          "CronJobs",
			createFunc:    createTestCronJobs,
			getItemsFunc:  GetCronJobItems,
			expectedCount: 2,
		},
		{
			name:          "Jobs",
			createFunc:    createTestJobs,
			getItemsFunc:  GetJobItems,
			expectedCount: 2,
		},
		{
			name:          "DaemonSets",
			createFunc:    createTestDaemonSets,
			getItemsFunc:  GetDaemonSetItems,
			expectedCount: 2,
		},
		{
			name:          "StatefulSets",
			createFunc:    createTestStatefulSets,
			getItemsFunc:  GetStatefulSetItems,
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
		{"Deployment", &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, GetDeploymentAnnotations},
		{"CronJob", &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, GetCronJobAnnotations},
		{"Job", &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, GetJobAnnotations},
		{"DaemonSet", &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, GetDaemonSetAnnotations},
		{"StatefulSet", &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, GetStatefulSetAnnotations},
		{"Rollout", &argorolloutv1alpha1.Rollout{ObjectMeta: metav1.ObjectMeta{Annotations: testAnnotations}}, GetRolloutAnnotations},
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
		{"Deployment", createResourceWithPodAnnotations(&appsv1.Deployment{}, testAnnotations), GetDeploymentPodAnnotations},
		{"CronJob", createResourceWithPodAnnotations(&batchv1.CronJob{}, testAnnotations), GetCronJobPodAnnotations},
		{"Job", createResourceWithPodAnnotations(&batchv1.Job{}, testAnnotations), GetJobPodAnnotations},
		{"DaemonSet", createResourceWithPodAnnotations(&appsv1.DaemonSet{}, testAnnotations), GetDaemonSetPodAnnotations},
		{"StatefulSet", createResourceWithPodAnnotations(&appsv1.StatefulSet{}, testAnnotations), GetStatefulSetPodAnnotations},
		{"Rollout", createResourceWithPodAnnotations(&argorolloutv1alpha1.Rollout{}, testAnnotations), GetRolloutPodAnnotations},
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
		{"Deployment", createResourceWithContainers(&appsv1.Deployment{}, fixtures.defaultContainers), GetDeploymentContainers},
		{"DaemonSet", createResourceWithContainers(&appsv1.DaemonSet{}, fixtures.defaultContainers), GetDaemonSetContainers},
		{"StatefulSet", createResourceWithContainers(&appsv1.StatefulSet{}, fixtures.defaultContainers), GetStatefulSetContainers},
		{"CronJob", createResourceWithContainers(&batchv1.CronJob{}, fixtures.defaultContainers), GetCronJobContainers},
		{"Job", createResourceWithContainers(&batchv1.Job{}, fixtures.defaultContainers), GetJobContainers},
		{"Rollout", createResourceWithContainers(&argorolloutv1alpha1.Rollout{}, fixtures.defaultContainers), GetRolloutContainers},
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
		{"Deployment", createResourceWithInitContainers(&appsv1.Deployment{}, fixtures.defaultInitContainers), GetDeploymentInitContainers},
		{"DaemonSet", createResourceWithInitContainers(&appsv1.DaemonSet{}, fixtures.defaultInitContainers), GetDaemonSetInitContainers},
		{"StatefulSet", createResourceWithInitContainers(&appsv1.StatefulSet{}, fixtures.defaultInitContainers), GetStatefulSetInitContainers},
		{"CronJob", createResourceWithInitContainers(&batchv1.CronJob{}, fixtures.defaultInitContainers), GetCronJobInitContainers},
		{"Job", createResourceWithInitContainers(&batchv1.Job{}, fixtures.defaultInitContainers), GetJobInitContainers},
		{"Rollout", createResourceWithInitContainers(&argorolloutv1alpha1.Rollout{}, fixtures.defaultInitContainers), GetRolloutInitContainers},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, fixtures.defaultInitContainers, tt.getFunc(tt.resource))
		})
	}
}

func TestUpdateResources(t *testing.T) {
	fixtures := newTestFixtures()
	clients := setupTestClients()

	tests := []struct {
		name       string
		createFunc func(kube.Clients, string, string) (runtime.Object, error)
		updateFunc func(kube.Clients, string, runtime.Object) error
	}{
		{"Deployment", createTestDeploymentWithAnnotations, UpdateDeployment},
		{"DaemonSet", createTestDaemonSetWithAnnotations, UpdateDaemonSet},
		{"StatefulSet", createTestStatefulSetWithAnnotations, UpdateStatefulSet},
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
	clients := setupTestClients()

	cronJob, err := createTestCronJobWithAnnotations(clients, fixtures.namespace, "1")
	assert.NoError(t, err)

	err = CreateJobFromCronjob(clients, fixtures.namespace, cronJob.(*batchv1.CronJob))
	assert.NoError(t, err)
}

func TestReCreateJobFromJob(t *testing.T) {
	fixtures := newTestFixtures()
	clients := setupTestClients()

	job, err := createTestJobWithAnnotations(clients, fixtures.namespace, "1")
	assert.NoError(t, err)

	err = ReCreateJobFromjob(clients, fixtures.namespace, job.(*batchv1.Job))
	assert.NoError(t, err)
}

func TestGetVolumes(t *testing.T) {
	fixtures := newTestFixtures()

	tests := []struct {
		name     string
		resource runtime.Object
		getFunc  func(runtime.Object) []v1.Volume
	}{
		{"Deployment", createResourceWithVolumes(&appsv1.Deployment{}, fixtures.defaultVolumes), GetDeploymentVolumes},
		{"CronJob", createResourceWithVolumes(&batchv1.CronJob{}, fixtures.defaultVolumes), GetCronJobVolumes},
		{"Job", createResourceWithVolumes(&batchv1.Job{}, fixtures.defaultVolumes), GetJobVolumes},
		{"DaemonSet", createResourceWithVolumes(&appsv1.DaemonSet{}, fixtures.defaultVolumes), GetDaemonSetVolumes},
		{"StatefulSet", createResourceWithVolumes(&appsv1.StatefulSet{}, fixtures.defaultVolumes), GetStatefulSetVolumes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, fixtures.defaultVolumes, tt.getFunc(tt.resource))
		})
	}
}

// Helper functions

// Helper functions for creating test resources

func createTestDeployments(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-deployment-%d", i),
				Namespace: namespace,
			},
		}
		_, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestCronJobs(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-cronjob-%d", i),
				Namespace: namespace,
			},
		}
		_, err := clients.KubernetesClient.BatchV1().CronJobs(namespace).Create(context.TODO(), cronJob, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestJobs(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-job-%d", i),
				Namespace: namespace,
			},
		}
		_, err := clients.KubernetesClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestDaemonSets(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		daemonSet := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-daemonset-%d", i),
				Namespace: namespace,
			},
		}
		_, err := clients.KubernetesClient.AppsV1().DaemonSets(namespace).Create(context.TODO(), daemonSet, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func createTestStatefulSets(clients kube.Clients, namespace string) error {
	for i := 1; i <= 2; i++ {
		statefulSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-statefulset-%d", i),
				Namespace: namespace,
			},
		}
		_, err := clients.KubernetesClient.AppsV1().StatefulSets(namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
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
