package main

import (
	"reflect"
	"testing"

	"github.com/stakater/Reloader/pkg/config"
)

func TestNamespaceWatchScope(t *testing.T) {
	tests := []struct {
		name              string
		watchedNamespaces []string
		ignoredNamespaces []string
		wantMsg           string
		wantKV            []any
	}{
		{
			name:    "global mode with no ignored namespaces",
			wantMsg: "watching all namespaces",
		},
		{
			name:              "global mode reports the ignored namespaces",
			ignoredNamespaces: []string{"kube-system"},
			wantMsg:           "watching all namespaces except the ignored ones",
			wantKV:            []any{"ignoredNamespaces", []string{"kube-system"}},
		},
		{
			name:              "global mode reports several ignored namespaces",
			ignoredNamespaces: []string{"kube-system", "kube-public"},
			wantMsg:           "watching all namespaces except the ignored ones",
			wantKV:            []any{"ignoredNamespaces", []string{"kube-system", "kube-public"}},
		},
		{
			name:              "scoped mode reports the watched namespaces",
			watchedNamespaces: []string{"team-a"},
			wantMsg:           "watching scoped namespaces",
			wantKV:            []any{"namespaces", []string{"team-a"}},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cfg := config.NewDefault()
				cfg.WatchedNamespaces = tt.watchedNamespaces
				cfg.IgnoredNamespaces = tt.ignoredNamespaces

				gotMsg, gotKV := namespaceWatchScope(cfg)
				if gotMsg != tt.wantMsg {
					t.Errorf("namespaceWatchScope() msg = %q, want %q", gotMsg, tt.wantMsg)
				}
				if !reflect.DeepEqual(gotKV, tt.wantKV) {
					t.Errorf("namespaceWatchScope() kv = %v, want %v", gotKV, tt.wantKV)
				}
			},
		)
	}
}
