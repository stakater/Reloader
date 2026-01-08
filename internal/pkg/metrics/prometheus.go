package metrics

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Collectors holds all Prometheus metrics collectors for Reloader.
type Collectors struct {
	Reloaded            *prometheus.CounterVec
	ReloadedByNamespace *prometheus.CounterVec
	countByNamespace    bool

	// === Comprehensive metrics for load testing ===

	// Reconcile/Handler metrics
	ReconcileTotal    *prometheus.CounterVec   // Total reconcile calls by result
	ReconcileDuration *prometheus.HistogramVec // Time spent in reconcile/handler

	// Action metrics
	ActionTotal   *prometheus.CounterVec   // Total actions by workload kind and result
	ActionLatency *prometheus.HistogramVec // Time from event to action applied

	// Skip metrics
	SkippedTotal *prometheus.CounterVec // Skipped operations by reason

	// Queue metrics (controller-runtime exposes some automatically, but we add custom ones)
	QueueDepth   prometheus.Gauge         // Current queue depth
	QueueAdds    prometheus.Counter       // Total items added to queue
	QueueLatency *prometheus.HistogramVec // Time spent in queue

	// Error and retry metrics
	ErrorsTotal  *prometheus.CounterVec // Errors by type
	RetriesTotal prometheus.Counter     // Total retries

	// Event processing metrics
	EventsReceived  *prometheus.CounterVec // Events received by type (add/update/delete)
	EventsProcessed *prometheus.CounterVec // Events processed by type and result

	// Resource discovery metrics
	WorkloadsScanned *prometheus.CounterVec // Workloads scanned by kind
	WorkloadsMatched *prometheus.CounterVec // Workloads matched for reload by kind
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
		c.ReloadedByNamespace.With(
			prometheus.Labels{
				"success":   successLabel,
				"namespace": namespace,
			},
		).Inc()
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

	// === Comprehensive metrics ===

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

	ctrlmetrics.Registry.MustRegister(collectors.Reloaded)
	ctrlmetrics.Registry.MustRegister(collectors.ReconcileTotal)
	ctrlmetrics.Registry.MustRegister(collectors.ReconcileDuration)
	ctrlmetrics.Registry.MustRegister(collectors.ActionTotal)
	ctrlmetrics.Registry.MustRegister(collectors.ActionLatency)
	ctrlmetrics.Registry.MustRegister(collectors.SkippedTotal)
	ctrlmetrics.Registry.MustRegister(collectors.QueueDepth)
	ctrlmetrics.Registry.MustRegister(collectors.QueueAdds)
	ctrlmetrics.Registry.MustRegister(collectors.QueueLatency)
	ctrlmetrics.Registry.MustRegister(collectors.ErrorsTotal)
	ctrlmetrics.Registry.MustRegister(collectors.RetriesTotal)
	ctrlmetrics.Registry.MustRegister(collectors.EventsReceived)
	ctrlmetrics.Registry.MustRegister(collectors.EventsProcessed)
	ctrlmetrics.Registry.MustRegister(collectors.WorkloadsScanned)
	ctrlmetrics.Registry.MustRegister(collectors.WorkloadsMatched)

	if os.Getenv("METRICS_COUNT_BY_NAMESPACE") == "enabled" {
		ctrlmetrics.Registry.MustRegister(collectors.ReloadedByNamespace)
	}

	// Note: For controller-runtime based Reloader, the metrics are served
	// by controller-runtime's metrics server. This http.Handle is kept for
	// the legacy informer-based Reloader which uses its own HTTP server.
	http.Handle("/metrics", promhttp.Handler())

	return collectors
}
