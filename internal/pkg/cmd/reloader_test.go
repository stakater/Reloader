package cmd

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/stakater/Reloader/internal/pkg/options"
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

func TestValidateLeaderElectionTimings(t *testing.T) {
	origLease, origRenew, origRetry := options.LeaderElectionLeaseDuration, options.LeaderElectionRenewDeadline, options.LeaderElectionRetryPeriod
	defer func() {
		options.LeaderElectionLeaseDuration = origLease
		options.LeaderElectionRenewDeadline = origRenew
		options.LeaderElectionRetryPeriod = origRetry
	}()

	tests := []struct {
		name string
		// wantErr is the substring the validation error must contain; empty means the
		// timings must be accepted. Matching the message keeps each case pinned to the
		// rule it exercises rather than to any rule that happens to reject it.
		lease   time.Duration
		renew   time.Duration
		retry   time.Duration
		wantErr string
	}{
		{
			name:  "defaults are valid",
			lease: origLease,
			renew: origRenew,
			retry: origRetry,
		},
		{
			name:  "relaxed timings are valid",
			lease: 60 * time.Second,
			renew: 45 * time.Second,
			retry: 10 * time.Second,
		},
		{
			name:  "lease duration at the minimum whole second",
			lease: time.Second,
			renew: 800 * time.Millisecond,
			retry: 500 * time.Millisecond,
		},
		{
			name:  "lease duration at the largest representable value",
			lease: math.MaxInt32 * time.Second,
			renew: 10 * time.Second,
			retry: 2 * time.Second,
		},
		{
			// client-go persists this as 1s while the leader renews at 1.3s, so a
			// follower can acquire the lease while the incumbent still holds it.
			name:    "fractional lease duration truncating to a shorter lease",
			lease:   1500 * time.Millisecond,
			renew:   1300 * time.Millisecond,
			retry:   time.Second,
			wantErr: "must be a whole number of seconds",
		},
		{
			name:    "sub-second lease duration truncating to zero",
			lease:   900 * time.Millisecond,
			renew:   500 * time.Millisecond,
			retry:   100 * time.Millisecond,
			wantErr: "must be a whole number of seconds",
		},
		{
			name:    "lease duration beyond the Lease API int32 range",
			lease:   (math.MaxInt32 + 1) * time.Second,
			renew:   10 * time.Second,
			retry:   2 * time.Second,
			wantErr: "must not exceed",
		},
		{
			name:    "lease duration equal to renew deadline",
			lease:   10 * time.Second,
			renew:   10 * time.Second,
			retry:   2 * time.Second,
			wantErr: "must be greater than --leader-election-renew-deadline",
		},
		{
			name:    "lease duration shorter than renew deadline",
			lease:   5 * time.Second,
			renew:   10 * time.Second,
			retry:   2 * time.Second,
			wantErr: "must be greater than --leader-election-renew-deadline",
		},
		{
			name:    "renew deadline within jitter of retry period",
			lease:   15 * time.Second,
			renew:   10 * time.Second,
			retry:   9 * time.Second,
			wantErr: "jitter factor",
		},
		{
			name:    "zero lease duration",
			lease:   0,
			renew:   10 * time.Second,
			retry:   2 * time.Second,
			wantErr: "must be at least 1s",
		},
		{
			name:    "zero renew deadline",
			lease:   15 * time.Second,
			renew:   0,
			retry:   2 * time.Second,
			wantErr: "--leader-election-renew-deadline must be greater than zero",
		},
		{
			name:    "zero retry period",
			lease:   15 * time.Second,
			renew:   10 * time.Second,
			retry:   0,
			wantErr: "--leader-election-retry-period must be greater than zero",
		},
		{
			name:    "negative retry period",
			lease:   15 * time.Second,
			renew:   10 * time.Second,
			retry:   -1 * time.Second,
			wantErr: "--leader-election-retry-period must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options.LeaderElectionLeaseDuration = tt.lease
			options.LeaderElectionRenewDeadline = tt.renew
			options.LeaderElectionRetryPeriod = tt.retry

			err := validateLeaderElectionTimings()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.wantErr)
			}

			// The validation turns a client-go panic into a CLI error, so everything
			// client-go rejects must be rejected here too. The reverse does not hold:
			// client-go accepts fractional lease durations that break leader exclusivity.
			_, clientGoErr := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
				Lock: &resourcelock.LeaseLock{
					LeaseMeta:  v1.ObjectMeta{Name: "test", Namespace: "test"},
					LockConfig: resourcelock.ResourceLockConfig{Identity: "test"},
				},
				LeaseDuration: tt.lease,
				RenewDeadline: tt.renew,
				RetryPeriod:   tt.retry,
				Callbacks: leaderelection.LeaderCallbacks{
					OnStartedLeading: func(context.Context) {},
					OnStoppedLeading: func() {},
				},
			})
			if clientGoErr != nil {
				assert.Error(t, err, "client-go rejects these timings (%v), so validation must too", clientGoErr)
			}
		})
	}
}
