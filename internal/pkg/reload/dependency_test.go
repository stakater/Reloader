package reload

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	corev1 "k8s.io/api/core/v1"

	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
	"github.com/stakater/Reloader/pkg/matcher"
)

func TestParseReloadAfter(t *testing.T) {
	tests := []struct {
		value   string
		want    []DependencyRef
		wantErr bool
	}{
		{value: "", want: nil},
		{value: "cronjob/migrate", want: []DependencyRef{{Kind: workload.KindCronJob, Name: "migrate"}}},
		{
			value: " Deployments/api , statefulset/db,deployment/api ",
			want: []DependencyRef{
				{Kind: workload.KindDeployment, Name: "api"},
				{Kind: workload.KindStatefulSet, Name: "db"},
			},
		},
		{value: "migrate", wantErr: true},
		{value: "cronjob/", wantErr: true},
		{value: "pod/migrate", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseReloadAfter(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseReloadAfter(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseReloadAfter(%q) = %v, want %v", tt.value, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseReloadAfter(%q)[%d] = %v, want %v", tt.value, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPendingReloadsRoundTrip(t *testing.T) {
	pending := PendingReloads{}
	entry := PendingReload{
		ResourceType: matcher.ResourceTypeSecret, ResourceName: "db", Hash: "h1",
		Dependencies: []DependencyRef{{Kind: workload.KindJob, Name: "migrate"}},
		Since:        time.Now().UTC().Truncate(time.Second),
	}
	pending[entry.Key()] = entry

	value, err := pending.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePendingReloads(value)
	if err != nil {
		t.Fatal(err)
	}
	if got["secret/db"].Hash != "h1" || got["secret/db"].Dependencies[0].Name != "migrate" {
		t.Errorf("round trip = %+v", got)
	}

	if empty, _ := (PendingReloads{}).Encode(); empty != "" {
		t.Errorf("empty Encode() = %q, want empty", empty)
	}
	if _, err := ParsePendingReloads("{not json"); err == nil {
		t.Error("expected error for invalid annotation")
	}
}

func TestAppliedCheck(t *testing.T) {
	cfg := config.NewDefault()
	svc := NewService(cfg, testr.New(t))
	check := svc.AppliedCheck("db-creds", matcher.ResourceTypeSecret, "h1")

	source, _ := json.Marshal(ReloadSource{Kind: string(matcher.ResourceTypeSecret), Name: "db-creds", Hash: "h1"})
	other, _ := json.Marshal(ReloadSource{Kind: string(matcher.ResourceTypeSecret), Name: "db-creds", Hash: "old"})

	tests := []struct {
		name string
		tmpl *corev1.PodTemplateSpec
		want bool
	}{
		{name: "nil template", tmpl: nil, want: false},
		{name: "untouched", tmpl: &corev1.PodTemplateSpec{}, want: false},
		{
			name: "env var with hash",
			tmpl: &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Env: []corev1.EnvVar{{Name: "STAKATER_DB_CREDS_SECRET", Value: "h1"}},
			}}}},
			want: true,
		},
		{
			name: "env var with old hash",
			tmpl: &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Env: []corev1.EnvVar{{Name: "STAKATER_DB_CREDS_SECRET", Value: "old"}},
			}}}},
			want: false,
		},
		{
			name: "last-reloaded-from with hash",
			tmpl: func() *corev1.PodTemplateSpec {
				tmpl := &corev1.PodTemplateSpec{}
				tmpl.Annotations = map[string]string{cfg.Annotations.LastReloadedFrom: string(source)}
				return tmpl
			}(),
			want: true,
		},
		{
			name: "last-reloaded-from with old hash",
			tmpl: func() *corev1.PodTemplateSpec {
				tmpl := &corev1.PodTemplateSpec{}
				tmpl.Annotations = map[string]string{cfg.Annotations.LastReloadedFrom: string(other)}
				return tmpl
			}(),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := check(tt.tmpl); got != tt.want {
				t.Errorf("AppliedCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}
