package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"

	kaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
)

// OutputFormat represents the report output format
type OutputFormat string

const (
	// OutputFormatTable represents table format output
	OutputFormatTable OutputFormat = "table"
	// OutputFormatJSON represents JSON format output
	OutputFormatJSON OutputFormat = "json"
	// OutputFormatYAML represents YAML format output
	OutputFormatYAML OutputFormat = "yaml"
)

// Reporter outputs test results report
type Reporter interface {
	// Report outputs test results
	Report(results *kaptestv1.ValidatingAdmissionPolicyTestStatus) error
}

// baseReporter provides common functionality
type baseReporter struct {
	writer  io.Writer
	verbose bool
}

// JSONReporter outputs reports in JSON format
type JSONReporter struct {
	baseReporter
}

// Report outputs test results in JSON format
func (r *JSONReporter) Report(results *kaptestv1.ValidatingAdmissionPolicyTestStatus) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results to JSON: %w", err)
	}

	_, err = fmt.Fprintln(r.writer, string(data))
	return err
}

// YAMLReporter outputs reports in YAML format
type YAMLReporter struct {
	baseReporter
}

// Report outputs test results in YAML format
func (r *YAMLReporter) Report(results *kaptestv1.ValidatingAdmissionPolicyTestStatus) error {
	data, err := yaml.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results to YAML: %w", err)
	}

	_, err = fmt.Fprintln(r.writer, string(data))
	return err
}

// TableReporter outputs reports in table format
type TableReporter struct {
	baseReporter
}

// Report outputs test results in table format
func (r *TableReporter) Report(results *kaptestv1.ValidatingAdmissionPolicyTestStatus) error {
	// Get terminal width
	termWidth := 120 // Default width
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		termWidth = width
	}

	// Calculate dynamic column widths
	maxNameLen := len("Test Name")
	maxReasonLen := len("Reason")
	maxMessageLen := len("Message")

	// Find maximum lengths in data
	for _, result := range results.Results {
		if len(result.Name) > maxNameLen {
			maxNameLen = len(result.Name)
		}
		if result.ActualResponse != nil {
			if len(result.ActualResponse.Reason) > maxReasonLen {
				maxReasonLen = len(result.ActualResponse.Reason)
			}
			if len(result.ActualResponse.Message) > maxMessageLen {
				maxMessageLen = len(result.ActualResponse.Message)
			}
		}
	}

	// Set reasonable limits
	if maxNameLen > 50 && !r.verbose {
		maxNameLen = 50
	}
	if maxReasonLen > 25 {
		maxReasonLen = 25
	}

	// Calculate remaining width for message column
	// Format: name + status(6) + reason + message + separators(12)
	minMessageWidth := 40
	messageWidth := termWidth - maxNameLen - 6 - maxReasonLen - 12
	if messageWidth < minMessageWidth {
		messageWidth = minMessageWidth
	}

	// Colors
	successColor := color.New(color.FgGreen).SprintFunc()
	failColor := color.New(color.FgRed).SprintFunc()
	headerColor := color.New(color.Bold).SprintFunc()

	// Header with separators
	fmt.Fprintln(r.writer, strings.Repeat("=", termWidth))
	fmt.Fprintf(r.writer, "%-*s  %-6s  %-*s  %s\n",
		maxNameLen, headerColor("Test Name"),
		headerColor("Status"),
		maxReasonLen, headerColor("Reason"),
		headerColor("Message"))
	fmt.Fprintln(r.writer, strings.Repeat("-", termWidth))

	// Detail rows
	for _, result := range results.Results {
		statusDisplay := failColor("FAIL")
		if result.Success {
			statusDisplay = successColor("PASS")
		}

		reason := ""
		message := ""

		if result.ActualResponse != nil {
			reason = result.ActualResponse.Reason
			message = result.ActualResponse.Message
		}

		// Format name
		name := result.Name
		if !r.verbose && len(name) > maxNameLen {
			name = name[:maxNameLen-3] + "..."
		}

		// Format reason
		if len(reason) > maxReasonLen {
			reason = reason[:maxReasonLen-3] + "..."
		}

		// Format message - wrap or truncate
		if r.verbose && len(message) > messageWidth {
			// In verbose mode, wrap long messages
			lines := wrapText(message, messageWidth)
			fmt.Fprintf(r.writer, "%-*s  %-6s  %-*s  %s\n",
				maxNameLen, name,
				statusDisplay,
				maxReasonLen, reason,
				lines[0])
			// Print wrapped lines
			for i := 1; i < len(lines); i++ {
				fmt.Fprintf(r.writer, "%-*s  %-6s  %-*s  %s\n",
					maxNameLen, "",
					"",
					maxReasonLen, "",
					lines[i])
			}
		} else {
			// In normal mode, truncate
			if len(message) > messageWidth {
				message = message[:messageWidth-3] + "..."
			}
			fmt.Fprintf(r.writer, "%-*s  %-6s  %-*s  %s\n",
				maxNameLen, name,
				statusDisplay,
				maxReasonLen, reason,
				message)
		}
	}

	// Summary
	fmt.Fprintln(r.writer, strings.Repeat("=", termWidth))
	summaryStr := fmt.Sprintf("Total: %d | Passed: %s | Failed: %s",
		results.Summary.Total,
		successColor(fmt.Sprintf("%d", results.Summary.Successful)),
		failColor(fmt.Sprintf("%d", results.Summary.Failed)))
	
	// Center the summary
	padding := (termWidth - len("Total: XX | Passed: XX | Failed: XX")) / 2
	if padding > 0 {
		fmt.Fprintf(r.writer, "%*s%s\n", padding, "", summaryStr)
	} else {
		fmt.Fprintln(r.writer, summaryStr)
	}

	// Detailed display (verbose mode)
	if r.verbose && results.Summary.Failed > 0 {
		fmt.Fprintln(r.writer)
		fmt.Fprintln(r.writer, headerColor("Failed Test Details:"))
		fmt.Fprintln(r.writer, strings.Repeat("-", termWidth))
		for _, result := range results.Results {
			if !result.Success {
				fmt.Fprintf(r.writer, "\n%s: %s\n", headerColor("Test"), result.Name)
				if result.Details != "" {
					fmt.Fprintf(r.writer, "%s: %s\n", headerColor("Details"), result.Details)
				}
				if result.ActualResponse != nil {
					if result.ActualResponse.Reason != "" {
						fmt.Fprintf(r.writer, "%s: %s\n", headerColor("Reason"), result.ActualResponse.Reason)
					}
					if result.ActualResponse.Message != "" {
						fmt.Fprintf(r.writer, "%s: %s\n", headerColor("Message"), result.ActualResponse.Message)
					}
				}
			}
		}
	}

	return nil
}

// wrapText wraps text to specified width
func wrapText(text string, width int) []string {
	if width <= 0 || len(text) <= width {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine string

	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// NewReporter creates a new Reporter
func NewReporter(format OutputFormat, verbose bool) Reporter {
	var reporter Reporter

	switch format {
	case OutputFormatJSON:
		reporter = &JSONReporter{
			baseReporter: baseReporter{
				writer:  os.Stdout,
				verbose: verbose,
			},
		}
	case OutputFormatYAML:
		reporter = &YAMLReporter{
			baseReporter: baseReporter{
				writer:  os.Stdout,
				verbose: verbose,
			},
		}
	case OutputFormatTable:
		fallthrough
	default:
		reporter = &TableReporter{
			baseReporter: baseReporter{
				writer:  os.Stdout,
				verbose: verbose,
			},
		}
	}

	return reporter
}

// PrintError outputs error message
func PrintError(err error) {
	errColor := color.New(color.FgRed).SprintFunc()
	fmt.Fprintf(os.Stderr, "%s: %s\n", errColor("Error"), err.Error())
}

// PrintSuccess outputs success message
func PrintSuccess(message string) {
	successColor := color.New(color.FgGreen).SprintFunc()
	fmt.Fprintf(os.Stdout, "%s: %s\n", successColor("Success"), message)
}

// PrintInfo outputs info message
func PrintInfo(message string) {
	infoColor := color.New(color.FgBlue).SprintFunc()
	fmt.Fprintf(os.Stdout, "%s: %s\n", infoColor("Info"), message)
}

// PrintWarning outputs warning message
func PrintWarning(message string) {
	warnColor := color.New(color.FgYellow).SprintFunc()
	fmt.Fprintf(os.Stdout, "%s: %s\n", warnColor("Warning"), message)
}
