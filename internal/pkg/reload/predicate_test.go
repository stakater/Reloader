package reload

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestNamespaceFilterPredicate_Create(t *testing.T) {
	tests := []struct {
		name              string
		ignoredNamespaces []string
		eventNamespace    string
		wantAllow         bool
	}{
		{
			name:              "allow non-ignored namespace",
			ignoredNamespaces: []string{"kube-system"},
			eventNamespace:    "default",
			wantAllow:         true,
		},
		{
			name:              "block ignored namespace",
			ignoredNamespaces: []string{"kube-system"},
			eventNamespace:    "kube-system",
			wantAllow:         false,
		},
		{
			name:              "allow when no namespaces ignored",
			ignoredNamespaces: []string{},
			eventNamespace:    "kube-system",
			wantAllow:         true,
		},
		{
			name:              "block multiple ignored namespaces",
			ignoredNamespaces: []string{"kube-system", "kube-public", "test-ns"},
			eventNamespace:    "test-ns",
			wantAllow:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefault()
			cfg.IgnoredNamespaces = tt.ignoredNamespaces
			predicate := NamespaceFilterPredicate(cfg)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: tt.eventNamespace,
				},
			}

			e := event.CreateEvent{Object: cm}
			got := predicate.Create(e)

			if got != tt.wantAllow {
				t.Errorf("Create() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}

func TestNamespaceFilterPredicate_Update(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}
	predicate := NamespaceFilterPredicate(cfg)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	e := event.UpdateEvent{ObjectNew: cm}
	if !predicate.Update(e) {
		t.Error("Update() should allow non-ignored namespace")
	}

	cm.Namespace = "kube-system"
	e = event.UpdateEvent{ObjectNew: cm}
	if predicate.Update(e) {
		t.Error("Update() should block ignored namespace")
	}
}

func TestNamespaceFilterPredicate_Delete(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}
	predicate := NamespaceFilterPredicate(cfg)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	e := event.DeleteEvent{Object: cm}
	if !predicate.Delete(e) {
		t.Error("Delete() should allow non-ignored namespace")
	}
}

func TestNamespaceFilterPredicate_Generic(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}
	predicate := NamespaceFilterPredicate(cfg)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	e := event.GenericEvent{Object: cm}
	if !predicate.Generic(e) {
		t.Error("Generic() should allow non-ignored namespace")
	}
}

func TestLabelSelectorPredicate_Create(t *testing.T) {
	tests := []struct {
		name          string
		selector      string
		objectLabels  map[string]string
		wantAllow     bool
	}{
		{
			name:         "match single label",
			selector:     "app=reloader",
			objectLabels: map[string]string{"app": "reloader"},
			wantAllow:    true,
		},
		{
			name:         "no match single label",
			selector:     "app=reloader",
			objectLabels: map[string]string{"app": "other"},
			wantAllow:    false,
		},
		{
			name:         "match multiple labels",
			selector:     "app=reloader,env=prod",
			objectLabels: map[string]string{"app": "reloader", "env": "prod", "extra": "value"},
			wantAllow:    true,
		},
		{
			name:         "partial match fails",
			selector:     "app=reloader,env=prod",
			objectLabels: map[string]string{"app": "reloader"},
			wantAllow:    false,
		},
		{
			name:         "empty labels no match",
			selector:     "app=reloader",
			objectLabels: map[string]string{},
			wantAllow:    false,
		},
		{
			name:         "nil labels no match",
			selector:     "app=reloader",
			objectLabels: nil,
			wantAllow:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefault()
			selector, err := labels.Parse(tt.selector)
			if err != nil {
				t.Fatalf("Failed to parse selector: %v", err)
			}
			cfg.ResourceSelectors = []labels.Selector{selector}
			predicate := LabelSelectorPredicate(cfg)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
					Labels:    tt.objectLabels,
				},
			}

			e := event.CreateEvent{Object: cm}
			got := predicate.Create(e)

			if got != tt.wantAllow {
				t.Errorf("Create() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}

func TestLabelSelectorPredicate_NoSelectors(t *testing.T) {
	cfg := config.NewDefault()
	// No selectors configured
	predicate := LabelSelectorPredicate(cfg)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels:    map[string]string{"any": "label"},
		},
	}

	e := event.CreateEvent{Object: cm}
	if !predicate.Create(e) {
		t.Error("Create() should allow all when no selectors configured")
	}
}

func TestLabelSelectorPredicate_MultipleSelectors(t *testing.T) {
	cfg := config.NewDefault()
	selector1, _ := labels.Parse("app=reloader")
	selector2, _ := labels.Parse("type=config")
	cfg.ResourceSelectors = []labels.Selector{selector1, selector2}
	predicate := LabelSelectorPredicate(cfg)

	tests := []struct {
		name      string
		labels    map[string]string
		wantAllow bool
	}{
		{
			name:      "matches first selector",
			labels:    map[string]string{"app": "reloader"},
			wantAllow: true,
		},
		{
			name:      "matches second selector",
			labels:    map[string]string{"type": "config"},
			wantAllow: true,
		},
		{
			name:      "matches both selectors",
			labels:    map[string]string{"app": "reloader", "type": "config"},
			wantAllow: true,
		},
		{
			name:      "matches neither selector",
			labels:    map[string]string{"other": "value"},
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
					Labels:    tt.labels,
				},
			}

			e := event.CreateEvent{Object: cm}
			got := predicate.Create(e)

			if got != tt.wantAllow {
				t.Errorf("Create() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}

func TestLabelSelectorPredicate_Update(t *testing.T) {
	cfg := config.NewDefault()
	selector, _ := labels.Parse("app=reloader")
	cfg.ResourceSelectors = []labels.Selector{selector}
	predicate := LabelSelectorPredicate(cfg)

	cmMatching := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels:    map[string]string{"app": "reloader"},
		},
	}

	cmNotMatching := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels:    map[string]string{"app": "other"},
		},
	}

	e := event.UpdateEvent{ObjectNew: cmMatching}
	if !predicate.Update(e) {
		t.Error("Update() should allow matching labels")
	}

	e = event.UpdateEvent{ObjectNew: cmNotMatching}
	if predicate.Update(e) {
		t.Error("Update() should block non-matching labels")
	}
}

func TestLabelSelectorPredicate_Delete(t *testing.T) {
	cfg := config.NewDefault()
	selector, _ := labels.Parse("app=reloader")
	cfg.ResourceSelectors = []labels.Selector{selector}
	predicate := LabelSelectorPredicate(cfg)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels:    map[string]string{"app": "reloader"},
		},
	}

	e := event.DeleteEvent{Object: cm}
	if !predicate.Delete(e) {
		t.Error("Delete() should allow matching labels")
	}
}

func TestLabelSelectorPredicate_Generic(t *testing.T) {
	cfg := config.NewDefault()
	selector, _ := labels.Parse("app=reloader")
	cfg.ResourceSelectors = []labels.Selector{selector}
	predicate := LabelSelectorPredicate(cfg)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels:    map[string]string{"app": "reloader"},
		},
	}

	e := event.GenericEvent{Object: cm}
	if !predicate.Generic(e) {
		t.Error("Generic() should allow matching labels")
	}
}

func TestCombinedFiltering(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}
	selector, _ := labels.Parse("managed=true")
	cfg.ResourceSelectors = []labels.Selector{selector}

	nsPredicate := NamespaceFilterPredicate(cfg)
	labelPredicate := LabelSelectorPredicate(cfg)

	tests := []struct {
		name        string
		namespace   string
		labels      map[string]string
		wantNSAllow bool
		wantLabelAllow bool
	}{
		{
			name:        "allowed namespace and matching labels",
			namespace:   "default",
			labels:      map[string]string{"managed": "true"},
			wantNSAllow: true,
			wantLabelAllow: true,
		},
		{
			name:        "allowed namespace but non-matching labels",
			namespace:   "default",
			labels:      map[string]string{"managed": "false"},
			wantNSAllow: true,
			wantLabelAllow: false,
		},
		{
			name:        "ignored namespace with matching labels",
			namespace:   "kube-system",
			labels:      map[string]string{"managed": "true"},
			wantNSAllow: false,
			wantLabelAllow: true,
		},
		{
			name:        "ignored namespace and non-matching labels",
			namespace:   "kube-system",
			labels:      map[string]string{"managed": "false"},
			wantNSAllow: false,
			wantLabelAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: tt.namespace,
					Labels:    tt.labels,
				},
			}

			e := event.CreateEvent{Object: cm}

			gotNS := nsPredicate.Create(e)
			if gotNS != tt.wantNSAllow {
				t.Errorf("Namespace predicate Create() = %v, want %v", gotNS, tt.wantNSAllow)
			}

			gotLabel := labelPredicate.Create(e)
			if gotLabel != tt.wantLabelAllow {
				t.Errorf("Label predicate Create() = %v, want %v", gotLabel, tt.wantLabelAllow)
			}

			// Both must be true for the event to pass through
			combinedAllow := gotNS && gotLabel
			expectedCombined := tt.wantNSAllow && tt.wantLabelAllow
			if combinedAllow != expectedCombined {
				t.Errorf("Combined allow = %v, want %v", combinedAllow, expectedCombined)
			}
		})
	}
}

func TestFilteringWithSecrets(t *testing.T) {
	cfg := config.NewDefault()
	cfg.IgnoredNamespaces = []string{"kube-system"}
	nsPredicate := NamespaceFilterPredicate(cfg)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	e := event.CreateEvent{Object: secret}
	if !nsPredicate.Create(e) {
		t.Error("Should allow secret in non-ignored namespace")
	}

	secret.Namespace = "kube-system"
	e = event.CreateEvent{Object: secret}
	if nsPredicate.Create(e) {
		t.Error("Should block secret in ignored namespace")
	}
}

func TestExistsLabelSelector(t *testing.T) {
	cfg := config.NewDefault()
	// Selector that checks if label exists (any value)
	selector, _ := labels.Parse("managed")
	cfg.ResourceSelectors = []labels.Selector{selector}
	predicate := LabelSelectorPredicate(cfg)

	tests := []struct {
		name      string
		labels    map[string]string
		wantAllow bool
	}{
		{
			name:      "label exists with value true",
			labels:    map[string]string{"managed": "true"},
			wantAllow: true,
		},
		{
			name:      "label exists with value false",
			labels:    map[string]string{"managed": "false"},
			wantAllow: true,
		},
		{
			name:      "label exists with empty value",
			labels:    map[string]string{"managed": ""},
			wantAllow: true,
		},
		{
			name:      "label does not exist",
			labels:    map[string]string{"other": "value"},
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
					Labels:    tt.labels,
				},
			}

			e := event.CreateEvent{Object: cm}
			got := predicate.Create(e)

			if got != tt.wantAllow {
				t.Errorf("Create() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}
