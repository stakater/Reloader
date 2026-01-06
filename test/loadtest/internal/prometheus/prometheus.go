// Package prometheus provides Prometheus deployment and querying functionality.
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Manager handles Prometheus operations.
type Manager struct {
	manifestPath string
	portForward  *exec.Cmd
	localPort    int
	kubeContext  string // Optional: use specific kubeconfig context
}

// NewManager creates a new Prometheus manager.
func NewManager(manifestPath string) *Manager {
	return &Manager{
		manifestPath: manifestPath,
		localPort:    9091, // Use 9091 to avoid conflicts
	}
}

// NewManagerWithPort creates a Prometheus manager with a custom port.
func NewManagerWithPort(manifestPath string, port int, kubeContext string) *Manager {
	return &Manager{
		manifestPath: manifestPath,
		localPort:    port,
		kubeContext:  kubeContext,
	}
}

// kubectl returns kubectl args with optional context
func (m *Manager) kubectl(args ...string) []string {
	if m.kubeContext != "" {
		return append([]string{"--context", m.kubeContext}, args...)
	}
	return args
}

// Deploy deploys Prometheus to the cluster.
func (m *Manager) Deploy(ctx context.Context) error {
	// Create namespace
	cmd := exec.CommandContext(ctx, "kubectl", m.kubectl("create", "namespace", "monitoring", "--dry-run=client", "-o", "yaml")...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("generating namespace yaml: %w", err)
	}

	applyCmd := exec.CommandContext(ctx, "kubectl", m.kubectl("apply", "-f", "-")...)
	applyCmd.Stdin = strings.NewReader(string(out))
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("applying namespace: %w", err)
	}

	// Apply Prometheus manifest
	applyCmd = exec.CommandContext(ctx, "kubectl", m.kubectl("apply", "-f", m.manifestPath)...)
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("applying prometheus manifest: %w", err)
	}

	// Wait for Prometheus to be ready
	fmt.Println("Waiting for Prometheus to be ready...")
	waitCmd := exec.CommandContext(ctx, "kubectl", m.kubectl("wait", "--for=condition=ready", "pod",
		"-l", "app=prometheus", "-n", "monitoring", "--timeout=120s")...)
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("waiting for prometheus: %w", err)
	}

	return nil
}

// StartPortForward starts port-forwarding to Prometheus.
func (m *Manager) StartPortForward(ctx context.Context) error {
	m.StopPortForward()

	// Start port-forward
	m.portForward = exec.CommandContext(ctx, "kubectl", m.kubectl("port-forward",
		"-n", "monitoring", "svc/prometheus", fmt.Sprintf("%d:9090", m.localPort))...)

	if err := m.portForward.Start(); err != nil {
		return fmt.Errorf("starting port-forward: %w", err)
	}

	// Wait for port-forward to be ready
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		if m.isAccessible() {
			fmt.Printf("Prometheus accessible at http://localhost:%d\n", m.localPort)
			return nil
		}
	}

	return fmt.Errorf("prometheus port-forward not ready after 30s")
}

// StopPortForward stops the port-forward process.
func (m *Manager) StopPortForward() {
	if m.portForward != nil && m.portForward.Process != nil {
		m.portForward.Process.Kill()
		m.portForward = nil
	}
	// Also kill any lingering port-forwards
	exec.Command("pkill", "-f", fmt.Sprintf("kubectl port-forward.*prometheus.*%d", m.localPort)).Run()
}

// Reset restarts Prometheus to clear all metrics.
func (m *Manager) Reset(ctx context.Context) error {
	m.StopPortForward()

	// Delete Prometheus pod to reset metrics
	cmd := exec.CommandContext(ctx, "kubectl", m.kubectl("delete", "pod", "-n", "monitoring",
		"-l", "app=prometheus", "--grace-period=0", "--force")...)
	cmd.Run() // Ignore errors

	// Wait for new pod
	fmt.Println("Waiting for Prometheus to restart...")
	waitCmd := exec.CommandContext(ctx, "kubectl", m.kubectl("wait", "--for=condition=ready", "pod",
		"-l", "app=prometheus", "-n", "monitoring", "--timeout=120s")...)
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("waiting for prometheus restart: %w", err)
	}

	// Restart port-forward
	if err := m.StartPortForward(ctx); err != nil {
		return err
	}

	// Wait for scraping to initialize
	fmt.Println("Waiting 5s for Prometheus to initialize scraping...")
	time.Sleep(5 * time.Second)

	return nil
}

func (m *Manager) isAccessible() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", m.localPort), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()

	// Also try HTTP
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/status/config", m.localPort))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// URL returns the local Prometheus URL.
func (m *Manager) URL() string {
	return fmt.Sprintf("http://localhost:%d", m.localPort)
}

// WaitForTarget waits for a specific job to be scraped by Prometheus.
func (m *Manager) WaitForTarget(ctx context.Context, job string, timeout time.Duration) error {
	fmt.Printf("Waiting for Prometheus to discover and scrape job '%s'...\n", job)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.isTargetHealthy(job) {
			fmt.Printf("Prometheus is scraping job '%s'\n", job)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	// Print debug info on timeout
	m.printTargetStatus(job)
	return fmt.Errorf("timeout waiting for Prometheus to scrape job '%s'", job)
}

// isTargetHealthy checks if a job has at least one healthy target.
func (m *Manager) isTargetHealthy(job string) bool {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/targets", m.URL()))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ActiveTargets []struct {
				Labels map[string]string `json:"labels"`
				Health string            `json:"health"`
			} `json:"activeTargets"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return false
	}

	for _, target := range result.Data.ActiveTargets {
		if target.Labels["job"] == job && target.Health == "up" {
			return true
		}
	}
	return false
}

// printTargetStatus prints debug info about targets.
func (m *Manager) printTargetStatus(job string) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/targets", m.URL()))
	if err != nil {
		fmt.Printf("Failed to get targets: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data struct {
			ActiveTargets []struct {
				Labels    map[string]string `json:"labels"`
				Health    string            `json:"health"`
				LastError string            `json:"lastError"`
				ScrapeURL string            `json:"scrapeUrl"`
			} `json:"activeTargets"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("Failed to parse targets: %v\n", err)
		return
	}

	fmt.Printf("Prometheus targets for job '%s':\n", job)
	found := false
	for _, target := range result.Data.ActiveTargets {
		if target.Labels["job"] == job {
			found = true
			fmt.Printf("  - %s: health=%s, lastError=%s\n",
				target.ScrapeURL, target.Health, target.LastError)
		}
	}
	if !found {
		fmt.Printf("  No targets found for job '%s'\n", job)
		fmt.Printf("  Available jobs: ")
		jobs := make(map[string]bool)
		for _, target := range result.Data.ActiveTargets {
			jobs[target.Labels["job"]] = true
		}
		for j := range jobs {
			fmt.Printf("%s ", j)
		}
		fmt.Println()
	}
}

// HasMetrics checks if the specified job has any metrics available.
func (m *Manager) HasMetrics(ctx context.Context, job string) bool {
	query := fmt.Sprintf(`up{job="%s"}`, job)
	result, err := m.Query(ctx, query)
	if err != nil {
		return false
	}
	return len(result.Data.Result) > 0 && result.Data.Result[0].Value[1] == "1"
}

// QueryResponse represents a Prometheus query response.
type QueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// Query executes a PromQL query and returns the response.
func (m *Manager) Query(ctx context.Context, query string) (*QueryResponse, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", m.URL(), url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result QueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &result, nil
}

// CollectMetrics collects all metrics for a scenario and writes to output directory.
func (m *Manager) CollectMetrics(ctx context.Context, job, outputDir, scenario string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	timeRange := "10m"

	// For S6 (restart scenario), use increase() to handle counter resets
	useIncrease := scenario == "S6"

	// Counter metrics
	counterMetrics := []string{
		"reloader_reconcile_total",
		"reloader_action_total",
		"reloader_skipped_total",
		"reloader_errors_total",
		"reloader_events_received_total",
		"reloader_workloads_scanned_total",
		"reloader_workloads_matched_total",
		"reloader_reload_executed_total",
	}

	for _, metric := range counterMetrics {
		var query string
		if useIncrease {
			query = fmt.Sprintf(`sum(increase(%s{job="%s"}[%s])) by (success, reason)`, metric, job, timeRange)
		} else {
			query = fmt.Sprintf(`sum(%s{job="%s"}) by (success, reason)`, metric, job)
		}

		if err := m.queryAndSave(ctx, query, filepath.Join(outputDir, metric+".json")); err != nil {
			fmt.Printf("Warning: failed to collect %s: %v\n", metric, err)
		}
	}

	// Histogram percentiles
	histogramMetrics := []struct {
		name   string
		prefix string
	}{
		{"reloader_reconcile_duration_seconds", "reconcile"},
		{"reloader_action_latency_seconds", "action"},
	}

	for _, hm := range histogramMetrics {
		for _, pct := range []int{50, 95, 99} {
			quantile := float64(pct) / 100
			query := fmt.Sprintf(`histogram_quantile(%v, sum(rate(%s_bucket{job="%s"}[%s])) by (le))`,
				quantile, hm.name, job, timeRange)
			outFile := filepath.Join(outputDir, fmt.Sprintf("%s_p%d.json", hm.prefix, pct))
			if err := m.queryAndSave(ctx, query, outFile); err != nil {
				fmt.Printf("Warning: failed to collect %s p%d: %v\n", hm.name, pct, err)
			}
		}
	}

	// REST client metrics
	restQueries := map[string]string{
		"rest_client_requests_total.json":  fmt.Sprintf(`sum(rest_client_requests_total{job="%s"})`, job),
		"rest_client_requests_get.json":    fmt.Sprintf(`sum(rest_client_requests_total{job="%s",method="GET"})`, job),
		"rest_client_requests_patch.json":  fmt.Sprintf(`sum(rest_client_requests_total{job="%s",method="PATCH"})`, job),
		"rest_client_requests_put.json":    fmt.Sprintf(`sum(rest_client_requests_total{job="%s",method="PUT"})`, job),
		"rest_client_requests_errors.json": fmt.Sprintf(`sum(rest_client_requests_total{job="%s",code=~"[45].."}) or vector(0)`, job),
	}

	for filename, query := range restQueries {
		if err := m.queryAndSave(ctx, query, filepath.Join(outputDir, filename)); err != nil {
			fmt.Printf("Warning: failed to collect %s: %v\n", filename, err)
		}
	}

	// Resource consumption metrics (memory, CPU, goroutines)
	resourceQueries := map[string]string{
		// Memory metrics (in bytes)
		"memory_rss_bytes_avg.json": fmt.Sprintf(`avg_over_time(process_resident_memory_bytes{job="%s"}[%s])`, job, timeRange),
		"memory_rss_bytes_max.json": fmt.Sprintf(`max_over_time(process_resident_memory_bytes{job="%s"}[%s])`, job, timeRange),
		"memory_rss_bytes_cur.json": fmt.Sprintf(`process_resident_memory_bytes{job="%s"}`, job),

		// Heap memory (Go runtime)
		"memory_heap_bytes_avg.json": fmt.Sprintf(`avg_over_time(go_memstats_heap_alloc_bytes{job="%s"}[%s])`, job, timeRange),
		"memory_heap_bytes_max.json": fmt.Sprintf(`max_over_time(go_memstats_heap_alloc_bytes{job="%s"}[%s])`, job, timeRange),

		// CPU metrics (rate of CPU seconds used)
		"cpu_usage_cores_avg.json": fmt.Sprintf(`rate(process_cpu_seconds_total{job="%s"}[%s])`, job, timeRange),
		"cpu_usage_cores_max.json": fmt.Sprintf(`max_over_time(rate(process_cpu_seconds_total{job="%s"}[1m])[%s:1m])`, job, timeRange),

		// Goroutines (concurrency indicator)
		"goroutines_avg.json": fmt.Sprintf(`avg_over_time(go_goroutines{job="%s"}[%s])`, job, timeRange),
		"goroutines_max.json": fmt.Sprintf(`max_over_time(go_goroutines{job="%s"}[%s])`, job, timeRange),
		"goroutines_cur.json": fmt.Sprintf(`go_goroutines{job="%s"}`, job),

		// GC metrics
		"gc_duration_seconds_p99.json": fmt.Sprintf(`histogram_quantile(0.99, sum(rate(go_gc_duration_seconds_bucket{job="%s"}[%s])) by (le))`, job, timeRange),

		// Threads
		"threads_cur.json": fmt.Sprintf(`go_threads{job="%s"}`, job),
	}

	for filename, query := range resourceQueries {
		if err := m.queryAndSave(ctx, query, filepath.Join(outputDir, filename)); err != nil {
			fmt.Printf("Warning: failed to collect %s: %v\n", filename, err)
		}
	}

	return nil
}

func (m *Manager) queryAndSave(ctx context.Context, query, outputPath string) error {
	result, err := m.Query(ctx, query)
	if err != nil {
		// Write empty result on error
		emptyResult := `{"status":"success","data":{"resultType":"vector","result":[]}}`
		return os.WriteFile(outputPath, []byte(emptyResult), 0644)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}
