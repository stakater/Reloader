package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/common"
)

func TestSendUpgradeWebhookReturnsErrorOnNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	err := sendUpgradeWebhook(common.Config{ResourceName: "my-cm", Namespace: "default"}, server.URL)
	assert.Error(t, err)
}

func TestSendUpgradeWebhookSucceedsOn2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	err := sendUpgradeWebhook(common.Config{ResourceName: "my-cm", Namespace: "default"}, server.URL)
	assert.NoError(t, err)
}

func TestSendUpgradeWebhookRetriesOnlyOn5xxAnd429(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantRetry bool
	}{
		{name: "500", status: http.StatusInternalServerError, wantRetry: true},
		{name: "503", status: http.StatusServiceUnavailable, wantRetry: true},
		{name: "429", status: http.StatusTooManyRequests, wantRetry: true},
		{name: "400", status: http.StatusBadRequest, wantRetry: false},
		{name: "401", status: http.StatusUnauthorized, wantRetry: false},
		{name: "404", status: http.StatusNotFound, wantRetry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			err := sendUpgradeWebhook(common.Config{ResourceName: "my-cm", Namespace: "default"}, server.URL)
			assert.Error(t, err)
			assert.Equal(t, !tt.wantRetry, errors.Is(err, ErrNotRetryable))
		})
	}
}

func TestResourceCreatedHandlerWebhookStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantRetry bool
	}{
		{name: "4xx is not requeued", status: http.StatusUnauthorized, wantRetry: false},
		{name: "429 is requeued", status: http.StatusTooManyRequests, wantRetry: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			oldURL := options.WebhookUrl
			options.WebhookUrl = server.URL
			defer func() { options.WebhookUrl = oldURL }()

			collectors := metrics.NewCollectors()
			handler := ResourceCreatedHandler{
				Resource: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
					Data:       map[string]string{"key": "value"},
				},
				Collectors: collectors,
			}

			err := handler.Handle()
			assert.Error(t, err)
			assert.Equal(t, !tt.wantRetry, errors.Is(err, ErrNotRetryable))
			assert.Equal(t, float64(1), reconcileCount(t, collectors, "error"))
			assert.Equal(t, float64(0), reconcileCount(t, collectors, "success"))
		})
	}
}

func reconcileCount(t *testing.T, collectors metrics.Collectors, result string) float64 {
	t.Helper()
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.ReconcileTotal)
	families, err := registry.Gather()
	assert.NoError(t, err)
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "result" && label.GetValue() == result {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}
