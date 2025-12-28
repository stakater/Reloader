package metrics

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Collectors holds Prometheus metrics collectors for Reloader.
type Collectors struct {
	Reloaded            *prometheus.CounterVec
	ReloadedByNamespace *prometheus.CounterVec
	countByNamespace    bool
}

// RecordReload records a reload event with the given success status and namespace.
func (c *Collectors) RecordReload(success bool, namespace string) {
	if c == nil {
		return
	}

	successLabel := "false"
	if success {
		successLabel = "true"
	}

	c.Reloaded.With(prometheus.Labels{"success": successLabel}).Inc()

	if c.countByNamespace {
		c.ReloadedByNamespace.With(prometheus.Labels{
			"success":   successLabel,
			"namespace": namespace,
		}).Inc()
	}
}

func NewCollectors() Collectors {
	reloaded := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "reload_executed_total",
			Help:      "Counter of reloads executed by Reloader.",
		},
		[]string{
			"success",
		},
	)

	reloaded.With(prometheus.Labels{"success": "true"}).Add(0)
	reloaded.With(prometheus.Labels{"success": "false"}).Add(0)

	reloadedByNamespace := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "reload_executed_total_by_namespace",
			Help:      "Counter of reloads executed by Reloader by namespace.",
		},
		[]string{
			"success",
			"namespace",
		},
	)
	return Collectors{
		Reloaded:            reloaded,
		ReloadedByNamespace: reloadedByNamespace,
		countByNamespace:    os.Getenv("METRICS_COUNT_BY_NAMESPACE") == "enabled",
	}
}

func SetupPrometheusEndpoint() Collectors {
	collectors := NewCollectors()
	prometheus.MustRegister(collectors.Reloaded)

	if os.Getenv("METRICS_COUNT_BY_NAMESPACE") == "enabled" {
		prometheus.MustRegister(collectors.ReloadedByNamespace)
	}

	http.Handle("/metrics", promhttp.Handler())

	return collectors
}
