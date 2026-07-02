package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveWatchNamespaces(t *testing.T) {
	tests := []struct {
		name                string
		namespaces          []string
		kubernetesNamespace string
		wantNamespaces      []string
		wantGlobal          bool
	}{
		{
			name:                "scoped mode takes precedence over env",
			namespaces:          []string{"team-a", "team-b"},
			kubernetesNamespace: "reloader-system",
			wantNamespaces:      []string{"team-a", "team-b"},
			wantGlobal:          false,
		},
		{
			name:                "scoped mode with single namespace",
			namespaces:          []string{"team-a"},
			kubernetesNamespace: "",
			wantNamespaces:      []string{"team-a"},
			wantGlobal:          false,
		},
		{
			name:                "single namespace mode from env",
			namespaces:          nil,
			kubernetesNamespace: "reloader-system",
			wantNamespaces:      []string{"reloader-system"},
			wantGlobal:          false,
		},
		{
			name:                "global mode when nothing set",
			namespaces:          nil,
			kubernetesNamespace: "",
			wantNamespaces:      []string{v1.NamespaceAll},
			wantGlobal:          true,
		},
		{
			name:                "empty list falls back to env",
			namespaces:          []string{},
			kubernetesNamespace: "reloader-system",
			wantNamespaces:      []string{"reloader-system"},
			wantGlobal:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNamespaces, gotGlobal := resolveWatchNamespaces(tt.namespaces, tt.kubernetesNamespace)
			assert.Equal(t, tt.wantNamespaces, gotNamespaces)
			assert.Equal(t, tt.wantGlobal, gotGlobal)
		})
	}
}

func TestNamespaceWatchScopeMessage(t *testing.T) {
	tests := []struct {
		name              string
		ignoredNamespaces []string
		want              string
	}{
		{
			name:              "no ignored namespaces - all namespaces",
			ignoredNamespaces: nil,
			want:              "KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces.",
		},
		{
			name:              "empty ignored namespaces - all namespaces",
			ignoredNamespaces: []string{},
			want:              "KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces.",
		},
		{
			name:              "single ignored namespace",
			ignoredNamespaces: []string{"kube-system"},
			want:              "KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces except: kube-system.",
		},
		{
			name:              "multiple ignored namespaces",
			ignoredNamespaces: []string{"kube-system", "kube-public"},
			want:              "KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces except: kube-system, kube-public.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := namespaceWatchScopeMessage(tt.ignoredNamespaces); got != tt.want {
				t.Errorf("namespaceWatchScopeMessage(%v) = %q, want %q", tt.ignoredNamespaces, got, tt.want)
			}
		})
	}
}
