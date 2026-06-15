package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	// DefaultClusterName is the default kind cluster name.
	DefaultClusterName = "reloader-loadtest"
	// TestNamespace is the namespace used for test resources.
	TestNamespace = "reloader-test"
)

// OutputFormat defines the output format for reports.
type OutputFormat string

const (
	OutputFormatText     OutputFormat = "text"
	OutputFormatJSON     OutputFormat = "json"
	OutputFormatMarkdown OutputFormat = "markdown"
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "loadtest",
	Short: "Reloader Load Test CLI",
	Long:  `A CLI tool for running A/B comparison load tests on Reloader.`,
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(summaryCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
