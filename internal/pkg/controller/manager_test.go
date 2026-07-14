package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestBuildDefaultNamespaces(t *testing.T) {
	if got := buildDefaultNamespaces(nil); got != nil {
		t.Errorf("empty input should return nil, got %v", got)
	}
	if got := buildDefaultNamespaces([]string{}); got != nil {
		t.Errorf("empty slice should return nil, got %v", got)
	}

	got := buildDefaultNamespaces([]string{"team-a", "team-b", "team-c"})
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	for _, ns := range []string{"team-a", "team-b", "team-c"} {
		if _, ok := got[ns]; !ok {
			t.Errorf("missing namespace %q in %v", ns, got)
		}
	}
}

func TestAddOptionalSchemesRegistersCSI(t *testing.T) {
	// Reset to a clean scheme for the test.
	runtimeScheme = runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(runtimeScheme))

	AddOptionalSchemes(false, false, true)

	gvk := schema.GroupVersionKind{
		Group:   "secrets-store.csi.x-k8s.io",
		Version: "v1",
		Kind:    "SecretProviderClassPodStatus",
	}
	if !runtimeScheme.Recognizes(gvk) {
		t.Fatal("expected CSI SecretProviderClassPodStatus to be registered in scheme")
	}
}
