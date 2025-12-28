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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestSecretReconciler(t *testing.T, cfg *config.Config, objects ...runtime.Object) *controller.SecretReconciler {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		Build()

	collectors := metrics.NewCollectors()

	return &controller.SecretReconciler{
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

func TestSecretReconciler_NotFound(t *testing.T) {
	cfg := config.NewDefault()
	reconciler := newTestSecretReconciler(t, cfg)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent-secret",
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

func TestSecretReconciler_NotFound_ReloadOnDelete(t *testing.T) {
	cfg := config.NewDefault()
	cfg.ReloadOnDelete = true

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "deleted-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "deleted-secret",
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

func TestSecretReconciler_IgnoredNamespace(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	reconciler := newTestSecretReconciler(t, cfg, secret)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_NoMatchingWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"key": []byte("value")},
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_MatchingDeployment_AutoAnnotation(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"password": []byte("secret123")},
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
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "test-secret",
								},
							},
						}},
					}},
				},
			},
		},
	}

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_MatchingDeployment_ExplicitAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"password": []byte("secret123")},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "test-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_WorkloadInDifferentNamespace(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "namespace-a",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "namespace-b",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "test-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_IgnoredWorkloadType(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredWorkloads = []string{"deployment"}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "test-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_DaemonSet(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "test-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, secret, daemonset)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_StatefulSet(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "test-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, secret, statefulset)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_VolumeMount(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "volume-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"credentials": []byte("supersecret")},
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
							Name:      "secrets",
							MountPath: "/etc/secrets",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "secrets",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "volume-secret",
							},
						},
					}},
				},
			},
		},
	}

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "volume-secret",
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

func TestSecretReconciler_ProjectedVolume(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "projected-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"credentials": []byte("supersecret")},
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
							Name:      "secrets",
							MountPath: "/etc/secrets",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "secrets",
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "projected-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "projected-secret",
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

func TestSecretReconciler_EnvKeyRef(t *testing.T) {
	cfg := config.NewDefault()
	cfg.AutoReloadAll = true

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "key-ref-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"password": []byte("secret123")},
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
						Env: []corev1.EnvVar{{
							Name: "DB_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "key-ref-secret",
									},
									Key: "password",
								},
							},
						}},
					}},
				},
			},
		},
	}

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "key-ref-secret",
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

func TestSecretReconciler_MultipleWorkloads(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}

	deployment1 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-1",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "shared-secret",
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
				cfg.Annotations.SecretReload: "shared-secret",
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

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "statefulset-1",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "shared-secret",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "stateful"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "stateful"},
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment1, deployment2, statefulset)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "shared-secret",
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

func TestSecretReconciler_SearchAnnotation(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.Match: "true",
			},
		},
		Data: map[string][]byte{"key": []byte("value")},
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
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

func TestSecretReconciler_TLSSecret(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
			"tls.key": []byte("-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"),
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "tls-secret",
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

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "tls-secret",
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

func TestSecretReconciler_ImagePullSecret(t *testing.T) {
	cfg := config.NewDefault()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				cfg.Annotations.SecretReload: "registry-secret",
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
					ImagePullSecrets: []corev1.LocalObjectReference{{
						Name: "registry-secret",
					}},
				},
			},
		},
	}

	reconciler := newTestSecretReconciler(t, cfg, secret, deployment)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "registry-secret",
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
