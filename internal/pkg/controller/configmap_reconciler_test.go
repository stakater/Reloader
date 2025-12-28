package controller_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stakater/Reloader/internal/pkg/alerting"
	"github.com/stakater/Reloader/internal/pkg/config"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/events"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/webhook"
	"github.com/stakater/Reloader/internal/pkg/workload"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	return scheme
}

func newTestConfigMapReconciler(t *testing.T, cfg *config.Config, objects ...runtime.Object) *controller.ConfigMapReconciler {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		Build()

	collectors := metrics.NewCollectors()

	return &controller.ConfigMapReconciler{
		Client:        fakeClient,
		Log:           testr.New(t),
		Config:        cfg,
		ReloadService: reload.NewService(cfg),
		Registry:      workload.NewRegistry(cfg.ArgoRolloutsEnabled),
		Collectors:    &collectors,
		EventRecorder: events.NewRecorder(nil),
		WebhookClient: webhook.NewClient("", testr.New(t)),
		Alerter:       &alerting.NoOpAlerter{},
	}
}

func TestConfigMapReconciler_NotFound(t *testing.T) {
	cfg := config.NewDefault()
	reconciler := newTestConfigMapReconciler(t, cfg)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue for NotFound")
	}
}

func TestConfigMapReconciler_NotFound_ReloadOnDelete(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnDelete = true

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "deleted-cm",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "deleted-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_IgnoredNamespace(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "kube-system",
		},
		Data: map[string]string{"key": "value"},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "kube-system",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue for ignored namespace")
	}
}

func TestConfigMapReconciler_NoMatchingWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_MatchingDeployment_AutoAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "test-cm",
								},
							},
						}},
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_MatchingDeployment_ExplicitAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_WorkloadInDifferentNamespace(t *testing.T) {
	cfg := config.NewDefault()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "namespace-a",
		},
		Data: map[string]string{"key": "value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "namespace-b",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "namespace-a",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_IgnoredWorkloadType(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredWorkloads = []string{"deployment"}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_DaemonSet(t *testing.T) {
	cfg := config.NewDefault()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, daemonset)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_StatefulSet(t *testing.T) {
	cfg := config.NewDefault()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "test-cm",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, statefulset)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_MultipleWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	deployment1 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-1",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "shared-cm",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test1"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test1"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	deployment2 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-2",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "shared-cm",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test2"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test2"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "daemonset-1",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.ConfigmapReload: "shared-cm",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "daemon"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "daemon"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment1, deployment2, daemonset)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "shared-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_VolumeMount(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "volume-cm",
			Namespace: "default",
		},
		Data: map[string]string{"config.yaml": "key: value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "config",
							MountPath: "/etc/config",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "volume-cm",
								},
							},
						},
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "volume-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_ProjectedVolume(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "projected-cm",
			Namespace: "default",
		},
		Data: map[string]string{"config.yaml": "key: value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "config",
							MountPath: "/etc/config",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{{
									ConfigMap: &corev1.ConfigMapProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "projected-cm",
										},
									},
								}},
							},
						},
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "projected-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

func TestConfigMapReconciler_SearchAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.Match: "true",
			},
		},
		Data: map[string]string{"key": "value"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.Search: "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "nginx",
					}},
				},
			},
		},
	}

	reconciler := newTestConfigMapReconciler(t, cfg, cm, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}
