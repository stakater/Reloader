package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	summaryResultsDir string
	summaryOutputFile string
	summaryFormat     string
	summaryTestType   string
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Generate summary across all scenarios (for CI)",
	Long: `Generate an aggregated summary report across all test scenarios.

Examples:
  # Generate markdown summary for CI
  loadtest summary --results-dir=./results --format=markdown`,
	Run: func(cmd *cobra.Command, args []string) {
		summaryCommand()
	},
}

func init() {
	summaryCmd.Flags().StringVar(&summaryResultsDir, "results-dir", "./results", "Directory containing results")
	summaryCmd.Flags().StringVar(&summaryOutputFile, "output", "", "Output file (default: stdout)")
	summaryCmd.Flags().StringVar(&summaryFormat, "format", "markdown", "Output format: text, json, markdown")
	summaryCmd.Flags().StringVar(&summaryTestType, "test-type", "full", "Test type label: quick, full")
}

// SummaryReport aggregates results from multiple scenarios.
type SummaryReport struct {
	Timestamp  time.Time         `json:"timestamp"`
	TestType   string            `json:"test_type"`
	PassCount  int               `json:"pass_count"`
	FailCount  int               `json:"fail_count"`
	TotalCount int               `json:"total_count"`
	Scenarios  []ScenarioSummary `json:"scenarios"`
}

// ScenarioSummary provides a brief summary of a single scenario.
type ScenarioSummary struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Description string  `json:"description"`
	ActionTotal float64 `json:"action_total"`
	ActionExp   float64 `json:"action_expected"`
	ErrorsTotal float64 `json:"errors_total"`
}

func summaryCommand() {
	summary, err := generateSummaryReport(summaryResultsDir, summaryTestType)
	if err != nil {
		log.Fatalf("Failed to generate summary: %v", err)
	}

	var output string
	switch OutputFormat(summaryFormat) {
	case OutputFormatJSON:
		output = renderSummaryJSON(summary)
	case OutputFormatText:
		output = renderSummaryText(summary)
	default:
		output = renderSummaryMarkdown(summary)
	}

	if summaryOutputFile != "" {
		if err := os.WriteFile(summaryOutputFile, []byte(output), 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		log.Printf("Summary written to %s", summaryOutputFile)
	} else {
		fmt.Print(output)
	}

	if summary.FailCount > 0 {
		os.Exit(1)
	}
}

func generateSummaryReport(resultsDir, testType string) (*SummaryReport, error) {
	summary := &SummaryReport{
		Timestamp: time.Now(),
		TestType:  testType,
	}

	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read results directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "S") {
			continue
		}

		scenarioID := entry.Name()
		report, err := generateScenarioReport(scenarioID, resultsDir)
		if err != nil {
			log.Printf("Warning: failed to load scenario %s: %v", scenarioID, err)
			continue
		}

		scenarioSummary := ScenarioSummary{
			ID:          scenarioID,
			Status:      report.OverallStatus,
			Description: report.TestDescription,
		}

		for _, c := range report.Comparisons {
			switch c.Name {
			case "action_total":
				scenarioSummary.ActionTotal = c.NewValue
				scenarioSummary.ActionExp = c.Expected
			case "errors_total":
				scenarioSummary.ErrorsTotal = c.NewValue
			}
		}

		summary.Scenarios = append(summary.Scenarios, scenarioSummary)
		summary.TotalCount++
		if report.OverallStatus == "PASS" {
			summary.PassCount++
		} else {
			summary.FailCount++
		}
	}

	sort.Slice(summary.Scenarios, func(i, j int) bool {
		return naturalSort(summary.Scenarios[i].ID, summary.Scenarios[j].ID)
	})

	return summary, nil
}

func naturalSort(a, b string) bool {
	var aNum, bNum int
	fmt.Sscanf(a, "S%d", &aNum)
	fmt.Sscanf(b, "S%d", &bNum)
	return aNum < bNum
}

func renderSummaryJSON(summary *SummaryReport) string {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(data)
}

func renderSummaryText(summary *SummaryReport) string {
	var sb strings.Builder

	sb.WriteString("================================================================================\n")
	sb.WriteString("                       LOAD TEST SUMMARY\n")
	sb.WriteString("================================================================================\n\n")

	passRate := 0
	if summary.TotalCount > 0 {
		passRate = summary.PassCount * 100 / summary.TotalCount
	}

	fmt.Fprintf(&sb, "Test Type: %s\n", summary.TestType)
	fmt.Fprintf(&sb, "Results:   %d/%d passed (%d%%)\n\n", summary.PassCount, summary.TotalCount, passRate)

	fmt.Fprintf(&sb, "%-6s %-8s %-45s %10s %8s\n", "ID", "Status", "Description", "Actions", "Errors")
	fmt.Fprintf(&sb, "%-6s %-8s %-45s %10s %8s\n", "------", "--------", strings.Repeat("-", 45), "----------", "--------")

	for _, s := range summary.Scenarios {
		desc := s.Description
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}
		actions := fmt.Sprintf("%.0f", s.ActionTotal)
		if s.ActionExp > 0 {
			actions = fmt.Sprintf("%.0f/%.0f", s.ActionTotal, s.ActionExp)
		}
		fmt.Fprintf(&sb, "%-6s %-8s %-45s %10s %8.0f\n", s.ID, s.Status, desc, actions, s.ErrorsTotal)
	}

	sb.WriteString("\n================================================================================\n")
	return sb.String()
}

func renderSummaryMarkdown(summary *SummaryReport) string {
	var sb strings.Builder

	emoji := "✅"
	title := "ALL TESTS PASSED"
	if summary.FailCount > 0 {
		emoji = "❌"
		title = fmt.Sprintf("%d TEST(S) FAILED", summary.FailCount)
	} else if summary.TotalCount == 0 {
		emoji = "⚠️"
		title = "NO RESULTS"
	}

	sb.WriteString(fmt.Sprintf("## %s Load Test Results: %s\n\n", emoji, title))

	if summary.TestType == "quick" {
		sb.WriteString("> 🚀 **Quick Test** (S1, S4, S6) — Use `/loadtest` for full suite\n\n")
	}

	passRate := 0
	if summary.TotalCount > 0 {
		passRate = summary.PassCount * 100 / summary.TotalCount
	}
	sb.WriteString(fmt.Sprintf("**%d/%d passed** (%d%%)\n\n", summary.PassCount, summary.TotalCount, passRate))

	sb.WriteString("| | Scenario | Description | Actions | Errors |\n")
	sb.WriteString("|:-:|:--------:|-------------|:-------:|:------:|\n")

	for _, s := range summary.Scenarios {
		icon := "✅"
		if s.Status != "PASS" {
			icon = "❌"
		}

		desc := s.Description
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}

		actions := fmt.Sprintf("%.0f", s.ActionTotal)
		if s.ActionExp > 0 {
			actions = fmt.Sprintf("%.0f/%.0f", s.ActionTotal, s.ActionExp)
		}

		errors := fmt.Sprintf("%.0f", s.ErrorsTotal)
		if s.ErrorsTotal > 0 {
			errors = fmt.Sprintf("⚠️ %.0f", s.ErrorsTotal)
		}

		sb.WriteString(fmt.Sprintf("| %s | **%s** | %s | %s | %s |\n", icon, s.ID, desc, actions, errors))
	}

	sb.WriteString("\n📦 **[Download detailed results](../artifacts)**\n")

	return sb.String()
}
