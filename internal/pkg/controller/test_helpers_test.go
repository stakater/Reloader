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

// testScheme is a shared scheme for all controller tests.
func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	return scheme
}

// newConfigMapReconciler creates a ConfigMapReconciler for testing.
func newConfigMapReconciler(t *testing.T, cfg *config.Config, objects ...runtime.Object) *controller.ConfigMapReconciler {
	t.Helper()
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme()).
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

// newSecretReconciler creates a SecretReconciler for testing.
func newSecretReconciler(t *testing.T, cfg *config.Config, objects ...runtime.Object) *controller.SecretReconciler {
	t.Helper()
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme()).
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

// testConfigMap creates a ConfigMap for testing.
func testConfigMap(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{"key": "value"},
	}
}

// testConfigMapWithAnnotations creates a ConfigMap with annotations.
func testConfigMapWithAnnotations(name, namespace string, annotations map[string]string) *corev1.ConfigMap {
	cm := testConfigMap(name, namespace)
	cm.Annotations = annotations
	return cm
}

// testSecret creates a Secret for testing.
func testSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{"key": []byte("value")},
	}
}

// testSecretWithAnnotations creates a Secret with annotations.
func testSecretWithAnnotations(name, namespace string, annotations map[string]string) *corev1.Secret {
	secret := testSecret(name, namespace)
	secret.Annotations = annotations
	return secret
}

// testDeployment creates a minimal Deployment for testing.
func testDeployment(name, namespace string, annotations map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
}

// testDeploymentWithEnvFrom creates a Deployment with EnvFrom referencing a ConfigMap or Secret.
func testDeploymentWithEnvFrom(name, namespace string, configMapName, secretName string) *appsv1.Deployment {
	d := testDeployment(name, namespace, nil)
	if configMapName != "" {
		d.Spec.Template.Spec.Containers[0].EnvFrom = append(
			d.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				},
			},
		)
	}
	if secretName != "" {
		d.Spec.Template.Spec.Containers[0].EnvFrom = append(
			d.Spec.Template.Spec.Containers[0].EnvFrom,
			corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			},
		)
	}
	return d
}

// testDeploymentWithVolume creates a Deployment with a volume from ConfigMap or Secret.
func testDeploymentWithVolume(name, namespace string, configMapName, secretName string) *appsv1.Deployment {
	d := testDeployment(name, namespace, nil)
	d.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/etc/config",
		},
	}

	if configMapName != "" {
		d.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					},
				},
			},
		}
	}
	if secretName != "" {
		d.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
					},
				},
			},
		}
	}
	return d
}

// testDeploymentWithProjectedVolume creates a Deployment with a projected volume.
func testDeploymentWithProjectedVolume(name, namespace string, configMapName, secretName string) *appsv1.Deployment {
	d := testDeployment(name, namespace, nil)
	d.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/etc/config",
		},
	}

	var sources []corev1.VolumeProjection
	if configMapName != "" {
		sources = append(
			sources, corev1.VolumeProjection{
				ConfigMap: &corev1.ConfigMapProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				},
			},
		)
	}
	if secretName != "" {
		sources = append(
			sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			},
		)
	}

	d.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{Sources: sources},
			},
		},
	}
	return d
}

// testDaemonSet creates a minimal DaemonSet for testing.
func testDaemonSet(name, namespace string, annotations map[string]string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
}

// testStatefulSet creates a minimal StatefulSet for testing.
func testStatefulSet(name, namespace string, annotations map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
}

// reconcileRequest creates a ctrl.Request for the given name and namespace.
func reconcileRequest(name, namespace string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// namespaceRequest creates a ctrl.Request for a namespace (no namespace field needed).
func namespaceRequest(name string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name},
	}
}

// testNamespace creates a Namespace with optional labels.
func testNamespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

// newNamespaceReconciler creates a NamespaceReconciler for testing.
func newNamespaceReconciler(t *testing.T, cfg *config.Config, cache *controller.NamespaceCache, objects ...runtime.Object) *controller.NamespaceReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		Build()

	return &controller.NamespaceReconciler{
		Client: fakeClient,
		Log:    testr.New(t),
		Config: cfg,
		Cache:  cache,
	}
}

// assertReconcileSuccess runs reconcile and asserts no error and no requeue.
func assertReconcileSuccess(t *testing.T, reconciler interface {
	Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
}, req ctrl.Request) {
	t.Helper()
	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue")
	}
}

// testJob creates a minimal Job for testing.
func testJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "busybox",
						},
					},
				},
			},
		},
	}
}

// testCronJob creates a minimal CronJob for testing.
func testCronJob(name, namespace string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "main",
									Image: "busybox",
								},
							},
						},
					},
				},
			},
		},
	}
}
