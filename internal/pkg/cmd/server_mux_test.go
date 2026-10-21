package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stakater/Reloader/internal/pkg/leadership"
	"github.com/stakater/Reloader/internal/pkg/metrics"
)

func TestMainServerMuxEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	metrics.SetupPrometheusEndpointWithMux(mux)
	leadership.SetupLivenessEndpointWithMux(mux)

	// Liveness endpoint should be served
	liveReq := httptest.NewRequest(http.MethodGet, "/live", nil)
	liveRec := httptest.NewRecorder()
	mux.ServeHTTP(liveRec, liveReq)
	assert.Equal(t, http.StatusOK, liveRec.Code)
	assert.Equal(t, "alive", liveRec.Body.String())

	// Metrics endpoint should be served
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	mux.ServeHTTP(metricsRec, metricsReq)
	assert.Equal(t, http.StatusOK, metricsRec.Code)

	// pprof and expvar endpoints should NOT be served on the main metrics/liveness mux
	disallowedEndpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
		"/debug/pprof/goroutine",
		"/debug/pprof/heap",
		"/debug/vars",
	}

	for _, endpoint := range disallowedEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNotFound, rec.Code, "endpoint %s should not be registered on main mux", endpoint)
		})
	}
}

func TestPProfServerMuxEndpoints(t *testing.T) {
	mux := newPProfServeMux()

	// pprof endpoints should be served on pprof mux
	allowedEndpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/goroutine",
		"/debug/pprof/heap",
	}

	for _, endpoint := range allowedEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "pprof endpoint %s should be served on pprof mux", endpoint)
		})
	}

	// Main metrics and liveness endpoints should NOT be served on the pprof mux
	disallowedEndpoints := []string{
		"/metrics",
		"/live",
	}

	for _, endpoint := range disallowedEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNotFound, rec.Code, "endpoint %s should not be registered on pprof mux", endpoint)
		})
	}
}
