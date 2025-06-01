package errors

import (
	"fmt"
	"strings"
)

// UserError provides user-friendly error messages
type UserError struct {
	Message    string   // User-facing message
	Detail     string   // Detailed technical information
	Suggestion string   // Suggestion for resolution
	Cause      error    // Original error
}

// Error implements the error interface
func (e *UserError) Error() string {
	var parts []string
	parts = append(parts, e.Message)
	
	if e.Detail != "" {
		parts = append(parts, fmt.Sprintf("Details: %s", e.Detail))
	}
	
	if e.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("Resolution: %s", e.Suggestion))
	}
	
	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("Cause: %v", e.Cause))
	}
	
	return strings.Join(parts, "\n")
}

// Unwrap returns the original error
func (e *UserError) Unwrap() error {
	return e.Cause
}

// NewUserError creates a new user error
func NewUserError(message string, cause error) *UserError {
	return &UserError{
		Message: message,
		Cause:   cause,
	}
}

// WithDetail adds detailed information
func (e *UserError) WithDetail(detail string) *UserError {
	e.Detail = detail
	return e
}

// WithSuggestion adds a resolution suggestion
func (e *UserError) WithSuggestion(suggestion string) *UserError {
	e.Suggestion = suggestion
	return e
}

// WrapWithContext wraps an error with context information
func WrapWithContext(err error, context string) error {
	if err == nil {
		return nil
	}
	
	// If it's already a UserError, add context
	if userErr, ok := err.(*UserError); ok {
		userErr.Message = fmt.Sprintf("%s: %s", context, userErr.Message)
		return userErr
	}
	
	// For regular errors, create a new UserError
	return &UserError{
		Message: context,
		Cause:   err,
	}
}

// CommonErrors provides helper functions for common error cases
var CommonErrors = struct {
	FileNotFound      func(path string) *UserError
	InvalidYAML       func(path string, err error) *UserError
	PolicyNotFound    func(name string) *UserError
	ClusterConnection func(err error) *UserError
	ResourceNotFound  func(kind, name string) *UserError
}{
	FileNotFound: func(path string) *UserError {
		return &UserError{
			Message:    fmt.Sprintf("File not found: %s", path),
			Suggestion: "Please verify that the file path is correct",
		}
	},
	
	InvalidYAML: func(path string, err error) *UserError {
		return &UserError{
			Message:    fmt.Sprintf("Failed to parse YAML file: %s", path),
			Detail:     "Please ensure the file is in valid YAML format",
			Suggestion: "Use tools like yamllint to validate the file",
			Cause:      err,
		}
	},
	
	PolicyNotFound: func(name string) *UserError {
		return &UserError{
			Message:    fmt.Sprintf("Policy '%s' not found", name),
			Suggestion: "Please verify the policy name is correct and that the policy file is specified",
		}
	},
	
	ClusterConnection: func(err error) *UserError {
		return &UserError{
			Message:    "Cannot connect to Kubernetes cluster",
			Detail:     "Please ensure your kubeconfig file is properly configured",
			Suggestion: "Check cluster status with kubectl cluster-info",
			Cause:      err,
		}
	},
	
	ResourceNotFound: func(kind, name string) *UserError {
		return &UserError{
			Message:    fmt.Sprintf("%s '%s' not found", kind, name),
			Suggestion: fmt.Sprintf("Check resource existence with kubectl get %s %s", strings.ToLower(kind), name),
		}
	},
}