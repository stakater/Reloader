package reload

import (
	"context"
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/workload"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestService_ProcessConfigMap_AutoReload(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Create a deployment with auto annotation that uses the configmap
	deploy := createTestDeployment("test-deploy", "default", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-cm",
					},
				},
			},
		},
	}

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)

	if len(decisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(decisions))
	}

	if !decisions[0].ShouldReload {
		t.Error("Expected ShouldReload to be true")
	}

	if !decisions[0].AutoReload {
		t.Error("Expected AutoReload to be true")
	}

	if decisions[0].Hash == "" {
		t.Error("Expected Hash to be non-empty")
	}
}

func TestService_ProcessConfigMap_ExplicitAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Create a deployment with explicit configmap annotation
	deploy := createTestDeployment("test-deploy", "default", map[string]string{
		"configmap.reloader.stakater.com/reload": "test-cm",
	})

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)

	if len(decisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(decisions))
	}

	if !decisions[0].ShouldReload {
		t.Error("Expected ShouldReload to be true for explicit annotation")
	}

	if decisions[0].AutoReload {
		t.Error("Expected AutoReload to be false for explicit annotation")
	}
}

func TestService_ProcessConfigMap_IgnoredResource(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Create a deployment with auto annotation
	deploy := createTestDeployment("test-deploy", "default", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-cm",
					},
				},
			},
		},
	}

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy),
	}

	// ConfigMap with ignore annotation
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Annotations: map[string]string{
				"reloader.stakater.com/ignore": "true",
			},
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)

	// Should still get a decision, but ShouldReload should be false
	for _, d := range decisions {
		if d.ShouldReload {
			t.Error("Expected ShouldReload to be false for ignored resource")
		}
	}
}

func TestService_ProcessSecret_AutoReload(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Create a deployment with auto annotation that uses the secret
	deploy := createTestDeployment("test-deploy", "default", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "secret-vol",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "test-secret",
				},
			},
		},
	}

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	change := SecretChange{
		Secret:    secret,
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)

	if len(decisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(decisions))
	}

	if !decisions[0].ShouldReload {
		t.Error("Expected ShouldReload to be true")
	}

	if !decisions[0].AutoReload {
		t.Error("Expected AutoReload to be true")
	}
}

func TestService_ProcessConfigMap_DeleteEvent(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnDelete = true
	svc := NewService(cfg)

	// Create a deployment with explicit configmap annotation
	deploy := createTestDeployment("test-deploy", "default", map[string]string{
		"configmap.reloader.stakater.com/reload": "test-cm",
	})

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeDelete,
	}

	decisions := svc.Process(change, workloads)

	if len(decisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(decisions))
	}

	if !decisions[0].ShouldReload {
		t.Error("Expected ShouldReload to be true for delete event")
	}

	// Hash should be empty for delete events
	if decisions[0].Hash != "" {
		t.Errorf("Expected empty hash for delete event, got %s", decisions[0].Hash)
	}
}

func TestService_ProcessConfigMap_DeleteEventDisabled(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnDelete = false // Disabled by default
	svc := NewService(cfg)

	deploy := createTestDeployment("test-deploy", "default", map[string]string{
		"configmap.reloader.stakater.com/reload": "test-cm",
	})

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeDelete,
	}

	decisions := svc.Process(change, workloads)

	// Should return nil when delete events are disabled
	if decisions != nil {
		t.Error("Expected nil decisions when delete events are disabled")
	}
}

func TestService_ApplyReload_EnvVarStrategy(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadStrategy = config.ReloadStrategyEnvVars
	svc := NewService(cfg)

	deploy := createTestDeployment("test-deploy", "default", nil)
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", ResourceTypeConfigMap, "default", "abc123hash", false)

	if err != nil {
		t.Fatalf("ApplyReload failed: %v", err)
	}

	if !updated {
		t.Error("Expected updated to be true")
	}

	// Verify env var was added
	containers := accessor.GetContainers()
	if len(containers) == 0 {
		t.Fatal("No containers found")
	}

	found := false
	for _, env := range containers[0].Env {
		if env.Name == "STAKATER_TEST_CM_CONFIGMAP" && env.Value == "abc123hash" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected env var STAKATER_TEST_CM_CONFIGMAP to be set")
	}

	// Verify attribution annotation was set
	annotations := accessor.GetPodTemplateAnnotations()
	if annotations["reloader.stakater.com/last-reloaded-from"] == "" {
		t.Error("Expected last-reloaded-from annotation to be set")
	}
}

func TestService_ApplyReload_AnnotationStrategy(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadStrategy = config.ReloadStrategyAnnotations
	svc := NewService(cfg)

	deploy := createTestDeployment("test-deploy", "default", nil)
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", ResourceTypeConfigMap, "default", "abc123hash", false)

	if err != nil {
		t.Fatalf("ApplyReload failed: %v", err)
	}

	if !updated {
		t.Error("Expected updated to be true")
	}

	// Verify annotation was added
	annotations := accessor.GetPodTemplateAnnotations()
	if annotations["reloader.stakater.com/last-reloaded-from"] == "" {
		t.Error("Expected last-reloaded-from annotation to be set")
	}
}

func TestService_ApplyReload_EnvVarDeletion(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadStrategy = config.ReloadStrategyEnvVars
	svc := NewService(cfg)

	deploy := createTestDeployment("test-deploy", "default", nil)
	// Pre-add an env var
	deploy.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "STAKATER_TEST_CM_CONFIGMAP", Value: "oldhash"},
		{Name: "OTHER_VAR", Value: "keep"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	// Empty hash signals deletion
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", ResourceTypeConfigMap, "default", "", false)

	if err != nil {
		t.Fatalf("ApplyReload failed: %v", err)
	}

	if !updated {
		t.Error("Expected updated to be true for env var removal")
	}

	// Verify env var was removed
	containers := accessor.GetContainers()
	for _, env := range containers[0].Env {
		if env.Name == "STAKATER_TEST_CM_CONFIGMAP" {
			t.Error("Expected env var STAKATER_TEST_CM_CONFIGMAP to be removed")
		}
	}

	// Verify other env var was kept
	found := false
	for _, env := range containers[0].Env {
		if env.Name == "OTHER_VAR" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected OTHER_VAR to be kept")
	}
}

func TestService_ApplyReload_NoChangeIfSameHash(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadStrategy = config.ReloadStrategyEnvVars
	svc := NewService(cfg)

	deploy := createTestDeployment("test-deploy", "default", nil)
	// Pre-add env var with same hash
	deploy.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "STAKATER_TEST_CM_CONFIGMAP", Value: "abc123hash"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", ResourceTypeConfigMap, "default", "abc123hash", false)

	if err != nil {
		t.Fatalf("ApplyReload failed: %v", err)
	}

	if updated {
		t.Error("Expected updated to be false when hash is unchanged")
	}
}

func TestService_ProcessConfigMap_MultipleWorkloads(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Create multiple workloads
	deploy1 := createTestDeployment("deploy1", "default", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	deploy1.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "shared-cm",
					},
				},
			},
		},
	}

	deploy2 := createTestDeployment("deploy2", "default", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	deploy2.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "shared-cm",
					},
				},
			},
		},
	}

	// Deploy3 doesn't use the configmap
	deploy3 := createTestDeployment("deploy3", "default", map[string]string{
		"reloader.stakater.com/auto": "true",
	})

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy1),
		workload.NewDeploymentWorkload(deploy2),
		workload.NewDeploymentWorkload(deploy3),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)

	if len(decisions) != 3 {
		t.Fatalf("Expected 3 decisions, got %d", len(decisions))
	}

	// Count how many should reload
	reloadCount := 0
	for _, d := range decisions {
		if d.ShouldReload {
			reloadCount++
		}
	}

	// Only deploy1 and deploy2 should reload (they use the configmap)
	if reloadCount != 2 {
		t.Errorf("Expected 2 workloads to reload, got %d", reloadCount)
	}
}

func TestService_ProcessConfigMap_DifferentNamespaces(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Create deployments in different namespaces
	deploy1 := createTestDeployment("deploy1", "namespace-a", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	deploy1.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-cm",
					},
				},
			},
		},
	}

	deploy2 := createTestDeployment("deploy2", "namespace-b", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	deploy2.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-cm",
					},
				},
			},
		},
	}

	workloads := []workload.WorkloadAccessor{
		workload.NewDeploymentWorkload(deploy1),
		workload.NewDeploymentWorkload(deploy2),
	}

	// ConfigMap in namespace-a
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "namespace-a",
		},
		Data: map[string]string{"key": "value"},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)

	// Should only affect deploy1 (same namespace)
	reloadCount := 0
	for _, d := range decisions {
		if d.ShouldReload {
			reloadCount++
		}
	}

	if reloadCount != 1 {
		t.Errorf("Expected 1 workload to reload (same namespace), got %d", reloadCount)
	}
}

// Helper function to create a test deployment
func createTestDeployment(name, namespace string, annotations map[string]string) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": name},
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}
