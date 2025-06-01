package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationResult(t *testing.T) {
	tests := []struct {
		name       string
		allowed    bool
		violations []Violation
		wantReason string
		wantMsg    string
	}{
		{
			name:       "allowed with no violations",
			allowed:    true,
			violations: nil,
			wantReason: "",
			wantMsg:    "",
		},
		{
			name:       "denied with single violation",
			allowed:    false,
			violations: []Violation{{
				Expression: "object.spec.replicas > 0",
				Reason:     "InvalidReplicas",
				Message:    "Replicas must be greater than 0",
			}},
			wantReason: "InvalidReplicas",
			wantMsg:    "Replicas must be greater than 0",
		},
		{
			name:    "denied with multiple violations same reason",
			allowed: false,
			violations: []Violation{
				{
					Expression: "object.spec.replicas > 0",
					Reason:     "InvalidReplicas",
					Message:    "Replicas must be greater than 0",
				},
				{
					Expression: "object.spec.replicas < 100",
					Reason:     "InvalidReplicas",
					Message:    "Replicas must be less than 100",
				},
			},
			wantReason: "InvalidReplicas",
			wantMsg:    "Multiple validation failures (2)",
		},
		{
			name:    "denied with multiple violations different reasons",
			allowed: false,
			violations: []Violation{
				{
					Expression: "object.spec.replicas > 0",
					Reason:     "InvalidReplicas",
					Message:    "Replicas must be greater than 0",
				},
				{
					Expression: "has(object.spec.template)",
					Reason:     "MissingTemplate",
					Message:    "Template is required",
				},
			},
			wantReason: "MultipleViolations",
			wantMsg:    "Multiple validation failures (2)",
		},
		{
			name:    "denied with FailedValidation reason",
			allowed: false,
			violations: []Violation{
				{
					Expression: "object.spec.replicas > 0",
					Reason:     "FailedValidation",
					Message:    "Validation failed",
				},
			},
			wantReason: "FailedValidation",
			wantMsg:    "Validation failed",
		},
		{
			name:    "denied with empty reason",
			allowed: false,
			violations: []Violation{
				{
					Expression: "object.spec.replicas > 0",
					Reason:     "",
					Message:    "Invalid value",
				},
			},
			wantReason: "",
			wantMsg:    "Invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewValidationResult(tt.allowed, tt.violations)

			assert.Equal(t, tt.allowed, result.IsAllowed())
			assert.Equal(t, tt.wantReason, result.GetReason())
			
			gotMsg := result.GetMessage()
			if strings.Contains(tt.wantMsg, "Multiple validation failures") {
				assert.Contains(t, gotMsg, "Multiple validation failures")
			} else {
				assert.Equal(t, tt.wantMsg, gotMsg)
			}

			assert.Equal(t, tt.violations, result.GetViolations())
		})
	}
}

func TestValidationResult_MultipleViolationsMessage(t *testing.T) {
	violations := []Violation{
		{
			Expression: "expr1",
			Reason:     "Reason1",
			Message:    "Message 1",
		},
		{
			Expression: "expr2",
			Reason:     "Reason2", 
			Message:    "Message 2",
		},
		{
			Expression: "expr3",
			Reason:     "FailedValidation",
			Message:    "Message 3",
		},
	}

	result := NewValidationResult(false, violations)
	msg := result.GetMessage()

	assert.Contains(t, msg, "Multiple validation failures (3)")
	assert.Contains(t, msg, "[Reason1] Message 1")
	assert.Contains(t, msg, "[Reason2] Message 2")
	assert.Contains(t, msg, "Message 3")
	assert.NotContains(t, msg, "[FailedValidation]")
}

func TestValidationResult_EdgeCases(t *testing.T) {
	t.Run("denied with empty violations", func(t *testing.T) {
		result := NewValidationResult(false, []Violation{})
		assert.False(t, result.IsAllowed())
		assert.Equal(t, "", result.GetReason())
		assert.Equal(t, "", result.GetMessage())
		assert.Empty(t, result.GetViolations())
	})

	t.Run("allowed with violations (inconsistent state)", func(t *testing.T) {
		result := NewValidationResult(true, []Violation{{
			Reason:  "ShouldNotHappen",
			Message: "This should be ignored",
		}})
		assert.True(t, result.IsAllowed())
		assert.Equal(t, "", result.GetReason())
		assert.Equal(t, "", result.GetMessage())
	})
}