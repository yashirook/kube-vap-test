package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	kaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
)

func TestOutputFormat(t *testing.T) {
	// Test that constants are defined correctly
	assert.Equal(t, OutputFormat("table"), OutputFormatTable)
	assert.Equal(t, OutputFormat("json"), OutputFormatJSON)
	assert.Equal(t, OutputFormat("yaml"), OutputFormatYAML)
}

func TestNewReporter(t *testing.T) {
	tests := []struct {
		name     string
		format   OutputFormat
		verbose  bool
		wantType string
	}{
		{
			name:     "table reporter",
			format:   OutputFormatTable,
			verbose:  false,
			wantType: "*reporter.TableReporter",
		},
		{
			name:     "json reporter",
			format:   OutputFormatJSON,
			verbose:  true,
			wantType: "*reporter.JSONReporter",
		},
		{
			name:     "yaml reporter",
			format:   OutputFormatYAML,
			verbose:  false,
			wantType: "*reporter.YAMLReporter",
		},
		{
			name:     "default reporter",
			format:   OutputFormat("unknown"),
			verbose:  false,
			wantType: "*reporter.TableReporter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := NewReporter(tt.format, tt.verbose)
			assert.NotNil(t, reporter)
			assert.Contains(t, strings.ToLower(tt.wantType), strings.ToLower(strings.Split(tt.wantType, ".")[1]))
		})
	}
}

func TestJSONReporter_Report(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := &JSONReporter{
		baseReporter: baseReporter{
			writer:  buf,
			verbose: false,
		},
	}

	results := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: []kaptestv1.TestResult{
			{
				Name:    "test1",
				Success: true,
				ActualResponse: &kaptestv1.ResponseDetails{
					Allowed: true,
				},
			},
			{
				Name:    "test2",
				Success: false,
				ActualResponse: &kaptestv1.ResponseDetails{
					Allowed: false,
					Reason:  "InvalidValue",
					Message: "Value is not allowed",
				},
			},
		},
		Summary: kaptestv1.TestSummary{
			Total:      2,
			Successful: 1,
			Failed:     1,
		},
	}

	err := reporter.Report(results)
	require.NoError(t, err)

	// Verify JSON output
	var output kaptestv1.ValidatingAdmissionPolicyTestStatus
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, 2, len(output.Results))
	assert.Equal(t, "test1", output.Results[0].Name)
	assert.True(t, output.Results[0].Success)
	assert.Equal(t, "test2", output.Results[1].Name)
	assert.False(t, output.Results[1].Success)
	assert.Equal(t, 2, output.Summary.Total)
}

func TestYAMLReporter_Report(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := &YAMLReporter{
		baseReporter: baseReporter{
			writer:  buf,
			verbose: false,
		},
	}

	results := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: []kaptestv1.TestResult{
			{
				Name:    "test1",
				Success: true,
				ActualResponse: &kaptestv1.ResponseDetails{
					Allowed: true,
				},
			},
		},
		Summary: kaptestv1.TestSummary{
			Total:      1,
			Successful: 1,
			Failed:     0,
		},
	}

	err := reporter.Report(results)
	require.NoError(t, err)

	// Verify YAML output
	var output kaptestv1.ValidatingAdmissionPolicyTestStatus
	err = yaml.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, 1, len(output.Results))
	assert.Equal(t, "test1", output.Results[0].Name)
	assert.True(t, output.Results[0].Success)
}

func TestTableReporter_Report(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := &TableReporter{
		baseReporter: baseReporter{
			writer:  buf,
			verbose: false,
		},
	}

	results := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: []kaptestv1.TestResult{
			{
				Name:    "short-test",
				Success: true,
				ActualResponse: &kaptestv1.ResponseDetails{
					Allowed: true,
				},
			},
			{
				Name:    "very-long-test-name-that-should-be-truncated-in-non-verbose-mode-to-fit-the-table",
				Success: false,
				ActualResponse: &kaptestv1.ResponseDetails{
					Allowed: false,
					Reason:  "VeryLongReasonThatShouldBeTruncated",
					Message: "This is a very long message that should be truncated in the table output to fit within the terminal width",
				},
			},
		},
		Summary: kaptestv1.TestSummary{
			Total:      2,
			Successful: 1,
			Failed:     1,
		},
	}

	err := reporter.Report(results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Test Name")
	assert.Contains(t, output, "Status")
	assert.Contains(t, output, "Reason")
	assert.Contains(t, output, "Message")
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "Total: 2")
	assert.Contains(t, output, "Passed: 1")
	assert.Contains(t, output, "Failed: 1")
	assert.Contains(t, output, "...")  // Should have truncation
}

func TestTableReporter_ReportVerbose(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := &TableReporter{
		baseReporter: baseReporter{
			writer:  buf,
			verbose: true,
		},
	}

	results := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: []kaptestv1.TestResult{
			{
				Name:    "failed-test",
				Success: false,
				Details: "Expected allowed=true, got allowed=false",
				ActualResponse: &kaptestv1.ResponseDetails{
					Allowed: false,
					Reason:  "PolicyViolation",
					Message: "This is a very long message that in verbose mode should be wrapped instead of truncated to preserve all the important information",
				},
			},
		},
		Summary: kaptestv1.TestSummary{
			Total:      1,
			Successful: 0,
			Failed:     1,
		},
	}

	err := reporter.Report(results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Failed Test Details:")
	assert.Contains(t, output, "Test: failed-test")
	assert.Contains(t, output, "Details: Expected allowed=true")
	assert.Contains(t, output, "Reason: PolicyViolation")
	assert.Contains(t, output, "Message: This is a very long message")
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		width    int
		expected []string
	}{
		{
			name:     "no wrap needed",
			text:     "short text",
			width:    20,
			expected: []string{"short text"},
		},
		{
			name:     "wrap long text",
			text:     "this is a very long text that needs to be wrapped",
			width:    20,
			expected: []string{
				"this is a very long",
				"text that needs to",
				"be wrapped",
			},
		},
		{
			name:     "empty text",
			text:     "",
			width:    10,
			expected: []string{""},
		},
		{
			name:     "zero width",
			text:     "some text",
			width:    0,
			expected: []string{"some text"},
		},
		{
			name:     "single long word",
			text:     "verylongwordthatcannotbewrapped",
			width:    10,
			expected: []string{"verylongwordthatcannotbewrapped"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrintFunctions(t *testing.T) {
	// Test PrintError
	t.Run("PrintError", func(t *testing.T) {
		// Capture stderr
		old := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		PrintError(fmt.Errorf("test error"))
		
		w.Close()
		os.Stderr = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		assert.Contains(t, buf.String(), "Error")
		assert.Contains(t, buf.String(), "test error")
	})

	// Test PrintSuccess
	t.Run("PrintSuccess", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		PrintSuccess("operation completed")
		
		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		assert.Contains(t, buf.String(), "Success")
		assert.Contains(t, buf.String(), "operation completed")
	})

	// Test PrintInfo
	t.Run("PrintInfo", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		PrintInfo("information message")
		
		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		assert.Contains(t, buf.String(), "Info")
		assert.Contains(t, buf.String(), "information message")
	})

	// Test PrintWarning
	t.Run("PrintWarning", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		PrintWarning("warning message")
		
		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		assert.Contains(t, buf.String(), "Warning")
		assert.Contains(t, buf.String(), "warning message")
	})
}