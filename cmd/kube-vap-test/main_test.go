package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// TestCommandStructure verifies that the new command structure works correctly
func TestCommandStructure(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		wantOutput     string
		wantErrOutput  string
	}{
		{
			name:    "run command exists",
			args:    []string{"run", "--help"},
			wantErr: false,
			wantOutput: "Run ValidatingAdmissionPolicy test definitions",
		},
		{
			name:    "check command exists",
			args:    []string{"check", "--help"},
			wantErr: false,
			wantOutput: "Check specified resources",
		},
		{
			name:    "check command has --cluster flag",
			args:    []string{"check", "--help"},
			wantErr: false,
			wantOutput: "--cluster",
		},
		{
			name:    "run command has --skip-bindings flag",
			args:    []string{"run", "--help"},
			wantErr: false,
			wantOutput: "--skip-bindings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", "main.go"}, tt.args...)...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Dir = "."

			err := cmd.Run()
			if (err != nil) != tt.wantErr {
				t.Errorf("command error = %v, wantErr %v", err, tt.wantErr)
			}

			output := stdout.String() + stderr.String()
			if tt.wantOutput != "" && !strings.Contains(output, tt.wantOutput) {
				t.Errorf("output does not contain %q\nGot: %s", tt.wantOutput, output)
			}

			if tt.wantErrOutput != "" && !strings.Contains(stderr.String(), tt.wantErrOutput) {
				t.Errorf("error output does not contain %q\nGot: %s", tt.wantErrOutput, stderr.String())
			}
		})
	}
}

