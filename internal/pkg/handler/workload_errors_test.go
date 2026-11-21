package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stakater/Reloader/pkg/kube"
)

const deniedWorkloadName = "api"

func reloadTestPodSpec() v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  "app",
				Image: "busybox",
				EnvFrom: []v1.EnvFromSource{{
					ConfigMapRef: &v1.ConfigMapEnvSource{
						LocalObjectReference: v1.LocalObjectReference{Name: "app-config"},
					},
				}},
			}},
		},
	}
}

func reloadTestDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: map[string]string{options.ConfigmapUpdateOnChangeAnnotation: "app-config"},
		},
		Spec: appsv1.DeploymentSpec{Template: reloadTestPodSpec()},
	}
}

func reloadTestStatefulSet(name string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: map[string]string{options.ConfigmapUpdateOnChangeAnnotation: "app-config"},
		},
		Spec: appsv1.StatefulSetSpec{Template: reloadTestPodSpec()},
	}
}

// newDenyingClient returns a fake client whose admission rejects every patch
// to the "api" Deployment, the way a validating webhook would.
func newDenyingClient(objects ...runtime.Object) *testclient.Clientset {
	client := testclient.NewClientset(objects...)
	client.PrependReactor("patch", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.(k8stesting.PatchAction).GetName() != deniedWorkloadName {
			return false, nil, nil
		}
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "apps", Resource: "deployments"},
			deniedWorkloadName,
			errors.New(`admission webhook "validate.example.com" denied the request`),
		)
	})
	return client
}

func reloadTestConfig() common.Config {
	return common.GetConfigmapConfig(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "default"},
		Data:       map[string]string{"key": "new-value"},
	})
}

func hasReloaderEnvVar(t *testing.T, spec v1.PodTemplateSpec) bool {
	t.Helper()
	envVar := getEnvVarName("app-config", "CONFIGMAP")
	for _, env := range spec.Spec.Containers[0].Env {
		if env.Name == envVar {
			return true
		}
	}
	return false
}

func TestPerformActionContinuesAfterWorkloadUpdateFails(t *testing.T) {
	client := newDenyingClient(reloadTestDeployment(deniedWorkloadName), reloadTestDeployment("worker"))
	clients := kube.Clients{KubernetesClient: client}

	err := PerformAction(clients, reloadTestConfig(), GetDeploymentRollingUpgradeFuncs(), createTestCollectors(), nil, invokeReloadStrategy)
	require.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err), "expected the admission error to be returned, got %v", err)

	worker, err := client.AppsV1().Deployments("default").Get(context.TODO(), "worker", metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, hasReloaderEnvVar(t, worker.Spec.Template), "worker Deployment was not reloaded because the api Deployment update failed")
}

func TestReloadWorkloadsContinuesAfterWorkloadTypeFails(t *testing.T) {
	client := newDenyingClient(reloadTestDeployment(deniedWorkloadName), reloadTestStatefulSet("db"))
	clients := kube.Clients{KubernetesClient: client}

	err := reloadWorkloads(clients, reloadTestConfig(), createTestCollectors(), nil, invokeReloadStrategy)
	require.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err), "expected the admission error to be returned, got %v", err)

	db, err := client.AppsV1().StatefulSets("default").Get(context.TODO(), "db", metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, hasReloaderEnvVar(t, db.Spec.Template), "db StatefulSet was not reloaded because the api Deployment update failed")
}

func reloadTestCronJob(name string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			UID:         types.UID(name + "-uid"),
			Annotations: map[string]string{options.ConfigmapUpdateOnChangeAnnotation: "app-config"},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{Template: reloadTestPodSpec()},
			},
		},
	}
}

func reloadTestJob(name string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: map[string]string{options.ConfigmapUpdateOnChangeAnnotation: "app-config"},
		},
		Spec: batchv1.JobSpec{Template: reloadTestPodSpec()},
	}
}

// newJobNamingClient returns a denying client that also fills in the names of
// Jobs created with GenerateName, which the fake object tracker does not do.
func newJobNamingClient(objects ...runtime.Object) *testclient.Clientset {
	client := newDenyingClient(objects...)
	created := 0
	client.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		job := action.(k8stesting.CreateAction).GetObject().(*batchv1.Job)
		if job.Name == "" && job.GenerateName != "" {
			created++
			job.Name = job.GenerateName + string(rune('a'+created))
		}
		return false, nil, nil
	})
	return client
}

func countJobs(t *testing.T, client *testclient.Clientset) int {
	t.Helper()
	jobs, err := client.BatchV1().Jobs("default").List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	return len(jobs.Items)
}

func TestReloadWorkloadsRetryDoesNotDuplicateCronJobJobs(t *testing.T) {
	originalStrategy := options.ReloadStrategy
	t.Cleanup(func() { options.ReloadStrategy = originalStrategy })

	for _, strategy := range []string{constants.EnvVarsReloadStrategy, constants.AnnotationsReloadStrategy} {
		t.Run(strategy, func(t *testing.T) {
			options.ReloadStrategy = strategy
			client := newJobNamingClient(reloadTestDeployment(deniedWorkloadName), reloadTestCronJob("nightly"))
			clients := kube.Clients{KubernetesClient: client}
			config := reloadTestConfig()

			// The denied Deployment makes every attempt fail, so handleErr retries the event.
			for attempt := 0; attempt < 3; attempt++ {
				err := reloadWorkloads(clients, config, createTestCollectors(), nil, invokeReloadStrategy)
				require.Error(t, err)
			}
			assert.Equal(t, 1, countJobs(t, client), "retries of the same change must create the CronJob Job only once")

			changed := common.GetConfigmapConfig(&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "default"},
				Data:       map[string]string{"key": "newer-value"},
			})
			require.Error(t, reloadWorkloads(clients, changed, createTestCollectors(), nil, invokeReloadStrategy))
			assert.Equal(t, 2, countJobs(t, client), "a new change must still create a Job")
		})
	}
}

func TestReloadWorkloadsRetryDoesNotRecreateJobTwice(t *testing.T) {
	originalStrategy := options.ReloadStrategy
	t.Cleanup(func() { options.ReloadStrategy = originalStrategy })

	for _, strategy := range []string{constants.EnvVarsReloadStrategy, constants.AnnotationsReloadStrategy} {
		t.Run(strategy, func(t *testing.T) {
			options.ReloadStrategy = strategy
			client := newJobNamingClient(reloadTestDeployment(deniedWorkloadName), reloadTestJob("migrate"))
			clients := kube.Clients{KubernetesClient: client}
			config := reloadTestConfig()

			for attempt := 0; attempt < 3; attempt++ {
				err := reloadWorkloads(clients, config, createTestCollectors(), nil, invokeReloadStrategy)
				require.Error(t, err)
			}

			deletes := 0
			for _, action := range client.Actions() {
				if action.GetVerb() == "delete" && action.GetResource().Resource == "jobs" {
					deletes++
				}
			}
			assert.Equal(t, 1, deletes, "retries of the same change must recreate the Job only once")
			assert.Equal(t, 1, countJobs(t, client))
		})
	}
}

func TestJoinErrorsKeepsFailuresOnOneLine(t *testing.T) {
	first := errors.New("update for 'api' failed")
	second := errors.New("update for 'worker' failed")

	err := joinErrors([]error{joinErrors([]error{first, ErrNotRetryable}), nil, second})
	require.Error(t, err)
	assert.Equal(t, "update for 'api' failed; not retryable; update for 'worker' failed", err.Error())
	assert.ErrorIs(t, err, ErrNotRetryable)
	assert.ErrorIs(t, err, second)

	assert.NoError(t, joinErrors(nil))
	assert.Same(t, first, joinErrors([]error{first}))
}
