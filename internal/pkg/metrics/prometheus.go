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
	QueueSize           *prometheus.GaugeVec
	Errors              *prometheus.CounterVec
	Requeues            *prometheus.CounterVec
	Dropped             *prometheus.CounterVec
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

	queueSize := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "reloader",
			Name:      "queue_size",
			Help:      "Gauge for the size of the work queue.",
		},
		[]string{
			"resource",
		},
	)

	errors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "errors_total",
			Help:      "Counter of errors encountered by Reloader.",
		},
		[]string{
			"error_type",
		},
	)
	requeues := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "requeues_total",
			Help:      "Counter of requeues encountered by Reloader.",
		},
		[]string{
			"resource",
		},
	)

	dropped := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "dropped_total",
			Help:      "Counter of dropped events by Reloader.",
		},
		[]string{
			"resource",
		},
	)

	return Collectors{
		Reloaded:            reloaded,
		ReloadedByNamespace: reloaded_by_namespace,
		QueueSize:           queueSize,
		Errors:              errors,
		Requeues:            requeues,
		Dropped:             dropped,
	}
}

func SetupPrometheusEndpoint() Collectors {
	collectors := NewCollectors()
	prometheus.MustRegister(collectors.Reloaded)

	if os.Getenv("METRICS_COUNT_BY_NAMESPACE") == "enabled" {
		prometheus.MustRegister(collectors.ReloadedByNamespace)
	}

	prometheus.MustRegister(collectors.QueueSize)
	prometheus.MustRegister(collectors.Errors)
	prometheus.MustRegister(collectors.Requeues)
	prometheus.MustRegister(collectors.Dropped)

	http.Handle("/metrics", promhttp.Handler())

	return collectors
}
