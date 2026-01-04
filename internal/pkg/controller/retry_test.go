package controller_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/testutil"
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
		workload     func(runtime.Object) workload.Workload
		resourceType reload.ResourceType
		verify       func(t *testing.T, c client.Client)
	}{
		{
			name:   "Deployment",
			object: testutil.NewDeployment("test-deployment", "default", nil),
			workload: func(o runtime.Object) workload.Workload {
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
			object: testutil.NewDaemonSet("test-daemonset", "default", nil),
			workload: func(o runtime.Object) workload.Workload {
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
			object: testutil.NewStatefulSet("test-statefulset", "default", nil),
			workload: func(o runtime.Object) workload.Workload {
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
			object: testutil.NewJob("test-job", "default"),
			workload: func(o runtime.Object) workload.Workload {
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
			object: testutil.NewCronJob("test-cronjob", "default"),
			workload: func(o runtime.Object) workload.Workload {
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
				reloadService := reload.NewService(cfg, testr.New(t))

				fakeClient := fake.NewClientBuilder().
					WithScheme(testutil.NewScheme()).
					WithRuntimeObjects(tt.object).
					Build()

				wl := tt.workload(tt.object)

				updated, err := controller.UpdateWorkloadWithRetry(
					context.Background(),
					fakeClient,
					reloadService,
					nil, // no pause handler
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
				reloadService := reload.NewService(cfg, testr.New(t))

				deployment := testutil.NewDeployment("test-deployment", "default", nil)
				fakeClient := fake.NewClientBuilder().
					WithScheme(testutil.NewScheme()).
					WithObjects(deployment).
					Build()

				wl := workload.NewDeploymentWorkload(deployment)

				updated, err := controller.UpdateWorkloadWithRetry(
					context.Background(),
					fakeClient,
					reloadService,
					nil, // no pause handler for this test
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
	reloadService := reload.NewService(cfg, testr.New(t))

	deployment := testutil.NewDeployment("test-deployment", "default", nil)
	deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "STAKATER_TEST_CM_CONFIGMAP",
			Value: "abc123",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithObjects(deployment).
		Build()

	wl := workload.NewDeploymentWorkload(deployment)

	updated, err := controller.UpdateWorkloadWithRetry(
		context.Background(),
		fakeClient,
		reloadService,
		nil, // no pause handler
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

func TestUpdateWorkloadWithRetry_PauseDeployment(t *testing.T) {
	cfg := config.NewDefault()
	reloadService := reload.NewService(cfg, testr.New(t))
	pauseHandler := reload.NewPauseHandler(cfg)

	deployment := testutil.NewDeployment(
		"test-deployment", "default", map[string]string{
			"reloader.stakater.com/auto":                    "true",
			"deployment.reloader.stakater.com/pause-period": "5m",
		},
	)

	fakeClient := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithObjects(deployment).
		Build()

	wl := workload.NewDeploymentWorkload(deployment)

	updated, err := controller.UpdateWorkloadWithRetry(
		context.Background(),
		fakeClient,
		reloadService,
		pauseHandler,
		wl,
		"test-cm",
		reload.ResourceTypeConfigMap,
		"default",
		"abc123",
		true,
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

	if result.Spec.Template.Annotations == nil {
		t.Fatal("Expected pod template annotations to be set")
	}

	if !result.Spec.Paused {
		t.Error("Expected deployment to be paused (spec.Paused=true)")
	}

	pausedAt := result.Annotations[cfg.Annotations.PausedAt]
	if pausedAt == "" {
		t.Error("Expected paused-at annotation to be set")
	}
}

// TestUpdateWorkloadWithRetry_PauseWithExplicitAnnotation tests pause with explicit configmap annotation (no auto).
func TestUpdateWorkloadWithRetry_PauseWithExplicitAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	reloadService := reload.NewService(cfg, testr.New(t))
	pauseHandler := reload.NewPauseHandler(cfg)

	deployment := testutil.NewDeployment(
		"test-deployment", "default", map[string]string{
			cfg.Annotations.ConfigmapReload: "test-cm", // explicit, not auto
			cfg.Annotations.PausePeriod:     "5m",
		},
	)

	fakeClient := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithObjects(deployment).
		Build()

	wl := workload.NewDeploymentWorkload(deployment)

	updated, err := controller.UpdateWorkloadWithRetry(
		context.Background(),
		fakeClient,
		reloadService,
		pauseHandler,
		wl,
		"test-cm",
		reload.ResourceTypeConfigMap,
		"default",
		"abc123",
		false, // NOT auto reload
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

	if result.Spec.Template.Annotations == nil {
		t.Fatal("Expected pod template annotations to be set")
	}

	if !result.Spec.Paused {
		t.Error("Expected deployment to be paused (spec.Paused=true)")
	}

	pausedAt := result.Annotations[cfg.Annotations.PausedAt]
	if pausedAt == "" {
		t.Error("Expected paused-at annotation to be set")
	}
}

// TestUpdateWorkloadWithRetry_PauseWithSecretReload tests pause with Secret-triggered reload.
func TestUpdateWorkloadWithRetry_PauseWithSecretReload(t *testing.T) {
	cfg := config.NewDefault()
	reloadService := reload.NewService(cfg, testr.New(t))
	pauseHandler := reload.NewPauseHandler(cfg)

	deployment := testutil.NewDeployment(
		"test-deployment", "default", map[string]string{
			cfg.Annotations.SecretReload: "test-secret", // explicit secret, not auto
			cfg.Annotations.PausePeriod:  "5m",
		},
	)

	fakeClient := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithObjects(deployment).
		Build()

	wl := workload.NewDeploymentWorkload(deployment)

	updated, err := controller.UpdateWorkloadWithRetry(
		context.Background(),
		fakeClient,
		reloadService,
		pauseHandler,
		wl,
		"test-secret",
		reload.ResourceTypeSecret,
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

	if !result.Spec.Paused {
		t.Error("Expected deployment to be paused (spec.Paused=true)")
	}

	pausedAt := result.Annotations[cfg.Annotations.PausedAt]
	if pausedAt == "" {
		t.Error("Expected paused-at annotation to be set")
	}
}

// TestUpdateWorkloadWithRetry_PauseWithAutoSecret tests pause with auto annotation + Secret change.
func TestUpdateWorkloadWithRetry_PauseWithAutoSecret(t *testing.T) {
	cfg := config.NewDefault()
	reloadService := reload.NewService(cfg, testr.New(t))
	pauseHandler := reload.NewPauseHandler(cfg)

	deployment := testutil.NewDeployment(
		"test-deployment", "default", map[string]string{
			cfg.Annotations.Auto:        "true",
			cfg.Annotations.PausePeriod: "5m",
		},
	)

	fakeClient := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithObjects(deployment).
		Build()

	wl := workload.NewDeploymentWorkload(deployment)

	updated, err := controller.UpdateWorkloadWithRetry(
		context.Background(),
		fakeClient,
		reloadService,
		pauseHandler,
		wl,
		"test-secret",
		reload.ResourceTypeSecret,
		"default",
		"abc123",
		true,
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

	if !result.Spec.Paused {
		t.Error("Expected deployment to be paused (spec.Paused=true)")
	}
}

func TestUpdateWorkloadWithRetry_NoPauseWithoutAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	reloadService := reload.NewService(cfg, testr.New(t))
	pauseHandler := reload.NewPauseHandler(cfg)

	deployment := testutil.NewDeployment(
		"test-deployment", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)

	fakeClient := fake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		WithObjects(deployment).
		Build()

	wl := workload.NewDeploymentWorkload(deployment)

	updated, err := controller.UpdateWorkloadWithRetry(
		context.Background(),
		fakeClient,
		reloadService,
		pauseHandler,
		wl,
		"test-cm",
		reload.ResourceTypeConfigMap,
		"default",
		"abc123",
		true,
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

	if result.Spec.Paused {
		t.Error("Expected deployment NOT to be paused (no pause-period annotation)")
	}
}
