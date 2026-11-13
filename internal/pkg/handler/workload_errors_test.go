package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

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
