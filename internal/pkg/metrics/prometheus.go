package metrics

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Collectors struct {
	Reloaded            *prometheus.CounterVec
	ReloadedByNamespace *prometheus.CounterVec
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

	//set 0 as default value
	reloaded.With(prometheus.Labels{"success": "true"}).Add(0)
	reloaded.With(prometheus.Labels{"success": "false"}).Add(0)

	reloaded_by_namespace := prometheus.NewCounterVec(
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
		ReloadedByNamespace: reloaded_by_namespace,
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
