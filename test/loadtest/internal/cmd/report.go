package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	reportScenario   string
	reportResultsDir string
	reportOutputFile string
	reportFormat     string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate comparison report for a scenario",
	Long: `Generate a detailed report for a specific test scenario.

Examples:
  # Generate report for a scenario
  loadtest report --scenario=S2 --results-dir=./results

  # Generate JSON report
  loadtest report --scenario=S2 --format=json`,
	Run: func(cmd *cobra.Command, args []string) {
		reportCommand()
	},
}

func init() {
	reportCmd.Flags().StringVar(&reportScenario, "scenario", "", "Scenario to report on (required)")
	reportCmd.Flags().StringVar(&reportResultsDir, "results-dir", "./results", "Directory containing results")
	reportCmd.Flags().StringVar(&reportOutputFile, "output", "", "Output file (default: stdout)")
	reportCmd.Flags().StringVar(&reportFormat, "format", "text", "Output format: text, json, markdown")
	reportCmd.MarkFlagRequired("scenario")
}

// PrometheusResponse represents a Prometheus API response for report parsing.
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// MetricComparison represents the comparison of a single metric.
type MetricComparison struct {
	Name             string  `json:"name"`
	DisplayName      string  `json:"display_name"`
	Unit             string  `json:"unit"`
	IsCounter        bool    `json:"is_counter"`
	OldValue         float64 `json:"old_value"`
	NewValue         float64 `json:"new_value"`
	Expected         float64 `json:"expected"`
	Difference       float64 `json:"difference"`
	DiffPct          float64 `json:"diff_pct"`
	Status           string  `json:"status"`
	Threshold        float64 `json:"threshold"`
	OldMeetsExpected string  `json:"old_meets_expected"`
	NewMeetsExpected string  `json:"new_meets_expected"`
}

type metricInfo struct {
	unit      string
	isCounter bool
}

var metricInfoMap = map[string]metricInfo{
	"reconcile_total":              {unit: "count", isCounter: true},
	"reconcile_duration_p50":       {unit: "s", isCounter: false},
	"reconcile_duration_p95":       {unit: "s", isCounter: false},
	"reconcile_duration_p99":       {unit: "s", isCounter: false},
	"action_total":                 {unit: "count", isCounter: true},
	"action_latency_p50":           {unit: "s", isCounter: false},
	"action_latency_p95":           {unit: "s", isCounter: false},
	"action_latency_p99":           {unit: "s", isCounter: false},
	"errors_total":                 {unit: "count", isCounter: true},
	"reload_executed_total":        {unit: "count", isCounter: true},
	"workloads_scanned_total":      {unit: "count", isCounter: true},
	"workloads_matched_total":      {unit: "count", isCounter: true},
	"skipped_total_no_data_change": {unit: "count", isCounter: true},
	"rest_client_requests_total":   {unit: "count", isCounter: true},
	"rest_client_requests_get":     {unit: "count", isCounter: true},
	"rest_client_requests_patch":   {unit: "count", isCounter: true},
	"rest_client_requests_put":     {unit: "count", isCounter: true},
	"rest_client_requests_errors":  {unit: "count", isCounter: true},
	"memory_rss_mb_avg":            {unit: "MB", isCounter: false},
	"memory_rss_mb_max":            {unit: "MB", isCounter: false},
	"memory_heap_mb_avg":           {unit: "MB", isCounter: false},
	"memory_heap_mb_max":           {unit: "MB", isCounter: false},
	"cpu_cores_avg":                {unit: "cores", isCounter: false},
	"cpu_cores_max":                {unit: "cores", isCounter: false},
	"goroutines_avg":               {unit: "count", isCounter: false},
	"goroutines_max":               {unit: "count", isCounter: false},
	"gc_pause_p99_ms":              {unit: "ms", isCounter: false},
}

// ReportExpectedMetrics matches the expected metrics from test scenarios.
type ReportExpectedMetrics struct {
	ActionTotal           int    `json:"action_total"`
	ReloadExecutedTotal   int    `json:"reload_executed_total"`
	ReconcileTotal        int    `json:"reconcile_total"`
	WorkloadsScannedTotal int    `json:"workloads_scanned_total"`
	WorkloadsMatchedTotal int    `json:"workloads_matched_total"`
	SkippedTotal          int    `json:"skipped_total"`
	Description           string `json:"description"`
}

// ScenarioReport represents the full report for a scenario.
type ScenarioReport struct {
	Scenario        string             `json:"scenario"`
	Timestamp       time.Time          `json:"timestamp"`
	Comparisons     []MetricComparison `json:"comparisons"`
	OverallStatus   string             `json:"overall_status"`
	Summary         string             `json:"summary"`
	PassCriteria    []string           `json:"pass_criteria"`
	FailedCriteria  []string           `json:"failed_criteria"`
	Expected        ReportExpectedMetrics `json:"expected"`
	TestDescription string             `json:"test_description"`
}

// MetricType defines how to evaluate a metric.
type MetricType int

const (
	LowerIsBetter MetricType = iota
	ShouldMatch
	HigherIsBetter
	Informational
)

type thresholdConfig struct {
	maxDiff    float64
	metricType MetricType
	minAbsDiff float64
}

var thresholds = map[string]thresholdConfig{
	"reconcile_total":              {maxDiff: 60.0, metricType: LowerIsBetter},
	"reconcile_duration_p50":       {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.5},
	"reconcile_duration_p95":       {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"reconcile_duration_p99":       {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"action_latency_p50":           {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.5},
	"action_latency_p95":           {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"action_latency_p99":           {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"errors_total":                 {maxDiff: 0.0, metricType: LowerIsBetter},
	"action_total":                 {maxDiff: 15.0, metricType: ShouldMatch},
	"reload_executed_total":        {maxDiff: 15.0, metricType: ShouldMatch},
	"workloads_scanned_total":      {maxDiff: 15.0, metricType: ShouldMatch},
	"workloads_matched_total":      {maxDiff: 15.0, metricType: ShouldMatch},
	"skipped_total_no_data_change": {maxDiff: 20.0, metricType: ShouldMatch},
	"rest_client_requests_total":   {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 50},
	"rest_client_requests_get":     {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 50},
	"rest_client_requests_patch":   {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 50},
	"rest_client_requests_put":     {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 20},
	"rest_client_requests_errors":  {maxDiff: 0.0, metricType: LowerIsBetter, minAbsDiff: 100},
	"memory_rss_mb_avg":            {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 20},
	"memory_rss_mb_max":            {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 30},
	"memory_heap_mb_avg":           {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 15},
	"memory_heap_mb_max":           {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 20},
	"cpu_cores_avg":                {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.1},
	"cpu_cores_max":                {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.2},
	"goroutines_avg":               {metricType: Informational},
	"goroutines_max":               {metricType: Informational},
	"gc_pause_p99_ms":              {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 5},
}

func reportCommand() {
	if reportScenario == "" {
		log.Fatal("--scenario is required for report command")
	}

	report, err := generateScenarioReport(reportScenario, reportResultsDir)
	if err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	var output string
	switch OutputFormat(reportFormat) {
	case OutputFormatJSON:
		output = renderScenarioReportJSON(report)
	case OutputFormatMarkdown:
		output = renderScenarioReportMarkdown(report)
	default:
		output = renderScenarioReport(report)
	}

	if reportOutputFile != "" {
		if err := os.WriteFile(reportOutputFile, []byte(output), 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		log.Printf("Report written to %s", reportOutputFile)
	} else {
		fmt.Println(output)
	}
}

func generateScenarioReport(scenario, resultsDir string) (*ScenarioReport, error) {
	oldDir := filepath.Join(resultsDir, scenario, "old")
	newDir := filepath.Join(resultsDir, scenario, "new")
	scenarioDir := filepath.Join(resultsDir, scenario)

	_, oldErr := os.Stat(oldDir)
	_, newErr := os.Stat(newDir)
	hasOld := oldErr == nil
	hasNew := newErr == nil
	isComparison := hasOld && hasNew

	singleVersion := ""
	singleDir := ""
	if !isComparison {
		if hasNew {
			singleVersion = "new"
			singleDir = newDir
		} else if hasOld {
			singleVersion = "old"
			singleDir = oldDir
		} else {
			return nil, fmt.Errorf("no results found in %s", scenarioDir)
		}
	}

	report := &ScenarioReport{
		Scenario:  scenario,
		Timestamp: time.Now(),
	}

	expectedPath := filepath.Join(scenarioDir, "expected.json")
	if data, err := os.ReadFile(expectedPath); err == nil {
		if err := json.Unmarshal(data, &report.Expected); err != nil {
			log.Printf("Warning: Could not parse expected metrics: %v", err)
		} else {
			report.TestDescription = report.Expected.Description
		}
	}

	if !isComparison {
		return generateSingleVersionReport(report, singleDir, singleVersion, scenario)
	}

	metricsToCompare := []struct {
		name     string
		file     string
		selector func(data PrometheusResponse) float64
	}{
		{"reconcile_total", "reloader_reconcile_total.json", sumAllValues},
		{"reconcile_duration_p50", "reconcile_p50.json", getFirstValue},
		{"reconcile_duration_p95", "reconcile_p95.json", getFirstValue},
		{"reconcile_duration_p99", "reconcile_p99.json", getFirstValue},
		{"action_total", "reloader_action_total.json", sumAllValues},
		{"action_latency_p50", "action_p50.json", getFirstValue},
		{"action_latency_p95", "action_p95.json", getFirstValue},
		{"action_latency_p99", "action_p99.json", getFirstValue},
		{"errors_total", "reloader_errors_total.json", sumAllValues},
		{"reload_executed_total", "reloader_reload_executed_total.json", sumSuccessValues},
		{"workloads_scanned_total", "reloader_workloads_scanned_total.json", sumAllValues},
		{"workloads_matched_total", "reloader_workloads_matched_total.json", sumAllValues},
		{"rest_client_requests_total", "rest_client_requests_total.json", getFirstValue},
		{"rest_client_requests_get", "rest_client_requests_get.json", getFirstValue},
		{"rest_client_requests_patch", "rest_client_requests_patch.json", getFirstValue},
		{"rest_client_requests_put", "rest_client_requests_put.json", getFirstValue},
		{"rest_client_requests_errors", "rest_client_requests_errors.json", getFirstValue},
		{"memory_rss_mb_avg", "memory_rss_bytes_avg.json", bytesToMB},
		{"memory_rss_mb_max", "memory_rss_bytes_max.json", bytesToMB},
		{"memory_heap_mb_avg", "memory_heap_bytes_avg.json", bytesToMB},
		{"memory_heap_mb_max", "memory_heap_bytes_max.json", bytesToMB},
		{"cpu_cores_avg", "cpu_usage_cores_avg.json", getFirstValue},
		{"cpu_cores_max", "cpu_usage_cores_max.json", getFirstValue},
		{"goroutines_avg", "goroutines_avg.json", getFirstValue},
		{"goroutines_max", "goroutines_max.json", getFirstValue},
		{"gc_pause_p99_ms", "gc_duration_seconds_p99.json", secondsToMs},
	}

	expectedValues := map[string]float64{
		"action_total":            float64(report.Expected.ActionTotal),
		"reload_executed_total":   float64(report.Expected.ReloadExecutedTotal),
		"reconcile_total":         float64(report.Expected.ReconcileTotal),
		"workloads_scanned_total": float64(report.Expected.WorkloadsScannedTotal),
		"workloads_matched_total": float64(report.Expected.WorkloadsMatchedTotal),
		"skipped_total":           float64(report.Expected.SkippedTotal),
	}

	metricValues := make(map[string]struct{ old, new, expected float64 })

	for _, m := range metricsToCompare {
		oldData, err := loadMetricFile(filepath.Join(oldDir, m.file))
		if err != nil {
			log.Printf("Warning: Could not load old metric %s: %v", m.name, err)
			continue
		}

		newData, err := loadMetricFile(filepath.Join(newDir, m.file))
		if err != nil {
			log.Printf("Warning: Could not load new metric %s: %v", m.name, err)
			continue
		}

		oldValue := m.selector(oldData)
		newValue := m.selector(newData)
		expected := expectedValues[m.name]

		metricValues[m.name] = struct{ old, new, expected float64 }{oldValue, newValue, expected}
	}

	newMeetsActionExpected := false
	newReconcileIsZero := false
	isChurnScenario := scenario == "S5"
	if v, ok := metricValues["action_total"]; ok && v.expected > 0 {
		tolerance := v.expected * 0.15
		newMeetsActionExpected = math.Abs(v.new-v.expected) <= tolerance
	}
	if v, ok := metricValues["reconcile_total"]; ok {
		newReconcileIsZero = v.new == 0
	}

	for _, m := range metricsToCompare {
		v, ok := metricValues[m.name]
		if !ok {
			continue
		}

		comparison := compareMetricWithExpected(m.name, v.old, v.new, v.expected)

		if strings.HasPrefix(m.name, "rest_client_requests") {
			if newMeetsActionExpected && comparison.Status != "pass" {
				if oldMeets, ok := metricValues["action_total"]; ok {
					oldTolerance := oldMeets.expected * 0.15
					oldMissed := math.Abs(oldMeets.old-oldMeets.expected) > oldTolerance
					if oldMissed {
						comparison.Status = "pass"
					}
				}
			}
			if newReconcileIsZero && comparison.Status != "pass" {
				comparison.Status = "pass"
			}
		}

		if isChurnScenario {
			if m.name == "errors_total" {
				if v.new < 50 && v.old < 50 {
					comparison.Status = "pass"
				} else if v.new <= v.old*1.5 {
					comparison.Status = "pass"
				}
			}
			if m.name == "action_total" || m.name == "reload_executed_total" {
				if v.old > 0 {
					diff := math.Abs(v.new-v.old) / v.old * 100
					if diff <= 20 {
						comparison.Status = "pass"
					}
				} else if v.new > 0 {
					comparison.Status = "pass"
				}
			}
		}

		report.Comparisons = append(report.Comparisons, comparison)

		if comparison.Status == "pass" {
			report.PassCriteria = append(report.PassCriteria, m.name)
		} else if comparison.Status == "fail" {
			report.FailedCriteria = append(report.FailedCriteria, m.name)
		}
	}

	if len(report.FailedCriteria) == 0 {
		report.OverallStatus = "PASS"
		report.Summary = "All metrics within acceptable thresholds"
	} else {
		report.OverallStatus = "FAIL"
		report.Summary = fmt.Sprintf("%d metrics failed: %s",
			len(report.FailedCriteria),
			strings.Join(report.FailedCriteria, ", "))
	}

	return report, nil
}

func generateSingleVersionReport(report *ScenarioReport, dataDir, version, scenario string) (*ScenarioReport, error) {
	metricsToCollect := []struct {
		name     string
		file     string
		selector func(data PrometheusResponse) float64
	}{
		{"reconcile_total", "reloader_reconcile_total.json", sumAllValues},
		{"reconcile_duration_p50", "reconcile_p50.json", getFirstValue},
		{"reconcile_duration_p95", "reconcile_p95.json", getFirstValue},
		{"reconcile_duration_p99", "reconcile_p99.json", getFirstValue},
		{"action_total", "reloader_action_total.json", sumAllValues},
		{"action_latency_p50", "action_p50.json", getFirstValue},
		{"action_latency_p95", "action_p95.json", getFirstValue},
		{"action_latency_p99", "action_p99.json", getFirstValue},
		{"errors_total", "reloader_errors_total.json", sumAllValues},
		{"reload_executed_total", "reloader_reload_executed_total.json", sumSuccessValues},
		{"workloads_scanned_total", "reloader_workloads_scanned_total.json", sumAllValues},
		{"workloads_matched_total", "reloader_workloads_matched_total.json", sumAllValues},
		{"rest_client_requests_total", "rest_client_requests_total.json", getFirstValue},
		{"rest_client_requests_get", "rest_client_requests_get.json", getFirstValue},
		{"rest_client_requests_patch", "rest_client_requests_patch.json", getFirstValue},
		{"rest_client_requests_put", "rest_client_requests_put.json", getFirstValue},
		{"rest_client_requests_errors", "rest_client_requests_errors.json", getFirstValue},
		{"memory_rss_mb_avg", "memory_rss_bytes_avg.json", bytesToMB},
		{"memory_rss_mb_max", "memory_rss_bytes_max.json", bytesToMB},
		{"memory_heap_mb_avg", "memory_heap_bytes_avg.json", bytesToMB},
		{"memory_heap_mb_max", "memory_heap_bytes_max.json", bytesToMB},
		{"cpu_cores_avg", "cpu_usage_cores_avg.json", getFirstValue},
		{"cpu_cores_max", "cpu_usage_cores_max.json", getFirstValue},
		{"goroutines_avg", "goroutines_avg.json", getFirstValue},
		{"goroutines_max", "goroutines_max.json", getFirstValue},
		{"gc_pause_p99_ms", "gc_duration_seconds_p99.json", secondsToMs},
	}

	expectedValues := map[string]float64{
		"action_total":            float64(report.Expected.ActionTotal),
		"reload_executed_total":   float64(report.Expected.ReloadExecutedTotal),
		"reconcile_total":         float64(report.Expected.ReconcileTotal),
		"workloads_scanned_total": float64(report.Expected.WorkloadsScannedTotal),
		"workloads_matched_total": float64(report.Expected.WorkloadsMatchedTotal),
		"skipped_total":           float64(report.Expected.SkippedTotal),
	}

	for _, m := range metricsToCollect {
		data, err := loadMetricFile(filepath.Join(dataDir, m.file))
		if err != nil {
			log.Printf("Warning: Could not load metric %s: %v", m.name, err)
			continue
		}

		value := m.selector(data)
		expected := expectedValues[m.name]

		info := metricInfoMap[m.name]
		if info.unit == "" {
			info = metricInfo{unit: "count", isCounter: true}
		}

		displayName := m.name
		if info.unit != "count" {
			displayName = fmt.Sprintf("%s (%s)", m.name, info.unit)
		}

		status := "info"
		meetsExp := "-"

		if expected > 0 {
			meetsExp = meetsExpected(value, expected)
			threshold, ok := thresholds[m.name]
			if ok && threshold.metricType == ShouldMatch {
				if meetsExp == "✓" {
					status = "pass"
					report.PassCriteria = append(report.PassCriteria, m.name)
				} else {
					status = "fail"
					report.FailedCriteria = append(report.FailedCriteria, m.name)
				}
			}
		}

		if info.isCounter {
			value = math.Round(value)
		}

		report.Comparisons = append(report.Comparisons, MetricComparison{
			Name:             m.name,
			DisplayName:      displayName,
			Unit:             info.unit,
			IsCounter:        info.isCounter,
			OldValue:         0,
			NewValue:         value,
			Expected:         expected,
			OldMeetsExpected: "-",
			NewMeetsExpected: meetsExp,
			Status:           status,
		})
	}

	if len(report.FailedCriteria) == 0 {
		report.OverallStatus = "PASS"
		report.Summary = fmt.Sprintf("Single-version test (%s) completed successfully", version)
	} else {
		report.OverallStatus = "FAIL"
		report.Summary = fmt.Sprintf("%d metrics failed: %s",
			len(report.FailedCriteria),
			strings.Join(report.FailedCriteria, ", "))
	}

	return report, nil
}

func loadMetricFile(path string) (PrometheusResponse, error) {
	var resp PrometheusResponse
	data, err := os.ReadFile(path)
	if err != nil {
		return resp, err
	}
	err = json.Unmarshal(data, &resp)
	return resp, err
}

func sumAllValues(data PrometheusResponse) float64 {
	var sum float64
	for _, result := range data.Data.Result {
		if len(result.Value) >= 2 {
			if v, ok := result.Value[1].(string); ok {
				var f float64
				fmt.Sscanf(v, "%f", &f)
				sum += f
			}
		}
	}
	return sum
}

func sumSuccessValues(data PrometheusResponse) float64 {
	var sum float64
	for _, result := range data.Data.Result {
		if result.Metric["success"] == "true" {
			if len(result.Value) >= 2 {
				if v, ok := result.Value[1].(string); ok {
					var f float64
					fmt.Sscanf(v, "%f", &f)
					sum += f
				}
			}
		}
	}
	return sum
}

func getFirstValue(data PrometheusResponse) float64 {
	if len(data.Data.Result) > 0 && len(data.Data.Result[0].Value) >= 2 {
		if v, ok := data.Data.Result[0].Value[1].(string); ok {
			var f float64
			fmt.Sscanf(v, "%f", &f)
			return f
		}
	}
	return 0
}

func bytesToMB(data PrometheusResponse) float64 {
	bytes := getFirstValue(data)
	return bytes / (1024 * 1024)
}

func secondsToMs(data PrometheusResponse) float64 {
	seconds := getFirstValue(data)
	return seconds * 1000
}

func meetsExpected(value, expected float64) string {
	if expected == 0 {
		return "-"
	}
	tolerance := expected * 0.15
	if math.Abs(value-expected) <= tolerance {
		return "✓"
	}
	return "✗"
}

func compareMetricWithExpected(name string, oldValue, newValue, expected float64) MetricComparison {
	diff := newValue - oldValue
	absDiff := math.Abs(diff)
	var diffPct float64
	if oldValue != 0 {
		diffPct = (diff / oldValue) * 100
	} else if newValue != 0 {
		diffPct = 100
	}

	threshold, ok := thresholds[name]
	if !ok {
		threshold = thresholdConfig{maxDiff: 10.0, metricType: ShouldMatch}
	}

	info := metricInfoMap[name]
	if info.unit == "" {
		info = metricInfo{unit: "count", isCounter: true}
	}
	displayName := name
	if info.unit != "count" {
		displayName = fmt.Sprintf("%s (%s)", name, info.unit)
	}

	if info.isCounter {
		oldValue = math.Round(oldValue)
		newValue = math.Round(newValue)
	}

	status := "pass"
	oldMeetsExp := meetsExpected(oldValue, expected)
	newMeetsExp := meetsExpected(newValue, expected)

	isNewMetric := info.isCounter && oldValue == 0 && newValue > 0 && expected == 0

	if isNewMetric {
		status = "info"
	} else if expected > 0 && threshold.metricType == ShouldMatch {
		if newMeetsExp == "✗" {
			status = "fail"
		}
	} else {
		switch threshold.metricType {
		case LowerIsBetter:
			if threshold.minAbsDiff > 0 && absDiff < threshold.minAbsDiff {
				status = "pass"
			} else if diffPct > threshold.maxDiff {
				status = "fail"
			}
		case HigherIsBetter:
			if diffPct < -threshold.maxDiff {
				status = "fail"
			}
		case ShouldMatch:
			if math.Abs(diffPct) > threshold.maxDiff {
				status = "fail"
			}
		case Informational:
			status = "info"
		}
	}

	return MetricComparison{
		Name:             name,
		DisplayName:      displayName,
		Unit:             info.unit,
		IsCounter:        info.isCounter,
		Expected:         expected,
		OldMeetsExpected: oldMeetsExp,
		NewMeetsExpected: newMeetsExp,
		OldValue:         oldValue,
		NewValue:         newValue,
		Difference:       diff,
		DiffPct:          diffPct,
		Status:           status,
		Threshold:        threshold.maxDiff,
	}
}

func renderScenarioReport(report *ScenarioReport) string {
	var sb strings.Builder

	isSingleVersion := true
	for _, c := range report.Comparisons {
		if c.OldValue != 0 {
			isSingleVersion = false
			break
		}
	}

	sb.WriteString("\n")
	sb.WriteString("================================================================================\n")
	if isSingleVersion {
		sb.WriteString("                        RELOADER TEST REPORT\n")
	} else {
		sb.WriteString("                     RELOADER A/B COMPARISON REPORT\n")
	}
	sb.WriteString("================================================================================\n\n")

	fmt.Fprintf(&sb, "Scenario:     %s\n", report.Scenario)
	fmt.Fprintf(&sb, "Generated:    %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "Status:       %s\n", report.OverallStatus)
	fmt.Fprintf(&sb, "Summary:      %s\n", report.Summary)

	if report.TestDescription != "" {
		fmt.Fprintf(&sb, "Test:         %s\n", report.TestDescription)
	}

	if report.Expected.ActionTotal > 0 {
		sb.WriteString("\n--------------------------------------------------------------------------------\n")
		sb.WriteString("                           EXPECTED VALUES\n")
		sb.WriteString("--------------------------------------------------------------------------------\n")
		fmt.Fprintf(&sb, "Expected Action Total:          %d\n", report.Expected.ActionTotal)
		fmt.Fprintf(&sb, "Expected Reload Executed Total: %d\n", report.Expected.ReloadExecutedTotal)
		if report.Expected.SkippedTotal > 0 {
			fmt.Fprintf(&sb, "Expected Skipped Total:         %d\n", report.Expected.SkippedTotal)
		}
	}

	sb.WriteString("\n--------------------------------------------------------------------------------\n")
	if isSingleVersion {
		sb.WriteString("                              METRICS\n")
	} else {
		sb.WriteString("                           METRIC COMPARISONS\n")
	}
	sb.WriteString("--------------------------------------------------------------------------------\n")

	if isSingleVersion {
		sb.WriteString("(✓ = meets expected value within 15%)\n\n")
		fmt.Fprintf(&sb, "%-32s %12s %10s %5s %8s\n",
			"Metric", "Value", "Expected", "Met?", "Status")
		fmt.Fprintf(&sb, "%-32s %12s %10s %5s %8s\n",
			"------", "-----", "--------", "----", "------")

		for _, c := range report.Comparisons {
			if c.IsCounter {
				if c.Expected > 0 {
					fmt.Fprintf(&sb, "%-32s %12.0f %10.0f %5s %8s\n",
						c.DisplayName, c.NewValue, c.Expected,
						c.NewMeetsExpected, c.Status)
				} else {
					fmt.Fprintf(&sb, "%-32s %12.0f %10s %5s %8s\n",
						c.DisplayName, c.NewValue, "-",
						c.NewMeetsExpected, c.Status)
				}
			} else {
				fmt.Fprintf(&sb, "%-32s %12.4f %10s %5s %8s\n",
					c.DisplayName, c.NewValue, "-",
					c.NewMeetsExpected, c.Status)
			}
		}
	} else {
		sb.WriteString("(Old✓/New✓ = meets expected value within 15%)\n\n")

		fmt.Fprintf(&sb, "%-32s %12s %12s %10s %5s %5s %8s\n",
			"Metric", "Old", "New", "Expected", "Old✓", "New✓", "Status")
		fmt.Fprintf(&sb, "%-32s %12s %12s %10s %5s %5s %8s\n",
			"------", "---", "---", "--------", "----", "----", "------")

		for _, c := range report.Comparisons {
			if c.IsCounter {
				if c.Expected > 0 {
					fmt.Fprintf(&sb, "%-32s %12.0f %12.0f %10.0f %5s %5s %8s\n",
						c.DisplayName, c.OldValue, c.NewValue, c.Expected,
						c.OldMeetsExpected, c.NewMeetsExpected, c.Status)
				} else {
					fmt.Fprintf(&sb, "%-32s %12.0f %12.0f %10s %5s %5s %8s\n",
						c.DisplayName, c.OldValue, c.NewValue, "-",
						c.OldMeetsExpected, c.NewMeetsExpected, c.Status)
				}
			} else {
				fmt.Fprintf(&sb, "%-32s %12.4f %12.4f %10s %5s %5s %8s\n",
					c.DisplayName, c.OldValue, c.NewValue, "-",
					c.OldMeetsExpected, c.NewMeetsExpected, c.Status)
			}
		}
	}

	sb.WriteString("\n--------------------------------------------------------------------------------\n")
	sb.WriteString("                           PASS/FAIL CRITERIA\n")
	sb.WriteString("--------------------------------------------------------------------------------\n\n")

	fmt.Fprintf(&sb, "Passed (%d):\n", len(report.PassCriteria))
	for _, p := range report.PassCriteria {
		fmt.Fprintf(&sb, "  ✓ %s\n", p)
	}

	if len(report.FailedCriteria) > 0 {
		fmt.Fprintf(&sb, "\nFailed (%d):\n", len(report.FailedCriteria))
		for _, f := range report.FailedCriteria {
			fmt.Fprintf(&sb, "  ✗ %s\n", f)
		}
	}

	sb.WriteString("\n--------------------------------------------------------------------------------\n")
	sb.WriteString("                           THRESHOLDS USED\n")
	sb.WriteString("--------------------------------------------------------------------------------\n\n")

	fmt.Fprintf(&sb, "%-35s %10s %15s %18s\n",
		"Metric", "Max Diff%", "Min Abs Diff", "Direction")
	fmt.Fprintf(&sb, "%-35s %10s %15s %18s\n",
		"------", "---------", "------------", "---------")

	var names []string
	for name := range thresholds {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t := thresholds[name]
		var direction string
		switch t.metricType {
		case LowerIsBetter:
			direction = "lower is better"
		case HigherIsBetter:
			direction = "higher is better"
		case ShouldMatch:
			direction = "should match"
		case Informational:
			direction = "info only"
		}
		minAbsDiff := "-"
		if t.minAbsDiff > 0 {
			minAbsDiff = fmt.Sprintf("%.1f", t.minAbsDiff)
		}
		fmt.Fprintf(&sb, "%-35s %9.1f%% %15s %18s\n",
			name, t.maxDiff, minAbsDiff, direction)
	}

	sb.WriteString("\n================================================================================\n")

	return sb.String()
}

func renderScenarioReportJSON(report *ScenarioReport) string {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(data)
}

func renderScenarioReportMarkdown(report *ScenarioReport) string {
	var sb strings.Builder

	emoji := "✅"
	if report.OverallStatus != "PASS" {
		emoji = "❌"
	}

	sb.WriteString(fmt.Sprintf("## %s %s: %s\n\n", emoji, report.Scenario, report.OverallStatus))

	if report.TestDescription != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", report.TestDescription))
	}

	sb.WriteString("| Metric | Value | Expected | Status |\n")
	sb.WriteString("|--------|------:|:--------:|:------:|\n")

	keyMetrics := []string{"action_total", "reload_executed_total", "errors_total", "reconcile_total"}
	for _, name := range keyMetrics {
		for _, c := range report.Comparisons {
			if c.Name == name {
				value := fmt.Sprintf("%.0f", c.NewValue)
				expected := "-"
				if c.Expected > 0 {
					expected = fmt.Sprintf("%.0f", c.Expected)
				}
				status := "✅"
				if c.Status == "fail" {
					status = "❌"
				} else if c.Status == "info" {
					status = "ℹ️"
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", c.DisplayName, value, expected, status))
				break
			}
		}
	}

	return sb.String()
}
