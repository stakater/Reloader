package metrics

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/tools/metrics"
)

// clientGoRequestMetrics implements metrics.LatencyMetric and metrics.ResultMetric
// to expose client-go's rest_client_requests_total metric
type clientGoRequestMetrics struct {
	requestCounter *prometheus.CounterVec
	requestLatency *prometheus.HistogramVec
}

func (m *clientGoRequestMetrics) Increment(ctx context.Context, code string, method string, host string) {
	m.requestCounter.WithLabelValues(code, method, host).Inc()
}

func (m *clientGoRequestMetrics) Observe(ctx context.Context, verb string, u url.URL, latency time.Duration) {
	m.requestLatency.WithLabelValues(verb, u.Host).Observe(latency.Seconds())
}

var clientGoMetrics = &clientGoRequestMetrics{
	requestCounter: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rest_client_requests_total",
			Help: "Number of HTTP requests, partitioned by status code, method, and host.",
		},
		[]string{"code", "method", "host"},
	),
	requestLatency: prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_request_duration_seconds",
			Help:    "Request latency in seconds. Broken down by verb and host.",
			Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30},
		},
		[]string{"verb", "host"},
	),
}

func init() {
	// Register the metrics collectors
	prometheus.MustRegister(clientGoMetrics.requestCounter)
	prometheus.MustRegister(clientGoMetrics.requestLatency)

	// Register our metrics implementation with client-go
	metrics.RequestResult = clientGoMetrics
	metrics.RequestLatency = clientGoMetrics
}

// Collectors holds all Prometheus metrics collectors for Reloader.
type Collectors struct {
	Reloaded            *prometheus.CounterVec
	ReloadedByNamespace *prometheus.CounterVec
	countByNamespace    bool

	ReconcileTotal    *prometheus.CounterVec   // Total reconcile calls by result
	ReconcileDuration *prometheus.HistogramVec // Time spent in reconcile/handler
	ActionTotal       *prometheus.CounterVec   // Total actions by workload kind and result
	ActionLatency     *prometheus.HistogramVec // Time from event to action applied
	SkippedTotal      *prometheus.CounterVec   // Skipped operations by reason
	QueueDepth        prometheus.Gauge         // Current queue depth
	QueueAdds         prometheus.Counter       // Total items added to queue
	QueueLatency      *prometheus.HistogramVec // Time spent in queue
	ErrorsTotal       *prometheus.CounterVec   // Errors by type
	RetriesTotal      prometheus.Counter       // Total retries
	EventsReceived    *prometheus.CounterVec   // Events received by type (add/update/delete)
	EventsProcessed   *prometheus.CounterVec   // Events processed by type and result
	WorkloadsScanned  *prometheus.CounterVec   // Workloads scanned by kind
	WorkloadsMatched  *prometheus.CounterVec   // Workloads matched for reload by kind
}

// RecordReload records a reload event with the given success status and namespace.
// Preserved for backward compatibility.
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

// RecordReconcile records a reconcile/handler invocation.
func (c *Collectors) RecordReconcile(result string, duration time.Duration) {
	if c == nil {
		return
	}
	c.ReconcileTotal.With(prometheus.Labels{"result": result}).Inc()
	c.ReconcileDuration.With(prometheus.Labels{"result": result}).Observe(duration.Seconds())
}

// RecordAction records a reload action on a workload.
func (c *Collectors) RecordAction(workloadKind string, result string, latency time.Duration) {
	if c == nil {
		return
	}
	c.ActionTotal.With(prometheus.Labels{"workload_kind": workloadKind, "result": result}).Inc()
	c.ActionLatency.With(prometheus.Labels{"workload_kind": workloadKind}).Observe(latency.Seconds())
}

// RecordSkipped records a skipped operation with reason.
func (c *Collectors) RecordSkipped(reason string) {
	if c == nil {
		return
	}
	c.SkippedTotal.With(prometheus.Labels{"reason": reason}).Inc()
}

// RecordQueueAdd records an item being added to the queue.
func (c *Collectors) RecordQueueAdd() {
	if c == nil {
		return
	}
	c.QueueAdds.Inc()
}

// SetQueueDepth sets the current queue depth.
func (c *Collectors) SetQueueDepth(depth int) {
	if c == nil {
		return
	}
	c.QueueDepth.Set(float64(depth))
}

// RecordQueueLatency records how long an item spent in the queue.
func (c *Collectors) RecordQueueLatency(latency time.Duration) {
	if c == nil {
		return
	}
	c.QueueLatency.With(prometheus.Labels{}).Observe(latency.Seconds())
}

// RecordError records an error by type.
func (c *Collectors) RecordError(errorType string) {
	if c == nil {
		return
	}
	c.ErrorsTotal.With(prometheus.Labels{"type": errorType}).Inc()
}

// RecordRetry records a retry attempt.
func (c *Collectors) RecordRetry() {
	if c == nil {
		return
	}
	c.RetriesTotal.Inc()
}

// RecordEventReceived records an event being received.
func (c *Collectors) RecordEventReceived(eventType string, resourceType string) {
	if c == nil {
		return
	}
	c.EventsReceived.With(prometheus.Labels{"event_type": eventType, "resource_type": resourceType}).Inc()
}

// RecordEventProcessed records an event being processed.
func (c *Collectors) RecordEventProcessed(eventType string, resourceType string, result string) {
	if c == nil {
		return
	}
	c.EventsProcessed.With(prometheus.Labels{"event_type": eventType, "resource_type": resourceType, "result": result}).Inc()
}

// RecordWorkloadsScanned records workloads scanned during a reconcile.
func (c *Collectors) RecordWorkloadsScanned(kind string, count int) {
	if c == nil {
		return
	}
	c.WorkloadsScanned.With(prometheus.Labels{"kind": kind}).Add(float64(count))
}

// RecordWorkloadsMatched records workloads matched for reload.
func (c *Collectors) RecordWorkloadsMatched(kind string, count int) {
	if c == nil {
		return
	}
	c.WorkloadsMatched.With(prometheus.Labels{"kind": kind}).Add(float64(count))
}

func NewCollectors() Collectors {
	// Existing metrics (preserved)
	reloaded := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "reload_executed_total",
			Help:      "Counter of reloads executed by Reloader.",
		},
		[]string{"success"},
	)
	reloaded.With(prometheus.Labels{"success": "true"}).Add(0)
	reloaded.With(prometheus.Labels{"success": "false"}).Add(0)

	reloadedByNamespace := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "reload_executed_total_by_namespace",
			Help:      "Counter of reloads executed by Reloader by namespace.",
		},
		[]string{"success", "namespace"},
	)

	reconcileTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "reconcile_total",
			Help:      "Total number of reconcile/handler invocations by result.",
		},
		[]string{"result"},
	)

	reconcileDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "reloader",
			Name:      "reconcile_duration_seconds",
			Help:      "Time spent in reconcile/handler in seconds.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"result"},
	)

	actionTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "action_total",
			Help:      "Total number of reload actions by workload kind and result.",
		},
		[]string{"workload_kind", "result"},
	)

	actionLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "reloader",
			Name:      "action_latency_seconds",
			Help:      "Time from event received to action applied in seconds.",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"workload_kind"},
	)

	skippedTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "skipped_total",
			Help:      "Total number of skipped operations by reason.",
		},
		[]string{"reason"},
	)

	queueDepth := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "reloader",
			Name:      "workqueue_depth",
			Help:      "Current depth of the work queue.",
		},
	)

	queueAdds := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "workqueue_adds_total",
			Help:      "Total number of items added to the work queue.",
		},
	)

	queueLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "reloader",
			Name:      "workqueue_latency_seconds",
			Help:      "Time spent in the work queue in seconds.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{},
	)

	errorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "errors_total",
			Help:      "Total number of errors by type.",
		},
		[]string{"type"},
	)

	retriesTotal := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "retries_total",
			Help:      "Total number of retry attempts.",
		},
	)

	eventsReceived := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "events_received_total",
			Help:      "Total number of events received by type and resource.",
		},
		[]string{"event_type", "resource_type"},
	)

	eventsProcessed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "events_processed_total",
			Help:      "Total number of events processed by type, resource, and result.",
		},
		[]string{"event_type", "resource_type", "result"},
	)

	workloadsScanned := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "workloads_scanned_total",
			Help:      "Total number of workloads scanned by kind.",
		},
		[]string{"kind"},
	)

	workloadsMatched := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "reloader",
			Name:      "workloads_matched_total",
			Help:      "Total number of workloads matched for reload by kind.",
		},
		[]string{"kind"},
	)

	return Collectors{
		Reloaded:            reloaded,
		ReloadedByNamespace: reloadedByNamespace,
		countByNamespace:    os.Getenv("METRICS_COUNT_BY_NAMESPACE") == "enabled",

		ReconcileTotal:    reconcileTotal,
		ReconcileDuration: reconcileDuration,
		ActionTotal:       actionTotal,
		ActionLatency:     actionLatency,
		SkippedTotal:      skippedTotal,
		QueueDepth:        queueDepth,
		QueueAdds:         queueAdds,
		QueueLatency:      queueLatency,
		ErrorsTotal:       errorsTotal,
		RetriesTotal:      retriesTotal,
		EventsReceived:    eventsReceived,
		EventsProcessed:   eventsProcessed,
		WorkloadsScanned:  workloadsScanned,
		WorkloadsMatched:  workloadsMatched,
	}
}

func SetupPrometheusEndpoint() Collectors {
	collectors := NewCollectors()

	prometheus.MustRegister(collectors.Reloaded)
	prometheus.MustRegister(collectors.ReconcileTotal)
	prometheus.MustRegister(collectors.ReconcileDuration)
	prometheus.MustRegister(collectors.ActionTotal)
	prometheus.MustRegister(collectors.ActionLatency)
	prometheus.MustRegister(collectors.SkippedTotal)
	prometheus.MustRegister(collectors.QueueDepth)
	prometheus.MustRegister(collectors.QueueAdds)
	prometheus.MustRegister(collectors.QueueLatency)
	prometheus.MustRegister(collectors.ErrorsTotal)
	prometheus.MustRegister(collectors.RetriesTotal)
	prometheus.MustRegister(collectors.EventsReceived)
	prometheus.MustRegister(collectors.EventsProcessed)
	prometheus.MustRegister(collectors.WorkloadsScanned)
	prometheus.MustRegister(collectors.WorkloadsMatched)

	if os.Getenv("METRICS_COUNT_BY_NAMESPACE") == "enabled" {
		prometheus.MustRegister(collectors.ReloadedByNamespace)
	}

	http.Handle("/metrics", promhttp.Handler())

	return collectors
}
