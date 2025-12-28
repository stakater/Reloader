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
	deploy := createTestDeployment(
		"test-deploy", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
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

	deploy := createTestDeployment(
		"test-deploy", "default", map[string]string{
			"configmap.reloader.stakater.com/reload": "test-cm",
		},
	)

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
	deploy := createTestDeployment(
		"test-deploy", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
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
	deploy := createTestDeployment(
		"test-deploy", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
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
	deploy := createTestDeployment(
		"test-deploy", "default", map[string]string{
			"configmap.reloader.stakater.com/reload": "test-cm",
		},
	)

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

	deploy := createTestDeployment(
		"test-deploy", "default", map[string]string{
			"configmap.reloader.stakater.com/reload": "test-cm",
		},
	)

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
	deploy1 := createTestDeployment(
		"deploy1", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
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

	deploy2 := createTestDeployment(
		"deploy2", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
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
	deploy3 := createTestDeployment(
		"deploy3", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)

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
	deploy1 := createTestDeployment(
		"deploy1", "namespace-a", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
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

	deploy2 := createTestDeployment(
		"deploy2", "namespace-b", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
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

func TestService_Hasher(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	hasher := svc.Hasher()
	if hasher == nil {
		t.Fatal("Expected Hasher to return non-nil hasher")
	}

	// Verify it's functional
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Data:       map[string]string{"key": "value"},
	}
	hash := hasher.HashConfigMap(cm)
	if hash == "" {
		t.Error("Expected hasher to produce non-empty hash")
	}
}

func TestService_shouldProcessEvent(t *testing.T) {
	tests := []struct {
		name           string
		reloadOnCreate bool
		reloadOnDelete bool
		eventType      EventType
		expected       bool
	}{
		{"create enabled", true, false, EventTypeCreate, true},
		{"create disabled", false, false, EventTypeCreate, false},
		{"delete enabled", false, true, EventTypeDelete, true},
		{"delete disabled", false, false, EventTypeDelete, false},
		{"update always true", false, false, EventTypeUpdate, true},
		{"unknown event", false, false, EventType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := config.NewDefault()
				cfg.ReloadOnCreate = tt.reloadOnCreate
				cfg.ReloadOnDelete = tt.reloadOnDelete
				svc := NewService(cfg)

				result := svc.shouldProcessEvent(tt.eventType)
				if result != tt.expected {
					t.Errorf("shouldProcessEvent(%s) = %v, want %v", tt.eventType, result, tt.expected)
				}
			},
		)
	}
}

func TestService_findVolumeUsingResource_ConfigMap(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	tests := []struct {
		name         string
		volumes      []corev1.Volume
		resourceName string
		resourceType ResourceType
		wantVolume   string
	}{
		{
			name: "direct configmap volume",
			volumes: []corev1.Volume{
				{
					Name: "config-vol",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
						},
					},
				},
			},
			resourceName: "my-cm",
			resourceType: ResourceTypeConfigMap,
			wantVolume:   "config-vol",
		},
		{
			name: "projected configmap volume",
			volumes: []corev1.Volume{
				{
					Name: "projected-vol",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									ConfigMap: &corev1.ConfigMapProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "projected-cm"},
									},
								},
							},
						},
					},
				},
			},
			resourceName: "projected-cm",
			resourceType: ResourceTypeConfigMap,
			wantVolume:   "projected-vol",
		},
		{
			name: "no match",
			volumes: []corev1.Volume{
				{
					Name: "other-vol",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "other-cm"},
						},
					},
				},
			},
			resourceName: "my-cm",
			resourceType: ResourceTypeConfigMap,
			wantVolume:   "",
		},
		{
			name:         "empty volumes",
			volumes:      []corev1.Volume{},
			resourceName: "my-cm",
			resourceType: ResourceTypeConfigMap,
			wantVolume:   "",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := svc.findVolumeUsingResource(tt.volumes, tt.resourceName, tt.resourceType)
				if got != tt.wantVolume {
					t.Errorf("findVolumeUsingResource() = %q, want %q", got, tt.wantVolume)
				}
			},
		)
	}
}

func TestService_findVolumeUsingResource_Secret(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	tests := []struct {
		name         string
		volumes      []corev1.Volume
		resourceName string
		wantVolume   string
	}{
		{
			name: "direct secret volume",
			volumes: []corev1.Volume{
				{
					Name: "secret-vol",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "my-secret",
						},
					},
				},
			},
			resourceName: "my-secret",
			wantVolume:   "secret-vol",
		},
		{
			name: "projected secret volume",
			volumes: []corev1.Volume{
				{
					Name: "projected-vol",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "projected-secret"},
									},
								},
							},
						},
					},
				},
			},
			resourceName: "projected-secret",
			wantVolume:   "projected-vol",
		},
		{
			name: "no match",
			volumes: []corev1.Volume{
				{
					Name: "other-vol",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "other-secret",
						},
					},
				},
			},
			resourceName: "my-secret",
			wantVolume:   "",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := svc.findVolumeUsingResource(tt.volumes, tt.resourceName, ResourceTypeSecret)
				if got != tt.wantVolume {
					t.Errorf("findVolumeUsingResource() = %q, want %q", got, tt.wantVolume)
				}
			},
		)
	}
}

func TestService_findContainerWithVolumeMount(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	tests := []struct {
		name        string
		containers  []corev1.Container
		volumeName  string
		wantName    string
		shouldMatch bool
	}{
		{
			name: "container with matching volume mount",
			containers: []corev1.Container{
				{
					Name: "container1",
					VolumeMounts: []corev1.VolumeMount{
						{Name: "config-vol", MountPath: "/config"},
					},
				},
			},
			volumeName:  "config-vol",
			wantName:    "container1",
			shouldMatch: true,
		},
		{
			name: "second container with matching mount",
			containers: []corev1.Container{
				{
					Name:         "container1",
					VolumeMounts: []corev1.VolumeMount{},
				},
				{
					Name: "container2",
					VolumeMounts: []corev1.VolumeMount{
						{Name: "config-vol", MountPath: "/config"},
					},
				},
			},
			volumeName:  "config-vol",
			wantName:    "container2",
			shouldMatch: true,
		},
		{
			name: "no matching mount",
			containers: []corev1.Container{
				{
					Name: "container1",
					VolumeMounts: []corev1.VolumeMount{
						{Name: "other-vol", MountPath: "/other"},
					},
				},
			},
			volumeName:  "config-vol",
			shouldMatch: false,
		},
		{
			name:        "empty containers",
			containers:  []corev1.Container{},
			volumeName:  "config-vol",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := svc.findContainerWithVolumeMount(tt.containers, tt.volumeName)
				if tt.shouldMatch {
					if got == nil {
						t.Error("Expected to find a container, got nil")
					} else if got.Name != tt.wantName {
						t.Errorf("findContainerWithVolumeMount() container name = %q, want %q", got.Name, tt.wantName)
					}
				} else {
					if got != nil {
						t.Errorf("Expected nil, got container %q", got.Name)
					}
				}
			},
		)
	}
}

func TestService_findContainerWithEnvRef_ConfigMap(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	tests := []struct {
		name         string
		containers   []corev1.Container
		resourceName string
		wantName     string
		shouldMatch  bool
	}{
		{
			name: "container with ConfigMapKeyRef",
			containers: []corev1.Container{
				{
					Name: "app",
					Env: []corev1.EnvVar{
						{
							Name: "CONFIG_VALUE",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
									Key:                  "key",
								},
							},
						},
					},
				},
			},
			resourceName: "my-cm",
			wantName:     "app",
			shouldMatch:  true,
		},
		{
			name: "container with ConfigMapRef in EnvFrom",
			containers: []corev1.Container{
				{
					Name: "app",
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
							},
						},
					},
				},
			},
			resourceName: "my-cm",
			wantName:     "app",
			shouldMatch:  true,
		},
		{
			name: "no matching env ref",
			containers: []corev1.Container{
				{
					Name: "app",
					Env: []corev1.EnvVar{
						{
							Name:  "SIMPLE_VAR",
							Value: "value",
						},
					},
				},
			},
			resourceName: "my-cm",
			shouldMatch:  false,
		},
		{
			name: "env without ValueFrom",
			containers: []corev1.Container{
				{
					Name: "app",
					Env: []corev1.EnvVar{
						{Name: "VAR1", Value: "val"},
					},
				},
			},
			resourceName: "my-cm",
			shouldMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := svc.findContainerWithEnvRef(tt.containers, tt.resourceName, ResourceTypeConfigMap)
				if tt.shouldMatch {
					if got == nil {
						t.Error("Expected to find a container, got nil")
					} else if got.Name != tt.wantName {
						t.Errorf("findContainerWithEnvRef() container name = %q, want %q", got.Name, tt.wantName)
					}
				} else {
					if got != nil {
						t.Errorf("Expected nil, got container %q", got.Name)
					}
				}
			},
		)
	}
}

func TestService_findContainerWithEnvRef_Secret(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	tests := []struct {
		name         string
		containers   []corev1.Container
		resourceName string
		wantName     string
		shouldMatch  bool
	}{
		{
			name: "container with SecretKeyRef",
			containers: []corev1.Container{
				{
					Name: "app",
					Env: []corev1.EnvVar{
						{
							Name: "SECRET_VALUE",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
									Key:                  "password",
								},
							},
						},
					},
				},
			},
			resourceName: "my-secret",
			wantName:     "app",
			shouldMatch:  true,
		},
		{
			name: "container with SecretRef in EnvFrom",
			containers: []corev1.Container{
				{
					Name: "app",
					EnvFrom: []corev1.EnvFromSource{
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
							},
						},
					},
				},
			},
			resourceName: "my-secret",
			wantName:     "app",
			shouldMatch:  true,
		},
		{
			name: "no matching env ref",
			containers: []corev1.Container{
				{
					Name: "app",
					Env: []corev1.EnvVar{
						{
							Name:  "SIMPLE_VAR",
							Value: "value",
						},
					},
				},
			},
			resourceName: "my-secret",
			shouldMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := svc.findContainerWithEnvRef(tt.containers, tt.resourceName, ResourceTypeSecret)
				if tt.shouldMatch {
					if got == nil {
						t.Error("Expected to find a container, got nil")
					} else if got.Name != tt.wantName {
						t.Errorf("findContainerWithEnvRef() container name = %q, want %q", got.Name, tt.wantName)
					}
				} else {
					if got != nil {
						t.Errorf("Expected nil, got container %q", got.Name)
					}
				}
			},
		)
	}
}

func TestService_findTargetContainer_AutoReload(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Test with autoReload=true and volume mount
	deploy := createTestDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
				},
			},
		},
	}
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:  "app",
			Image: "nginx",
			VolumeMounts: []corev1.VolumeMount{
				{Name: "config-vol", MountPath: "/config"},
			},
		},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "my-cm", ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "app" {
		t.Errorf("Expected container 'app', got %q", container.Name)
	}
}

func TestService_findTargetContainer_AutoReload_EnvRef(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Test with autoReload=true and env ref (no volume)
	deploy := createTestDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:  "sidecar",
			Image: "busybox",
		},
		{
			Name:  "app",
			Image: "nginx",
			Env: []corev1.EnvVar{
				{
					Name: "CONFIG_VAL",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
							Key:                  "key",
						},
					},
				},
			},
		},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "my-cm", ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "app" {
		t.Errorf("Expected container 'app', got %q", container.Name)
	}
}

func TestService_findTargetContainer_AutoReload_InitContainer(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Test with autoReload=true where init container uses the volume
	deploy := createTestDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-vol",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
				},
			},
		},
	}
	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:  "init",
			Image: "busybox",
			VolumeMounts: []corev1.VolumeMount{
				{Name: "config-vol", MountPath: "/config"},
			},
		},
	}
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:  "app",
			Image: "nginx",
		},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "my-cm", ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	// Should return first main container when init container uses the volume
	if container.Name != "app" {
		t.Errorf("Expected container 'app', got %q", container.Name)
	}
}

func TestService_findTargetContainer_AutoReload_InitContainerEnvRef(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// Test with autoReload=true where init container has env ref
	deploy := createTestDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:  "init",
			Image: "busybox",
			Env: []corev1.EnvVar{
				{
					Name: "CONFIG_VAL",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-cm"},
							Key:                  "key",
						},
					},
				},
			},
		},
	}
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:  "app",
			Image: "nginx",
		},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "my-cm", ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	// Should return first main container when init container has the env ref
	if container.Name != "app" {
		t.Errorf("Expected container 'app', got %q", container.Name)
	}
}

func TestService_findTargetContainer_NoContainers(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	deploy := createTestDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "my-cm", ResourceTypeConfigMap, false)
	if container != nil {
		t.Error("Expected nil container for empty container list")
	}
}

func TestService_findTargetContainer_NonAutoReload(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	deploy := createTestDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{Name: "first", Image: "nginx"},
		{Name: "second", Image: "busybox"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	// Without autoReload, should return first container
	container := svc.findTargetContainer(accessor, "my-cm", ResourceTypeConfigMap, false)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "first" {
		t.Errorf("Expected first container, got %q", container.Name)
	}
}

func TestService_findTargetContainer_AutoReload_FallbackToFirst(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	// autoReload=true but no matching volume or env ref - should fallback to first container
	deploy := createTestDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{Name: "first", Image: "nginx"},
		{Name: "second", Image: "busybox"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "non-existent", ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "first" {
		t.Errorf("Expected first container as fallback, got %q", container.Name)
	}
}

func TestService_ProcessNilChange(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg)

	deploy := createTestDeployment("test", "default", nil)
	workloads := []workload.WorkloadAccessor{workload.NewDeploymentWorkload(deploy)}

	// Test with nil ConfigMap
	change := ConfigMapChange{
		ConfigMap: nil,
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)
	if decisions != nil {
		t.Errorf("Expected nil decisions for nil change, got %v", decisions)
	}
}

func TestService_ProcessCreateEventDisabled(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnCreate = false
	svc := NewService(cfg)

	deploy := createTestDeployment(
		"test", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
	workloads := []workload.WorkloadAccessor{workload.NewDeploymentWorkload(deploy)}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "default"},
		Data:       map[string]string{"key": "value"},
	}

	change := ConfigMapChange{
		ConfigMap: cm,
		EventType: EventTypeCreate,
	}

	decisions := svc.Process(change, workloads)
	if decisions != nil {
		t.Errorf("Expected nil decisions when create events disabled, got %v", decisions)
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
