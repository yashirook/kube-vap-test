package commands

import (
	"fmt"

	"github.com/yashirook/kube-vap-test/internal/reporter"
)

// CommonOptions contains options shared by multiple commands
type CommonOptions struct {
	// Output format (table, json, yaml)
	OutputFormat string
	// Suppress progress information
	Quiet bool
	// Show detailed output
	Verbose bool
	// Path to kubeconfig file
	Kubeconfig string
}

// ValidateCommonOptions validates common options
func (o *CommonOptions) ValidateCommonOptions() error {
	switch o.OutputFormat {
	case "table", "json", "yaml":
		// Valid formats
	default:
		return fmt.Errorf("invalid output format: %s (must be table, json, or yaml)", o.OutputFormat)
	}
	return nil
}

// GetReporter returns a reporter based on common options
func (o *CommonOptions) GetReporter() reporter.Reporter {
	return reporter.NewReporter(reporter.OutputFormat(o.OutputFormat), o.Verbose)
}