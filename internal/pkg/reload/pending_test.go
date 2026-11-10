package reload

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/pkg/matcher"
)

const pendingKey = "reloader.stakater.com/pending-reload"

func TestPendingReload_SetFindClear(t *testing.T) {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app"}}
	due := time.Date(2026, time.November, 11, 2, 7, 30, 500, time.FixedZone("CET", 3600))
	cm := PendingReload{Kind: matcher.ResourceTypeConfigMap, Name: "app-cm", Hash: "h1", NotBefore: due}
	secret := PendingReload{Kind: matcher.ResourceTypeSecret, Name: "app-cm", Hash: "h2", NotBefore: due}

	if !SetPendingReload(deploy, pendingKey, cm) {
		t.Fatal("SetPendingReload() = false for a new entry, want true")
	}
	if SetPendingReload(deploy, pendingKey, cm) {
		t.Error("SetPendingReload() = true for an unchanged entry, want false")
	}
	if !SetPendingReload(deploy, pendingKey, secret) {
		t.Fatal("SetPendingReload() = false for a second resource, want true")
	}

	got, ok := FindPendingReload(deploy.Annotations, pendingKey, matcher.ResourceTypeConfigMap, "app-cm")
	if !ok {
		t.Fatal("FindPendingReload() found nothing for the ConfigMap")
	}
	if want := due.UTC().Truncate(time.Second); !got.NotBefore.Equal(want) || got.Hash != "h1" {
		t.Errorf("FindPendingReload() = %+v, want hash h1 and NotBefore %s", got, want)
	}

	cm.Hash = "h3"
	if !SetPendingReload(deploy, pendingKey, cm) {
		t.Error("SetPendingReload() = false for new content, want true")
	}
	if got, _ := FindPendingReload(deploy.Annotations, pendingKey, matcher.ResourceTypeConfigMap, "app-cm"); got.Hash != "h3" {
		t.Errorf("entry hash = %q after replacing it, want h3", got.Hash)
	}
	if n := len(ParsePendingReloads(deploy.Annotations[pendingKey])); n != 2 {
		t.Errorf("annotation holds %d entries, want 2", n)
	}

	if !ClearPendingReload(deploy, pendingKey, matcher.ResourceTypeConfigMap, "app-cm") {
		t.Error("ClearPendingReload() = false for an existing entry, want true")
	}
	if ClearPendingReload(deploy, pendingKey, matcher.ResourceTypeConfigMap, "app-cm") {
		t.Error("ClearPendingReload() = true for a missing entry, want false")
	}
	if _, ok := FindPendingReload(deploy.Annotations, pendingKey, matcher.ResourceTypeSecret, "app-cm"); !ok {
		t.Error("clearing the ConfigMap entry removed the Secret entry")
	}
	if !ClearPendingReload(deploy, pendingKey, matcher.ResourceTypeSecret, "app-cm") {
		t.Error("ClearPendingReload() = false for the last entry, want true")
	}
	if _, ok := deploy.Annotations[pendingKey]; ok {
		t.Error("annotation is still present after its last entry was cleared")
	}
}

func TestPendingReload_UnparseableAnnotation(t *testing.T) {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{pendingKey: "not json"},
	}}

	if entries := ParsePendingReloads(deploy.Annotations[pendingKey]); entries != nil {
		t.Errorf("ParsePendingReloads() = %v, want no entries", entries)
	}
	if !ClearPendingReload(deploy, pendingKey, matcher.ResourceTypeConfigMap, "app-cm") {
		t.Error("ClearPendingReload() = false, want the unparseable annotation removed")
	}
	if _, ok := deploy.Annotations[pendingKey]; ok {
		t.Error("unparseable annotation was not removed")
	}
}
