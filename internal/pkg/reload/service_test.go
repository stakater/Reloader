package reload

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
	"github.com/stakater/Reloader/pkg/matcher"
)

func TestService_ProcessConfigMap_AutoReload(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	// Create a deployment with auto annotation that uses the configmap
	deploy := testutil.NewDeployment(
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

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment(
		"test-deploy", "default", map[string]string{
			"configmap.reloader.stakater.com/reload": "test-cm",
		},
	)

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

	// Create a deployment with auto annotation
	deploy := testutil.NewDeployment(
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

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

	// Create a deployment with auto annotation that uses the secret
	deploy := testutil.NewDeployment(
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

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

	// Create a deployment with explicit configmap annotation
	deploy := testutil.NewDeployment(
		"test-deploy", "default", map[string]string{
			"configmap.reloader.stakater.com/reload": "test-cm",
		},
	)

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment(
		"test-deploy", "default", map[string]string{
			"configmap.reloader.stakater.com/reload": "test-cm",
		},
	)

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test-deploy", "default", nil)
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", matcher.ResourceTypeConfigMap, "default", "abc123hash", false)

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
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test-deploy", "default", nil)
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", matcher.ResourceTypeConfigMap, "default", "abc123hash", false)

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
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test-deploy", "default", nil)
	// Pre-add an env var
	deploy.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "STAKATER_TEST_CM_CONFIGMAP", Value: "oldhash"},
		{Name: "OTHER_VAR", Value: "keep"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	// Empty hash signals deletion
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", matcher.ResourceTypeConfigMap, "default", "", false)

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
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test-deploy", "default", nil)
	// Pre-add env var with same hash
	deploy.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "STAKATER_TEST_CM_CONFIGMAP", Value: "abc123hash"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	ctx := context.Background()
	updated, err := svc.ApplyReload(ctx, accessor, "test-cm", matcher.ResourceTypeConfigMap, "default", "abc123hash", false)

	if err != nil {
		t.Fatalf("ApplyReload failed: %v", err)
	}

	if updated {
		t.Error("Expected updated to be false when hash is unchanged")
	}
}

func TestService_ProcessConfigMap_MultipleWorkloads(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	// Create multiple workloads
	deploy1 := testutil.NewDeployment(
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

	deploy2 := testutil.NewDeployment(
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
	deploy3 := testutil.NewDeployment(
		"deploy3", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

	// Create deployments in different namespaces
	deploy1 := testutil.NewDeployment(
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

	deploy2 := testutil.NewDeployment(
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

	workloads := []workload.Workload{
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
	svc := NewService(cfg, testr.New(t))

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
				svc := NewService(cfg, testr.New(t))

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
	svc := NewService(cfg, testr.New(t))

	tests := []struct {
		name         string
		volumes      []corev1.Volume
		resourceName string
		resourceType matcher.ResourceType
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
			resourceType: matcher.ResourceTypeConfigMap,
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
			resourceType: matcher.ResourceTypeConfigMap,
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
			resourceType: matcher.ResourceTypeConfigMap,
			wantVolume:   "",
		},
		{
			name:         "empty volumes",
			volumes:      []corev1.Volume{},
			resourceName: "my-cm",
			resourceType: matcher.ResourceTypeConfigMap,
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
	svc := NewService(cfg, testr.New(t))

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
				got := svc.findVolumeUsingResource(tt.volumes, tt.resourceName, matcher.ResourceTypeSecret)
				if got != tt.wantVolume {
					t.Errorf("findVolumeUsingResource() = %q, want %q", got, tt.wantVolume)
				}
			},
		)
	}
}

func TestService_findContainerWithVolumeMount(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

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
	svc := NewService(cfg, testr.New(t))

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
				got := svc.findContainerWithEnvRef(tt.containers, tt.resourceName, matcher.ResourceTypeConfigMap)
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
	svc := NewService(cfg, testr.New(t))

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
				got := svc.findContainerWithEnvRef(tt.containers, tt.resourceName, matcher.ResourceTypeSecret)
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
	svc := NewService(cfg, testr.New(t))

	// Test with autoReload=true and volume mount
	deploy := testutil.NewDeployment("test", "default", nil)
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

	container := svc.findTargetContainer(accessor, "my-cm", matcher.ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "app" {
		t.Errorf("Expected container 'app', got %q", container.Name)
	}
}

func TestService_findTargetContainer_AutoReload_EnvRef(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	// Test with autoReload=true and env ref (no volume)
	deploy := testutil.NewDeployment("test", "default", nil)
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

	container := svc.findTargetContainer(accessor, "my-cm", matcher.ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "app" {
		t.Errorf("Expected container 'app', got %q", container.Name)
	}
}

func TestService_findTargetContainer_AutoReload_InitContainer(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	// Test with autoReload=true where init container uses the volume
	deploy := testutil.NewDeployment("test", "default", nil)
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

	container := svc.findTargetContainer(accessor, "my-cm", matcher.ResourceTypeConfigMap, true)
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
	svc := NewService(cfg, testr.New(t))

	// Test with autoReload=true where init container has env ref
	deploy := testutil.NewDeployment("test", "default", nil)
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

	container := svc.findTargetContainer(accessor, "my-cm", matcher.ResourceTypeConfigMap, true)
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
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "my-cm", matcher.ResourceTypeConfigMap, false)
	if container != nil {
		t.Error("Expected nil container for empty container list")
	}
}

func TestService_findTargetContainer_NonAutoReload(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{Name: "first", Image: "nginx"},
		{Name: "second", Image: "busybox"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	// Without autoReload, should return first container
	container := svc.findTargetContainer(accessor, "my-cm", matcher.ResourceTypeConfigMap, false)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "first" {
		t.Errorf("Expected first container, got %q", container.Name)
	}
}

func TestService_findTargetContainer_AutoReload_FallbackToFirst(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	// autoReload=true but no matching volume or env ref - should fallback to first container
	deploy := testutil.NewDeployment("test", "default", nil)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		{Name: "first", Image: "nginx"},
		{Name: "second", Image: "busybox"},
	}
	accessor := workload.NewDeploymentWorkload(deploy)

	container := svc.findTargetContainer(accessor, "non-existent", matcher.ResourceTypeConfigMap, true)
	if container == nil {
		t.Fatal("Expected to find a container")
	}
	if container.Name != "first" {
		t.Errorf("Expected first container as fallback, got %q", container.Name)
	}
}

func TestService_ProcessNilChange(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test", "default", nil)
	workloads := []workload.Workload{workload.NewDeploymentWorkload(deploy)}

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

func TestServiceProcessSecretProviderClassAuto(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment("test-deploy", "default", map[string]string{
		"secretproviderclass.reloader.stakater.com/auto": "true",
	})
	workloads := []workload.Workload{workload.NewDeploymentWorkload(deploy)}

	change := SecretProviderClassChange{
		Name:      "my-spc",
		Namespace: "default",
		Status: csiv1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "my-spc",
			Objects:                 []csiv1.SecretProviderClassObject{{ID: "a", Version: "1"}},
		},
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)
	if len(decisions) != 1 || !decisions[0].ShouldReload {
		t.Fatalf("expected auto SPC reload, got %+v", decisions)
	}
}

func TestService_ProcessCreateEventDisabled(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnCreate = false
	svc := NewService(cfg, testr.New(t))

	deploy := testutil.NewDeployment(
		"test", "default", map[string]string{
			"reloader.stakater.com/auto": "true",
		},
	)
	workloads := []workload.Workload{workload.NewDeploymentWorkload(deploy)}

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

func TestService_ApplyReload_SPC_TargetsMountingContainer(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadStrategy = config.ReloadStrategyEnvVars
	svc := NewService(cfg, testr.New(t))

	// Two containers; only the second mounts the CSI volume that references the SPC.
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "multi", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "c0"},
						{Name: "c1", VolumeMounts: []corev1.VolumeMount{{Name: "spc-vol", MountPath: "/mnt/secrets-store"}}},
					},
					Volumes: []corev1.Volume{
						{
							Name: "spc-vol",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "my-spc"},
								},
							},
						},
					},
				},
			},
		},
	}
	accessor := workload.NewDeploymentWorkload(dep)

	// autoReload=true exercises volume-based container targeting.
	updated, err := svc.ApplyReload(context.Background(), accessor, "my-spc", matcher.ResourceTypeSecretProviderClass, "default", "spchash", true)
	if err != nil {
		t.Fatalf("ApplyReload failed: %v", err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}

	containers := accessor.GetContainers()
	const envName = "STAKATER_MY_SPC_SECRETPROVIDERCLASS"
	hasEnv := func(c corev1.Container) bool {
		for _, e := range c.Env {
			if e.Name == envName {
				return true
			}
		}
		return false
	}
	if hasEnv(containers[0]) {
		t.Error("env var must NOT land on container[0] (it does not mount the SPC volume)")
	}
	if !hasEnv(containers[1]) {
		t.Error("env var must land on container[1], which mounts the SPC CSI volume")
	}
}

func TestService_ProcessSecretProviderClass_GenericAuto(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))

	// Generic auto annotation (not the typed SPC one) must also trigger an SPC reload.
	deploy := testutil.NewDeployment("test-deploy", "default", map[string]string{
		"reloader.stakater.com/auto": "true",
	})
	workloads := []workload.Workload{workload.NewDeploymentWorkload(deploy)}

	change := SecretProviderClassChange{
		Name:      "my-spc",
		Namespace: "default",
		Status: csiv1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "my-spc",
			Objects:                 []csiv1.SecretProviderClassObject{{ID: "a", Version: "1"}},
		},
		EventType: EventTypeUpdate,
	}

	decisions := svc.Process(change, workloads)
	if len(decisions) != 1 || !decisions[0].ShouldReload {
		t.Fatalf("expected generic-auto SPC reload, got %+v", decisions)
	}
}

func TestService_Process_SyncWindows(t *testing.T) {
	cfg := config.NewDefault()
	// Exact due times; the spread is covered by TestService_SyncWindowSpread.
	cfg.SyncWindowSpread = 0
	// Tuesday 2026-11-10 10:00 UTC.
	now := time.Date(2026, time.November, 10, 10, 0, 0, 0, time.UTC)

	cm := testutil.NewConfigMap("test-cm", "default")
	change := ConfigMapChange{ConfigMap: cm, EventType: EventTypeUpdate}
	hash := change.ComputeHash(NewHasher())

	allowAtTwo := `- kind: allow
  schedule: "0 2 * * *"
  duration: 1h
`
	denyNow := `[{"kind":"deny","schedule":"0 9 * * *","duration":"2h"}]`
	denyTonight := `[{"kind":"deny","schedule":"0 22 * * *","duration":"2h"}]`
	denyTonightSpread := `[{"kind":"deny","schedule":"0 22 * * *","duration":"2h","spread":"10m"}]`
	pending := func(hash string, notBefore time.Time) string {
		return `[{"kind":"configmap","name":"test-cm","hash":"` + hash + `","notBefore":"` + notBefore.Format(time.RFC3339) + `"}]`
	}

	tests := []struct {
		name               string
		workloadAnnotation map[string]string
		podAnnotation      map[string]string
		wantShouldReload   bool
		wantDeferUntil     time.Time
	}{
		{
			name:               "no sync windows reloads now",
			workloadAnnotation: map[string]string{cfg.Annotations.ConfigmapReload: "test-cm"},
			wantShouldReload:   true,
		},
		{
			name: "outside allow window defers until it opens",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     allowAtTwo,
			},
			wantShouldReload: true,
			wantDeferUntil:   time.Date(2026, time.November, 11, 2, 0, 0, 0, time.UTC),
		},
		{
			name: "inside deny window defers until it closes",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyNow,
			},
			wantShouldReload: true,
			wantDeferUntil:   now.Add(time.Hour),
		},
		{
			name: "outside deny window reloads now",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyTonight,
			},
			wantShouldReload: true,
		},
		{
			name:               "pod template annotation is honored",
			workloadAnnotation: map[string]string{cfg.Annotations.ConfigmapReload: "test-cm"},
			podAnnotation:      map[string]string{cfg.Annotations.SyncWindows: denyNow},
			wantShouldReload:   true,
			wantDeferUntil:     now.Add(time.Hour),
		},
		{
			name: "workload annotation takes precedence over pod template",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     allowAtTwo,
			},
			podAnnotation:    map[string]string{cfg.Annotations.SyncWindows: denyNow},
			wantShouldReload: true,
			wantDeferUntil:   time.Date(2026, time.November, 11, 2, 0, 0, 0, time.UTC),
		},
		{
			name: "invalid sync windows reload now",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     `[{"kind":"sometimes","schedule":"0 2 * * *","duration":"1h"}]`,
			},
			wantShouldReload: true,
		},
		{
			name: "workload that does not match is not deferred",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "other-cm",
				cfg.Annotations.SyncWindows:     denyNow,
			},
			wantShouldReload: false,
		},
		{
			name: "pending reload for the same content keeps its due time",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyTonightSpread,
				cfg.Annotations.PendingReload:   pending(hash, now.Add(5*time.Minute)),
			},
			wantShouldReload: true,
			wantDeferUntil:   now.Add(5 * time.Minute),
		},
		{
			name: "pending reload due after the spread is ignored inside the window",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyTonightSpread,
				cfg.Annotations.PendingReload:   pending(hash, time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC)),
			},
			wantShouldReload: true,
		},
		{
			name: "pending reload due long after the window opens is replaced",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyNow,
				cfg.Annotations.PendingReload:   pending(hash, time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC)),
			},
			wantShouldReload: true,
			wantDeferUntil:   now.Add(time.Hour),
		},
		{
			name: "pending reload that is due reloads inside the window",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyTonight,
				cfg.Annotations.PendingReload:   pending(hash, now.Add(-time.Minute)),
			},
			wantShouldReload: true,
		},
		{
			name: "pending reload that missed its window waits for the next one",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyNow,
				cfg.Annotations.PendingReload:   pending(hash, now.Add(-2*time.Hour)),
			},
			wantShouldReload: true,
			wantDeferUntil:   now.Add(time.Hour),
		},
		{
			name: "pending reload for older content does not delay a new change",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.SyncWindows:     denyTonight,
				cfg.Annotations.PendingReload:   pending("older", now.Add(5*time.Minute)),
			},
			wantShouldReload: true,
		},
		{
			name: "pending reload without sync windows reloads now",
			workloadAnnotation: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
				cfg.Annotations.PendingReload:   pending(hash, now.Add(5*time.Minute)),
			},
			wantShouldReload: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(cfg, testr.New(t))
			svc.now = func() time.Time { return now }

			deploy := testutil.NewDeployment("test-deploy", "default", tt.workloadAnnotation)
			deploy.Spec.Template.Annotations = tt.podAnnotation

			decisions := svc.Process(change, []workload.Workload{workload.NewDeploymentWorkload(deploy)})

			if len(decisions) != 1 {
				t.Fatalf("Expected 1 decision, got %d", len(decisions))
			}
			if decisions[0].ShouldReload != tt.wantShouldReload {
				t.Errorf("ShouldReload = %v, want %v", decisions[0].ShouldReload, tt.wantShouldReload)
			}
			if !decisions[0].DeferUntil.Equal(tt.wantDeferUntil) {
				t.Errorf("DeferUntil = %v, want %v", decisions[0].DeferUntil, tt.wantDeferUntil)
			}
		})
	}
}

func TestService_Process_SyncWindowsIgnoredForJobsAndCronJobs(t *testing.T) {
	cfg := config.NewDefault()
	annotations := map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     `[{"kind":"deny","schedule":"0 * * * *","duration":"90m"}]`,
	}
	workloads := []workload.Workload{
		workload.NewCronJobWorkload(testutil.NewCronJobWithAnnotations("cronjob", "default", annotations)),
		workload.NewJobWorkload(testutil.NewJobWithAnnotations("job", "default", annotations)),
	}

	svc := NewService(cfg, testr.New(t))
	change := ConfigMapChange{ConfigMap: testutil.NewConfigMap("test-cm", "default"), EventType: EventTypeUpdate}
	for _, d := range svc.Process(change, workloads) {
		if !d.ShouldReload || d.IsDeferred() {
			t.Errorf("%s: ShouldReload = %v, deferred = %v; want an immediate reload", d.Workload.Kind(), d.ShouldReload, d.IsDeferred())
		}
	}
}

func TestService_Process_SyncWindowsNeedPendingReloadAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Annotations.PendingReload = ""

	deploy := testutil.NewDeployment("test-deploy", "default", map[string]string{
		cfg.Annotations.ConfigmapReload: "test-cm",
		cfg.Annotations.SyncWindows:     `[{"kind":"deny","schedule":"0 * * * *","duration":"90m"}]`,
	})
	svc := NewService(cfg, testr.New(t))
	change := ConfigMapChange{ConfigMap: testutil.NewConfigMap("test-cm", "default"), EventType: EventTypeUpdate}
	decisions := svc.Process(change, []workload.Workload{workload.NewDeploymentWorkload(deploy)})

	if len(decisions) != 1 || !decisions[0].ShouldReload || decisions[0].IsDeferred() {
		t.Errorf("decisions = %+v, want an immediate reload when no pending-reload annotation is configured", decisions)
	}
}

func TestService_SyncWindowSpread(t *testing.T) {
	// Tuesday 2026-11-10 10:00 UTC; the allow window opens at 12:00 for an hour.
	now := time.Date(2026, time.November, 10, 10, 0, 0, 0, time.UTC)
	opens := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)

	dueTimes := func(t *testing.T, cfg *config.Config, windows string, count int) []time.Time {
		t.Helper()
		svc := NewService(cfg, testr.New(t))
		svc.now = func() time.Time { return now }

		var workloads []workload.Workload
		for i := range count {
			workloads = append(workloads, workload.NewDeploymentWorkload(testutil.NewDeployment(
				fmt.Sprintf("deploy-%d", i), "default", map[string]string{
					cfg.Annotations.ConfigmapReload: "test-cm",
					cfg.Annotations.SyncWindows:     windows,
				},
			)))
		}
		change := ConfigMapChange{ConfigMap: testutil.NewConfigMap("test-cm", "default"), EventType: EventTypeUpdate}

		var due []time.Time
		for _, d := range svc.Process(change, workloads) {
			due = append(due, d.DeferUntil)
		}
		return due
	}

	t.Run("reloads are spread over the default spread", func(t *testing.T) {
		cfg := config.NewDefault()
		due := dueTimes(t, cfg, `[{"kind":"allow","schedule":"0 12 * * *","duration":"1h"}]`, 50)

		distinct := map[time.Time]bool{}
		for _, at := range due {
			if at.Before(opens) || !at.Before(opens.Add(cfg.SyncWindowSpread)) {
				t.Errorf("due time %s is outside [%s, %s)", at, opens, opens.Add(cfg.SyncWindowSpread))
			}
			distinct[at] = true
		}
		if len(distinct) < 10 {
			t.Errorf("50 workloads got only %d distinct due times, want them spread out", len(distinct))
		}
	})

	t.Run("due time is stable for a workload", func(t *testing.T) {
		cfg := config.NewDefault()
		windows := `[{"kind":"allow","schedule":"0 12 * * *","duration":"1h"}]`
		first, second := dueTimes(t, cfg, windows, 5), dueTimes(t, cfg, windows, 5)
		for i := range first {
			if !first[i].Equal(second[i]) {
				t.Errorf("workload %d got due times %s and %s, want the same", i, first[i], second[i])
			}
		}
	})

	t.Run("spread is capped at half of the window", func(t *testing.T) {
		cfg := config.NewDefault()
		cfg.SyncWindowSpread = 4 * time.Hour
		for _, at := range dueTimes(t, cfg, `[{"kind":"allow","schedule":"0 12 * * *","duration":"1h"}]`, 50) {
			if !at.Before(opens.Add(30 * time.Minute)) {
				t.Errorf("due time %s is not in the first half of the window", at)
			}
		}
	})

	t.Run("window spread overrides the default", func(t *testing.T) {
		cfg := config.NewDefault()
		for _, at := range dueTimes(t, cfg, `[{"kind":"allow","schedule":"0 12 * * *","duration":"1h","spread":"0s"}]`, 5) {
			if !at.Equal(opens) {
				t.Errorf("due time = %s, want %s with a zero spread", at, opens)
			}
		}
	})
}
