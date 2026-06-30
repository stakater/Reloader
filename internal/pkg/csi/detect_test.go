package csi

import (
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fake "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

func newFakeDiscovery(resources []*metav1.APIResourceList) *fake.FakeDiscovery {
	return &fake.FakeDiscovery{
		Fake: &clienttesting.Fake{Resources: resources},
	}
}

func TestHasCSISupportTrue(t *testing.T) {
	d := newFakeDiscovery([]*metav1.APIResourceList{
		{
			GroupVersion: "secrets-store.csi.x-k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "secretproviderclasspodstatuses"},
				{Name: "secretproviderclasses"},
			},
		},
	})
	if !HasCSISupport(d, logr.Discard()) {
		t.Fatal("expected CSI support detected")
	}
}

func TestHasCSISupportFalse(t *testing.T) {
	d := newFakeDiscovery(nil)
	if HasCSISupport(d, logr.Discard()) {
		t.Fatal("expected CSI support not detected")
	}
}
