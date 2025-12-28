package controller_test

import (
	"context"
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateWorkloadWithRetry_WorkloadTypes(t *testing.T) {
	tests := []struct {
		name         string
		object       runtime.Object
		workload     func(runtime.Object) workload.WorkloadAccessor
		resourceType reload.ResourceType
		verify       func(t *testing.T, c client.Client)
	}{
		{
			name:   "Deployment",
			object: testDeployment("test-deployment", "default", nil),
			workload: func(o runtime.Object) workload.WorkloadAccessor {
				return workload.NewDeploymentWorkload(o.(*appsv1.Deployment))
			},
			resourceType: reload.ResourceTypeConfigMap,
			verify: func(t *testing.T, c client.Client) {
				var result appsv1.Deployment
				if err := c.Get(context.Background(), types.NamespacedName{Name: "test-deployment", Namespace: "default"}, &result); err != nil {
					t.Fatalf("Failed to get deployment: %v", err)
				}
				if result.Spec.Template.Annotations == nil {
					t.Fatal("Expected pod template annotations to be set")
				}
			},
		},
		{
			name:   "DaemonSet",
			object: testDaemonSet("test-daemonset", "default", nil),
			workload: func(o runtime.Object) workload.WorkloadAccessor {
				return workload.NewDaemonSetWorkload(o.(*appsv1.DaemonSet))
			},
			resourceType: reload.ResourceTypeSecret,
			verify: func(t *testing.T, c client.Client) {
				var result appsv1.DaemonSet
				if err := c.Get(context.Background(), types.NamespacedName{Name: "test-daemonset", Namespace: "default"}, &result); err != nil {
					t.Fatalf("Failed to get daemonset: %v", err)
				}
				if result.Spec.Template.Annotations == nil {
					t.Fatal("Expected pod template annotations to be set")
				}
			},
		},
		{
			name:   "StatefulSet",
			object: testStatefulSet("test-statefulset", "default", nil),
			workload: func(o runtime.Object) workload.WorkloadAccessor {
				return workload.NewStatefulSetWorkload(o.(*appsv1.StatefulSet))
			},
			resourceType: reload.ResourceTypeConfigMap,
			verify: func(t *testing.T, c client.Client) {
				var result appsv1.StatefulSet
				if err := c.Get(context.Background(), types.NamespacedName{Name: "test-statefulset", Namespace: "default"}, &result); err != nil {
					t.Fatalf("Failed to get statefulset: %v", err)
				}
				if result.Spec.Template.Annotations == nil {
					t.Fatal("Expected pod template annotations to be set")
				}
			},
		},
		{
			name:   "Job",
			object: testJob("test-job", "default"),
			workload: func(o runtime.Object) workload.WorkloadAccessor {
				return workload.NewJobWorkload(o.(*batchv1.Job))
			},
			resourceType: reload.ResourceTypeConfigMap,
			verify: func(t *testing.T, c client.Client) {
				var jobs batchv1.JobList
				if err := c.List(context.Background(), &jobs, client.InNamespace("default")); err != nil {
					t.Fatalf("Failed to list jobs: %v", err)
				}
				if len(jobs.Items) != 1 {
					t.Errorf("Expected 1 job (recreated), got %d", len(jobs.Items))
				}
			},
		},
		{
			name:   "CronJob",
			object: testCronJob("test-cronjob", "default"),
			workload: func(o runtime.Object) workload.WorkloadAccessor {
				return workload.NewCronJobWorkload(o.(*batchv1.CronJob))
			},
			resourceType: reload.ResourceTypeSecret,
			verify: func(t *testing.T, c client.Client) {
				var jobs batchv1.JobList
				if err := c.List(context.Background(), &jobs, client.InNamespace("default")); err != nil {
					t.Fatalf("Failed to list jobs: %v", err)
				}
				if len(jobs.Items) != 1 {
					t.Errorf("Expected 1 job from cronjob, got %d", len(jobs.Items))
				}
				if len(jobs.Items) > 0 && jobs.Items[0].Annotations["cronjob.kubernetes.io/instantiate"] != "manual" {
					t.Error("Expected job to have manual instantiate annotation")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := config.NewDefault()
				reloadService := reload.NewService(cfg)

				fakeClient := fake.NewClientBuilder().
					WithScheme(testScheme()).
					WithRuntimeObjects(tt.object).
					Build()

				wl := tt.workload(tt.object)

				updated, err := controller.UpdateWorkloadWithRetry(
					context.Background(),
					fakeClient,
					reloadService,
					wl,
					"test-resource",
					tt.resourceType,
					"default",
					"abc123",
					false,
				)

				if err != nil {
					t.Fatalf("UpdateWorkloadWithRetry failed: %v", err)
				}
				if !updated {
					t.Error("Expected workload to be updated")
				}

				tt.verify(t, fakeClient)
			},
		)
	}
}

func TestUpdateWorkloadWithRetry_Strategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy config.ReloadStrategy
		verify   func(t *testing.T, cfg *config.Config, result *appsv1.Deployment)
	}{
		{
			name:     "EnvVarStrategy",
			strategy: config.ReloadStrategyEnvVars,
			verify: func(t *testing.T, cfg *config.Config, result *appsv1.Deployment) {
				found := false
				for _, env := range result.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "STAKATER_TEST_CM_CONFIGMAP" && env.Value == "abc123" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected STAKATER_TEST_CM_CONFIGMAP env var to be set")
				}
			},
		},
		{
			name:     "AnnotationStrategy",
			strategy: config.ReloadStrategyAnnotations,
			verify: func(t *testing.T, cfg *config.Config, result *appsv1.Deployment) {
				if result.Spec.Template.Annotations == nil {
					t.Fatal("Expected pod template annotations to be set")
				}
				if _, ok := result.Spec.Template.Annotations[cfg.Annotations.LastReloadedFrom]; !ok {
					t.Errorf("Expected %s annotation to be set", cfg.Annotations.LastReloadedFrom)
				}
				for _, env := range result.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "STAKATER_TEST_CM_CONFIGMAP" {
						t.Error("Annotation strategy should not add env vars")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := config.NewDefault()
				cfg.ReloadStrategy = tt.strategy
				reloadService := reload.NewService(cfg)

				deployment := testDeployment("test-deployment", "default", nil)
				fakeClient := fake.NewClientBuilder().
					WithScheme(testScheme()).
					WithObjects(deployment).
					Build()

				wl := workload.NewDeploymentWorkload(deployment)

				updated, err := controller.UpdateWorkloadWithRetry(
					context.Background(),
					fakeClient,
					reloadService,
					wl,
					"test-cm",
					reload.ResourceTypeConfigMap,
					"default",
					"abc123",
					false,
				)

				if err != nil {
					t.Fatalf("UpdateWorkloadWithRetry failed: %v", err)
				}
				if !updated {
					t.Error("Expected workload to be updated")
				}

				var result appsv1.Deployment
				if err := fakeClient.Get(
					context.Background(), types.NamespacedName{Name: "test-deployment", Namespace: "default"}, &result,
				); err != nil {
					t.Fatalf("Failed to get deployment: %v", err)
				}

				tt.verify(t, cfg, &result)
			},
		)
	}
}

func TestUpdateWorkloadWithRetry_NoUpdate(t *testing.T) {
	cfg := config.NewDefault()
	reloadService := reload.NewService(cfg)

	deployment := testDeployment("test-deployment", "default", nil)
	deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "STAKATER_TEST_CM_CONFIGMAP",
			Value: "abc123",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(deployment).
		Build()

	wl := workload.NewDeploymentWorkload(deployment)

	updated, err := controller.UpdateWorkloadWithRetry(
		context.Background(),
		fakeClient,
		reloadService,
		wl,
		"test-cm",
		reload.ResourceTypeConfigMap,
		"default",
		"abc123", // Same hash as already set
		false,
	)

	if err != nil {
		t.Fatalf("UpdateWorkloadWithRetry failed: %v", err)
	}
	if updated {
		t.Error("Expected workload NOT to be updated (same hash)")
	}
}

func TestResourceTypeKind(t *testing.T) {
	tests := []struct {
		resourceType reload.ResourceType
		expectedKind string
	}{
		{reload.ResourceTypeConfigMap, "ConfigMap"},
		{reload.ResourceTypeSecret, "Secret"},
	}

	for _, tt := range tests {
		t.Run(
			string(tt.resourceType), func(t *testing.T) {
				if got := tt.resourceType.Kind(); got != tt.expectedKind {
					t.Errorf("ResourceType.Kind() = %v, want %v", got, tt.expectedKind)
				}
			},
		)
	}
}
