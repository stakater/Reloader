package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"

	"github.com/stakater/Reloader/internal/pkg/handler"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/common"
)

// mockResourceHandler implements handler.ResourceHandler and handler.TimedHandler for testing.
type mockResourceHandler struct {
	handleErr   error
	handleCalls int
	enqueueTime time.Time
}

func (m *mockResourceHandler) Handle() error {
	m.handleCalls++
	return m.handleErr
}

func (m *mockResourceHandler) GetConfig() (common.Config, string) {
	return common.Config{
		ResourceName: "test-resource",
		Namespace:    "test-ns",
		Type:         "configmap",
		SHAValue:     "sha256:test",
	}, "test-resource"
}

func (m *mockResourceHandler) GetEnqueueTime() time.Time {
	return m.enqueueTime
}

// resetGlobalState resets global variables between tests
func resetGlobalState() {
	secretControllerInitialized = false
	configmapControllerInitialized = false
	selectedNamespacesCache = []string{}
}

// newTestController creates a controller for testing without starting informers
func newTestController(ignoredNamespaces []string, namespaceSelector string) *Controller {
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]())
	collectors := metrics.NewCollectors()

	return &Controller{
		queue:             queue,
		ignoredNamespaces: ignoredNamespaces,
		namespaceSelector: namespaceSelector,
		collectors:        collectors,
		resource:          "configmaps",
	}
}

func TestResourceInIgnoredNamespace(t *testing.T) {
	tests := []struct {
		name              string
		ignoredNamespaces []string
		resource          interface{}
		expected          bool
	}{
		{
			name:              "ConfigMap in ignored namespace",
			ignoredNamespaces: []string{"kube-system", "default"},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "kube-system",
				},
			},
			expected: true,
		},
		{
			name:              "ConfigMap not in ignored namespace",
			ignoredNamespaces: []string{"kube-system", "default"},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "my-namespace",
				},
			},
			expected: false,
		},
		{
			name:              "Secret in ignored namespace",
			ignoredNamespaces: []string{"kube-system"},
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "kube-system",
				},
			},
			expected: true,
		},
		{
			name:              "Secret not in ignored namespace",
			ignoredNamespaces: []string{"kube-system"},
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "my-namespace",
				},
			},
			expected: false,
		},
		{
			name:              "Empty ignored namespaces list",
			ignoredNamespaces: []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "any-namespace",
				},
			},
			expected: false,
		},
		{
			name:              "Unknown resource type",
			ignoredNamespaces: []string{"kube-system"},
			resource:          &v1.Pod{}, // Not a ConfigMap or Secret
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				c := newTestController(tt.ignoredNamespaces, "")
				result := c.resourceInIgnoredNamespace(tt.resource)
				assert.Equal(t, tt.expected, result)
			},
		)
	}
}

func TestResourceInSelectedNamespaces(t *testing.T) {
	tests := []struct {
		name              string
		namespaceSelector string
		cachedNamespaces  []string
		resource          interface{}
		expected          bool
	}{
		{
			name:              "No namespace selector - all namespaces allowed",
			namespaceSelector: "",
			cachedNamespaces:  []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "any-namespace",
				},
			},
			expected: true,
		},
		{
			name:              "ConfigMap in selected namespace",
			namespaceSelector: "env=prod",
			cachedNamespaces:  []string{"prod-ns", "staging-ns"},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "prod-ns",
				},
			},
			expected: true,
		},
		{
			name:              "ConfigMap not in selected namespace",
			namespaceSelector: "env=prod",
			cachedNamespaces:  []string{"prod-ns"},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "dev-ns",
				},
			},
			expected: false,
		},
		{
			name:              "Secret in selected namespace",
			namespaceSelector: "env=prod",
			cachedNamespaces:  []string{"prod-ns"},
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "prod-ns",
				},
			},
			expected: true,
		},
		{
			name:              "Secret not in selected namespace",
			namespaceSelector: "env=prod",
			cachedNamespaces:  []string{"prod-ns"},
			resource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "dev-ns",
				},
			},
			expected: false,
		},
		{
			name:              "Unknown resource type with selector",
			namespaceSelector: "env=prod",
			cachedNamespaces:  []string{"prod-ns"},
			resource:          &v1.Pod{},
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				resetGlobalState()
				selectedNamespacesCache = tt.cachedNamespaces

				c := newTestController([]string{}, tt.namespaceSelector)
				result := c.resourceInSelectedNamespaces(tt.resource)
				assert.Equal(t, tt.expected, result)
			},
		)
	}
}

func TestAddSelectedNamespaceToCache(t *testing.T) {
	resetGlobalState()

	c := newTestController([]string{}, "env=prod")

	// Add first namespace
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "namespace-1"},
	}
	c.addSelectedNamespaceToCache(ns1)
	assert.Contains(t, selectedNamespacesCache, "namespace-1")
	assert.Len(t, selectedNamespacesCache, 1)

	// Add second namespace
	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "namespace-2"},
	}
	c.addSelectedNamespaceToCache(ns2)
	assert.Contains(t, selectedNamespacesCache, "namespace-1")
	assert.Contains(t, selectedNamespacesCache, "namespace-2")
	assert.Len(t, selectedNamespacesCache, 2)
}

func TestRemoveSelectedNamespaceFromCache(t *testing.T) {
	tests := []struct {
		name              string
		initialCache      []string
		namespaceToRemove string
		expectedCache     []string
	}{
		{
			name:              "Remove existing namespace",
			initialCache:      []string{"ns-1", "ns-2", "ns-3"},
			namespaceToRemove: "ns-2",
			expectedCache:     []string{"ns-1", "ns-3"},
		},
		{
			name:              "Remove non-existing namespace",
			initialCache:      []string{"ns-1", "ns-2"},
			namespaceToRemove: "ns-3",
			expectedCache:     []string{"ns-1", "ns-2"},
		},
		{
			name:              "Remove from empty cache",
			initialCache:      []string{},
			namespaceToRemove: "ns-1",
			expectedCache:     []string{},
		},
		{
			name:              "Remove only namespace",
			initialCache:      []string{"ns-1"},
			namespaceToRemove: "ns-1",
			expectedCache:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				resetGlobalState()
				selectedNamespacesCache = tt.initialCache

				c := newTestController([]string{}, "env=prod")
				ns := v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: tt.namespaceToRemove},
				}
				c.removeSelectedNamespaceFromCache(ns)

				assert.Equal(t, tt.expectedCache, selectedNamespacesCache)
			},
		)
	}
}

func TestAddHandler(t *testing.T) {
	tests := []struct {
		name              string
		reloadOnCreate    string
		ignoredNamespaces []string
		resource          interface{}
		controllersInit   bool
		expectQueueItem   bool
	}{
		{
			name:              "Namespace resource - should not queue",
			reloadOnCreate:    "true",
			ignoredNamespaces: []string{},
			resource: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
			},
			controllersInit: true,
			expectQueueItem: false,
		},
		{
			name:              "ReloadOnCreate disabled",
			reloadOnCreate:    "false",
			ignoredNamespaces: []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			},
			controllersInit: true,
			expectQueueItem: false,
		},
		{
			name:              "ConfigMap in ignored namespace",
			reloadOnCreate:    "true",
			ignoredNamespaces: []string{"kube-system"},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "kube-system",
				},
			},
			controllersInit: true,
			expectQueueItem: false,
		},
		{
			name:              "Controllers not initialized",
			reloadOnCreate:    "true",
			ignoredNamespaces: []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			},
			controllersInit: false,
			expectQueueItem: false,
		},
		{
			name:              "Valid ConfigMap - should queue",
			reloadOnCreate:    "true",
			ignoredNamespaces: []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			},
			controllersInit: true,
			expectQueueItem: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				resetGlobalState()
				options.ReloadOnCreate = tt.reloadOnCreate
				secretControllerInitialized = tt.controllersInit
				configmapControllerInitialized = tt.controllersInit

				c := newTestController(tt.ignoredNamespaces, "")
				c.Add(tt.resource)

				if tt.expectQueueItem {
					assert.Equal(t, 1, c.queue.Len(), "Expected queue to have 1 item")
				} else {
					assert.Equal(t, 0, c.queue.Len(), "Expected queue to be empty")
				}
			},
		)
	}
}

func TestUpdateHandler(t *testing.T) {
	tests := []struct {
		name              string
		ignoredNamespaces []string
		namespaceSelector string
		cachedNamespaces  []string
		oldResource       interface{}
		newResource       interface{}
		expectQueueItem   bool
	}{
		{
			name:              "Namespace resource - should not queue",
			ignoredNamespaces: []string{},
			oldResource: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
			},
			newResource: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
			},
			expectQueueItem: false,
		},
		{
			name:              "ConfigMap in ignored namespace",
			ignoredNamespaces: []string{"kube-system"},
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "kube-system",
				},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "kube-system",
				},
			},
			expectQueueItem: false,
		},
		{
			name:              "ConfigMap not in selected namespace",
			ignoredNamespaces: []string{},
			namespaceSelector: "env=prod",
			cachedNamespaces:  []string{"prod-ns"},
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "dev-ns",
				},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "dev-ns",
				},
			},
			expectQueueItem: false,
		},
		{
			name:              "Valid ConfigMap update - should queue",
			ignoredNamespaces: []string{},
			oldResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "old-value"},
			},
			newResource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "new-value"},
			},
			expectQueueItem: true,
		},
		{
			name:              "Valid Secret update - should queue",
			ignoredNamespaces: []string{},
			oldResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
			newResource: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
			expectQueueItem: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				resetGlobalState()
				if tt.cachedNamespaces != nil {
					selectedNamespacesCache = tt.cachedNamespaces
				}

				c := newTestController(tt.ignoredNamespaces, tt.namespaceSelector)
				c.Update(tt.oldResource, tt.newResource)

				if tt.expectQueueItem {
					assert.Equal(t, 1, c.queue.Len(), "Expected queue to have 1 item")
					// Verify the queued item is the correct type
					item, _ := c.queue.Get()
					_, ok := item.(handler.ResourceUpdatedHandler)
					assert.True(t, ok, "Expected ResourceUpdatedHandler in queue")
					c.queue.Done(item)
				} else {
					assert.Equal(t, 0, c.queue.Len(), "Expected queue to be empty")
				}
			},
		)
	}
}

func TestDeleteHandler(t *testing.T) {
	tests := []struct {
		name              string
		reloadOnDelete    string
		ignoredNamespaces []string
		resource          interface{}
		controllersInit   bool
		expectQueueItem   bool
	}{
		{
			name:              "ReloadOnDelete disabled",
			reloadOnDelete:    "false",
			ignoredNamespaces: []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			},
			controllersInit: true,
			expectQueueItem: false,
		},
		{
			name:              "ConfigMap in ignored namespace",
			reloadOnDelete:    "true",
			ignoredNamespaces: []string{"kube-system"},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "kube-system",
				},
			},
			controllersInit: true,
			expectQueueItem: false,
		},
		{
			name:              "Controllers not initialized",
			reloadOnDelete:    "true",
			ignoredNamespaces: []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			},
			controllersInit: false,
			expectQueueItem: false,
		},
		{
			name:              "Valid ConfigMap delete - should queue",
			reloadOnDelete:    "true",
			ignoredNamespaces: []string{},
			resource: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			},
			controllersInit: true,
			expectQueueItem: true,
		},
		{
			name:              "Namespace delete - updates cache",
			reloadOnDelete:    "false", // Disable to test cache update only
			ignoredNamespaces: []string{},
			resource: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
			},
			controllersInit: true,
			expectQueueItem: false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				resetGlobalState()
				options.ReloadOnDelete = tt.reloadOnDelete
				secretControllerInitialized = tt.controllersInit
				configmapControllerInitialized = tt.controllersInit

				c := newTestController(tt.ignoredNamespaces, "")
				c.Delete(tt.resource)

				if tt.expectQueueItem {
					assert.Equal(t, 1, c.queue.Len(), "Expected queue to have 1 item")
					// Verify the queued item is the correct type
					item, _ := c.queue.Get()
					_, ok := item.(handler.ResourceDeleteHandler)
					assert.True(t, ok, "Expected ResourceDeleteHandler in queue")
					c.queue.Done(item)
				} else {
					assert.Equal(t, 0, c.queue.Len(), "Expected queue to be empty")
				}
			},
		)
	}
}

func TestHandleErr(t *testing.T) {
	t.Run(
		"No error - should forget key", func(t *testing.T) {
			resetGlobalState()
			c := newTestController([]string{}, "")

			key := "test-key"
			// Add key to queue first
			c.queue.Add(key)
			item, _ := c.queue.Get()

			// Handle with no error
			c.handleErr(nil, item)
			c.queue.Done(item)

			// Key should be forgotten (NumRequeues should be 0)
			assert.Equal(t, 0, c.queue.NumRequeues(key))
		},
	)

	t.Run(
		"Error at max retries - should drop key", func(t *testing.T) {
			resetGlobalState()
			c := newTestController([]string{}, "")

			key := "test-key-max"

			// Simulate 5 previous failures (max retries)
			for range 5 {
				c.queue.AddRateLimited(key)
			}

			// After max retries, handleErr should forget the key
			c.handleErr(assert.AnError, key)

			// Key should be forgotten
			assert.Equal(t, 0, c.queue.NumRequeues(key))
		},
	)
}

func TestAddHandlerWithNamespaceEvent(t *testing.T) {
	resetGlobalState()

	c := newTestController([]string{}, "env=prod")

	// When a namespace is added, it should be cached
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "new-namespace"},
	}

	c.Add(ns)

	assert.Contains(t, selectedNamespacesCache, "new-namespace")
	assert.Equal(t, 0, c.queue.Len(), "Namespace add should not queue anything")
}

func TestDeleteHandlerWithNamespaceEvent(t *testing.T) {
	resetGlobalState()
	selectedNamespacesCache = []string{"ns-1", "ns-to-delete", "ns-2"}

	c := newTestController([]string{}, "env=prod")
	options.ReloadOnDelete = "true"
	secretControllerInitialized = true
	configmapControllerInitialized = true

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-to-delete"},
	}

	c.Delete(ns)

	assert.NotContains(t, selectedNamespacesCache, "ns-to-delete")
	assert.Contains(t, selectedNamespacesCache, "ns-1")
	assert.Contains(t, selectedNamespacesCache, "ns-2")
	assert.Equal(t, 0, c.queue.Len(), "Namespace delete should not queue anything")
}

func TestProcessNextItem(t *testing.T) {
	tests := []struct {
		name           string
		handler        *mockResourceHandler
		expectContinue bool
		expectCalls    int
	}{
		{
			name: "Successful handler execution",
			handler: &mockResourceHandler{
				handleErr:   nil,
				enqueueTime: time.Now().Add(-10 * time.Millisecond),
			},
			expectContinue: true,
			expectCalls:    1,
		},
		{
			name: "Handler returns error",
			handler: &mockResourceHandler{
				handleErr:   errors.New("test error"),
				enqueueTime: time.Now().Add(-10 * time.Millisecond),
			},
			expectContinue: true,
			expectCalls:    1,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				resetGlobalState()
				c := newTestController([]string{}, "")

				c.queue.Add(tt.handler)

				result := c.processNextItem()

				assert.Equal(t, tt.expectContinue, result)
				assert.Equal(t, tt.expectCalls, tt.handler.handleCalls)
			},
		)
	}
}

func TestProcessNextItemQueueShutdown(t *testing.T) {
	resetGlobalState()
	c := newTestController([]string{}, "")

	c.queue.ShutDown()

	result := c.processNextItem()
	assert.False(t, result, "Should return false when queue is shutdown")
}
