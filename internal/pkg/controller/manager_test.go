package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

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
