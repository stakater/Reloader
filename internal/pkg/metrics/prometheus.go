package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type Collectors struct {
	Reloaded *prometheus.CounterVec
}

func NewCollectors() Collectors {
	reloaded := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "reload_executed_total",
			Help:      "Counter of reloads executed by Reloader.",
		},
		[]string{"success"},
	)

	//set 0 as default value
	reloaded.With(prometheus.Labels{"success": "true"}).Add(0)
	reloaded.With(prometheus.Labels{"success": "false"}).Add(0)

	return Collectors{
		Reloaded: reloaded,
	}
}

func SetupPrometheusEndpoint() Collectors {
	collectors := NewCollectors()
	prometheus.MustRegister(collectors.Reloaded)
	http.Handle("/metrics", promhttp.Handler())

	return collectors
}
