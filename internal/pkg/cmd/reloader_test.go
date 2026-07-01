package cmd

import "testing"

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
