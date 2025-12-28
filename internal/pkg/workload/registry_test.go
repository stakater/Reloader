package workload

import (
	"testing"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	openshiftv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewRegistry_WithoutArgoRollouts(t *testing.T) {
	r := NewRegistry(RegistryOptions{ArgoRolloutsEnabled: false})

	kinds := r.SupportedKinds()
	if len(kinds) != 5 {
		t.Errorf("SupportedKinds() = %d kinds, want 5", len(kinds))
	}

	for _, k := range kinds {
		if k == KindArgoRollout {
			t.Error("SupportedKinds() should not include ArgoRollout when disabled")
		}
	}

	if r.ListerFor(KindArgoRollout) != nil {
		t.Error("ListerFor(KindArgoRollout) should return nil when disabled")
	}
}

func TestNewRegistry_WithArgoRollouts(t *testing.T) {
	r := NewRegistry(RegistryOptions{ArgoRolloutsEnabled: true})

	kinds := r.SupportedKinds()
	if len(kinds) != 6 {
		t.Errorf("SupportedKinds() = %d kinds, want 6", len(kinds))
	}

	found := false
	for _, k := range kinds {
		if k == KindArgoRollout {
			found = true
			break
		}
	}
	if !found {
		t.Error("SupportedKinds() should include ArgoRollout when enabled")
	}

	if r.ListerFor(KindArgoRollout) == nil {
		t.Error("ListerFor(KindArgoRollout) should return a function when enabled")
	}
}

func TestRegistry_ListerFor_AllKinds(t *testing.T) {
	r := NewRegistry(RegistryOptions{ArgoRolloutsEnabled: true})

	tests := []struct {
		kind    Kind
		wantNil bool
	}{
		{KindDeployment, false},
		{KindDaemonSet, false},
		{KindStatefulSet, false},
		{KindJob, false},
		{KindCronJob, false},
		{KindArgoRollout, false},
		{Kind("unknown"), true},
	}

	for _, tt := range tests {
		lister := r.ListerFor(tt.kind)
		if (lister == nil) != tt.wantNil {
			t.Errorf("ListerFor(%s) = nil? %v, want nil? %v", tt.kind, lister == nil, tt.wantNil)
		}
	}
}

func TestRegistry_FromObject_Deployment(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	w, err := r.FromObject(deploy)
	if err != nil {
		t.Fatalf("FromObject(Deployment) error = %v", err)
	}
	if w.Kind() != KindDeployment {
		t.Errorf("FromObject(Deployment).Kind() = %v, want %v", w.Kind(), KindDeployment)
	}
}

func TestRegistry_FromObject_DaemonSet(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	w, err := r.FromObject(ds)
	if err != nil {
		t.Fatalf("FromObject(DaemonSet) error = %v", err)
	}
	if w.Kind() != KindDaemonSet {
		t.Errorf("FromObject(DaemonSet).Kind() = %v, want %v", w.Kind(), KindDaemonSet)
	}
}

func TestRegistry_FromObject_StatefulSet(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	w, err := r.FromObject(sts)
	if err != nil {
		t.Fatalf("FromObject(StatefulSet) error = %v", err)
	}
	if w.Kind() != KindStatefulSet {
		t.Errorf("FromObject(StatefulSet).Kind() = %v, want %v", w.Kind(), KindStatefulSet)
	}
}

func TestRegistry_FromObject_Job(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	w, err := r.FromObject(job)
	if err != nil {
		t.Fatalf("FromObject(Job) error = %v", err)
	}
	if w.Kind() != KindJob {
		t.Errorf("FromObject(Job).Kind() = %v, want %v", w.Kind(), KindJob)
	}
}

func TestRegistry_FromObject_CronJob(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	w, err := r.FromObject(cj)
	if err != nil {
		t.Fatalf("FromObject(CronJob) error = %v", err)
	}
	if w.Kind() != KindCronJob {
		t.Errorf("FromObject(CronJob).Kind() = %v, want %v", w.Kind(), KindCronJob)
	}
}

func TestRegistry_FromObject_Rollout_Enabled(t *testing.T) {
	r := NewRegistry(RegistryOptions{ArgoRolloutsEnabled: true})
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	w, err := r.FromObject(rollout)
	if err != nil {
		t.Fatalf("FromObject(Rollout) error = %v", err)
	}
	if w.Kind() != KindArgoRollout {
		t.Errorf("FromObject(Rollout).Kind() = %v, want %v", w.Kind(), KindArgoRollout)
	}
}

func TestRegistry_FromObject_Rollout_Disabled(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	rollout := &argorolloutv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	_, err := r.FromObject(rollout)
	if err == nil {
		t.Error("FromObject(Rollout) should return error when Argo Rollouts disabled")
	}
}

func TestRegistry_FromObject_UnsupportedType(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	_, err := r.FromObject(cm)
	if err == nil {
		t.Error("FromObject(ConfigMap) should return error for unsupported type")
	}
}

func TestKindFromString(t *testing.T) {
	tests := []struct {
		input   string
		want    Kind
		wantErr bool
	}{
		// Lowercase
		{"deployment", KindDeployment, false},
		{"daemonset", KindDaemonSet, false},
		{"statefulset", KindStatefulSet, false},
		{"job", KindJob, false},
		{"cronjob", KindCronJob, false},
		{"rollout", KindArgoRollout, false},
		// Plural forms
		{"deployments", KindDeployment, false},
		{"daemonsets", KindDaemonSet, false},
		{"statefulsets", KindStatefulSet, false},
		{"jobs", KindJob, false},
		{"cronjobs", KindCronJob, false},
		{"rollouts", KindArgoRollout, false},
		// Mixed case
		{"Deployment", KindDeployment, false},
		{"DAEMONSET", KindDaemonSet, false},
		{"StatefulSet", KindStatefulSet, false},
		// Unknown
		{"unknown", "", true},
		{"replicaset", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := KindFromString(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("KindFromString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("KindFromString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNewLister(t *testing.T) {
	r := NewRegistry(RegistryOptions{})
	l := NewLister(nil, r, nil)

	if l == nil {
		t.Fatal("NewLister should not return nil")
	}
	if l.Registry != r {
		t.Error("NewLister should set Registry")
	}
}

// DeploymentConfig registry tests
func TestNewRegistry_WithDeploymentConfig(t *testing.T) {
	r := NewRegistry(RegistryOptions{DeploymentConfigEnabled: true})

	kinds := r.SupportedKinds()
	if len(kinds) != 6 {
		t.Errorf("SupportedKinds() = %d kinds, want 6", len(kinds))
	}

	found := false
	for _, k := range kinds {
		if k == KindDeploymentConfig {
			found = true
			break
		}
	}
	if !found {
		t.Error("SupportedKinds() should include DeploymentConfig when enabled")
	}

	if r.ListerFor(KindDeploymentConfig) == nil {
		t.Error("ListerFor(KindDeploymentConfig) should return a function when enabled")
	}
}

func TestNewRegistry_WithoutDeploymentConfig(t *testing.T) {
	r := NewRegistry(RegistryOptions{DeploymentConfigEnabled: false})

	for _, k := range r.SupportedKinds() {
		if k == KindDeploymentConfig {
			t.Error("SupportedKinds() should not include DeploymentConfig when disabled")
		}
	}

	if r.ListerFor(KindDeploymentConfig) != nil {
		t.Error("ListerFor(KindDeploymentConfig) should return nil when disabled")
	}
}

func TestNewRegistry_WithBothOptionalWorkloads(t *testing.T) {
	r := NewRegistry(RegistryOptions{
		ArgoRolloutsEnabled:     true,
		DeploymentConfigEnabled: true,
	})

	kinds := r.SupportedKinds()
	if len(kinds) != 7 {
		t.Errorf("SupportedKinds() = %d kinds, want 7 (5 base + ArgoRollout + DeploymentConfig)", len(kinds))
	}

	foundRollout := false
	foundDC := false
	for _, k := range kinds {
		if k == KindArgoRollout {
			foundRollout = true
		}
		if k == KindDeploymentConfig {
			foundDC = true
		}
	}
	if !foundRollout {
		t.Error("SupportedKinds() should include ArgoRollout")
	}
	if !foundDC {
		t.Error("SupportedKinds() should include DeploymentConfig")
	}
}

func TestRegistry_FromObject_DeploymentConfig_Enabled(t *testing.T) {
	r := NewRegistry(RegistryOptions{DeploymentConfigEnabled: true})
	dc := &openshiftv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	w, err := r.FromObject(dc)
	if err != nil {
		t.Fatalf("FromObject(DeploymentConfig) error = %v", err)
	}
	if w.Kind() != KindDeploymentConfig {
		t.Errorf("FromObject(DeploymentConfig).Kind() = %v, want %v", w.Kind(), KindDeploymentConfig)
	}
}

func TestRegistry_FromObject_DeploymentConfig_Disabled(t *testing.T) {
	r := NewRegistry(RegistryOptions{DeploymentConfigEnabled: false})
	dc := &openshiftv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	_, err := r.FromObject(dc)
	if err == nil {
		t.Error("FromObject(DeploymentConfig) should return error when DeploymentConfig disabled")
	}
}

func TestKindFromString_DeploymentConfig(t *testing.T) {
	tests := []struct {
		input   string
		want    Kind
		wantErr bool
	}{
		{"deploymentconfig", KindDeploymentConfig, false},
		{"deploymentconfigs", KindDeploymentConfig, false},
		{"DeploymentConfig", KindDeploymentConfig, false},
		{"DEPLOYMENTCONFIG", KindDeploymentConfig, false},
	}

	for _, tt := range tests {
		got, err := KindFromString(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("KindFromString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("KindFromString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
