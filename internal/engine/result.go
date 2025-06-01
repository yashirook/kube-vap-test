package engine

import (
	"fmt"
	"strings"
)

// ValidationResult represents the result of a validation
type ValidationResult interface {
	// IsAllowed returns true if the validation passed
	IsAllowed() bool
	// GetReason returns the reason for denial (if denied)
	GetReason() string
	// GetMessage returns the detailed message
	GetMessage() string
	// GetViolations returns all validation violations
	GetViolations() []Violation
}

// Violation represents a single validation violation
type Violation struct {
	// Expression that failed
	Expression string
	// Reason for the failure
	Reason string
	// Message describing the failure
	Message string
}

// validationResult is the default implementation of ValidationResult
type validationResult struct {
	allowed    bool
	violations []Violation
}

// NewValidationResult creates a new validation result
func NewValidationResult(allowed bool, violations []Violation) ValidationResult {
	return &validationResult{
		allowed:    allowed,
		violations: violations,
	}
}

// IsAllowed returns true if the validation passed
func (r *validationResult) IsAllowed() bool {
	return r.allowed
}

// GetReason returns the reason for denial
func (r *validationResult) GetReason() string {
	if r.allowed || len(r.violations) == 0 {
		return ""
	}

	if len(r.violations) == 1 {
		return r.violations[0].Reason
	}

	// Multiple violations
	reasonMap := make(map[string]bool)
	for _, v := range r.violations {
		if v.Reason != "" && v.Reason != "FailedValidation" {
			reasonMap[v.Reason] = true
		}
	}

	// If all violations have the same reason, use that
	if len(reasonMap) == 1 {
		for reason := range reasonMap {
			return reason
		}
	}

	return "MultipleViolations"
}

// GetMessage returns the detailed message
func (r *validationResult) GetMessage() string {
	if r.allowed || len(r.violations) == 0 {
		return ""
	}

	if len(r.violations) == 1 {
		return r.violations[0].Message
	}

	// Multiple violations - combine messages
	var messages []string
	for _, v := range r.violations {
		msg := v.Message
		if v.Reason != "" && v.Reason != "FailedValidation" {
			msg = fmt.Sprintf("[%s] %s", v.Reason, msg)
		}
		messages = append(messages, msg)
	}

	return fmt.Sprintf("Multiple validation failures (%d):\n%s",
		len(r.violations), strings.Join(messages, "\n"))
}

// GetViolations returns all validation violations
func (r *validationResult) GetViolations() []Violation {
	return r.violations
}