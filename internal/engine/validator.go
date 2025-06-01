package engine

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/yashirook/kube-vap-test/internal/engine/cel"
)

// PolicyValidator validates objects against policies
type PolicyValidator struct {
	celEvaluator *cel.Evaluator
	contextVars  map[string]interface{}
}

// NewPolicyValidator creates a new policy validator
func NewPolicyValidator() (*PolicyValidator, error) {
	evaluator, err := cel.NewEvaluator()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL evaluator: %w", err)
	}

	return &PolicyValidator{
		celEvaluator: evaluator,
		contextVars:  make(map[string]interface{}),
	}, nil
}

// SetContextVariables sets the context variables for evaluation
func (v *PolicyValidator) SetContextVariables(vars map[string]interface{}) {
	v.contextVars = vars
}

// ValidatePolicy validates an object against a policy
func (v *PolicyValidator) ValidatePolicy(
	ctx context.Context,
	policy *admissionregistrationv1.ValidatingAdmissionPolicy,
	collectAllViolations bool,
) ValidationResult {
	// Check matchConditions first if they exist
	if len(policy.Spec.MatchConditions) > 0 {
		matches, err := v.evaluateMatchConditions(policy.Spec.MatchConditions)
		if err != nil {
			return NewValidationResult(false, []Violation{{
				Reason:  "MatchConditionEvaluationError",
				Message: fmt.Sprintf("Failed to evaluate matchConditions: %s", err.Error()),
			}})
		}
		if !matches {
			// Policy doesn't apply, allow the object
			return NewValidationResult(true, nil)
		}
	}
	if policy.Spec.Validations == nil || len(policy.Spec.Validations) == 0 {
		// Allow if no validation expressions
		return NewValidationResult(true, nil)
	}

	// Evaluate variables first if they exist
	variableValues := make(map[string]interface{})
	if len(policy.Spec.Variables) > 0 {
		var err error
		variableValues, err = v.evaluateVariables(policy)
		if err != nil {
			return NewValidationResult(false, []Violation{{
				Reason:  "VariableEvaluationError",
				Message: fmt.Sprintf("Variable evaluation error: %s", err.Error()),
			}})
		}
	}

	// Evaluate each validation expression
	var violations []Violation
	for _, validation := range policy.Spec.Validations {
		result, err := v.evaluateValidation(validation, variableValues)
		if err != nil {
			violations = append(violations, Violation{
				Expression: validation.Expression,
				Reason:     "FailedValidation",
				Message:    fmt.Sprintf("Expression evaluation error: %s", err.Error()),
			})
			if !collectAllViolations {
				break
			}
			continue
		}

		// Check evaluation result
		allowed, ok := result.(bool)
		if !ok {
			violations = append(violations, Violation{
				Expression: validation.Expression,
				Reason:     "FailedValidation",
				Message:    fmt.Sprintf("Expression did not return a boolean: %v (type: %s)", result, reflect.TypeOf(result)),
			})
			if !collectAllViolations {
				break
			}
			continue
		}

		if !allowed {
			// Validation failed
			reason := "FailedValidation"
			if validation.Reason != nil {
				reason = string(*validation.Reason)
			}

			// Evaluate messageExpression if present, otherwise use static message
			message, err := v.evaluateMessage(validation, variableValues)
			if err != nil {
				// If messageExpression evaluation fails, fall back to static message
				// Note: verbose logging should be handled by the caller
				message = validation.Message
				if message == "" {
					message = fmt.Sprintf("failed expression: %s", validation.Expression)
				}
			}

			violations = append(violations, Violation{
				Expression: validation.Expression,
				Reason:     reason,
				Message:    message,
			})

			if !collectAllViolations {
				break
			}
		}
	}

	// Determine final result
	allowed := len(violations) == 0
	return NewValidationResult(allowed, violations)
}

// evaluateVariables evaluates all variables defined in the policy
func (v *PolicyValidator) evaluateVariables(policy *admissionregistrationv1.ValidatingAdmissionPolicy) (map[string]interface{}, error) {
	if len(policy.Spec.Variables) == 0 {
		return nil, nil
	}

	// Variables can reference other variables defined earlier, so we evaluate them in order
	variableValues := make(map[string]interface{})
	
	for _, variable := range policy.Spec.Variables {
		// Prepare evaluation variables including context vars and previously evaluated variables
		evalVars := make(map[string]interface{})
		
		// Copy context variables
		for k, val := range v.contextVars {
			evalVars[k] = val
		}
		
		// Add variables namespace with previously evaluated variables
		evalVars["variables"] = variableValues

		// Evaluate the variable expression
		result, err := v.celEvaluator.Evaluate(variable.Expression, evalVars)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate variable %s: %w", variable.Name, err)
		}

		// Store the evaluated value
		variableValues[variable.Name] = result
	}

	return variableValues, nil
}

// evaluateValidation evaluates a single validation expression
func (v *PolicyValidator) evaluateValidation(
	validation admissionregistrationv1.Validation,
	variableValues map[string]interface{},
) (interface{}, error) {
	// Prepare evaluation variables
	evalVars := make(map[string]interface{})
	
	// Copy context variables
	for k, val := range v.contextVars {
		evalVars[k] = val
	}
	
	// Add variables namespace if available
	if variableValues != nil {
		evalVars["variables"] = variableValues
	}

	// Evaluate expression
	return v.celEvaluator.Evaluate(validation.Expression, evalVars)
}

// SetupEvaluationContext sets up the evaluation context from objects
func (v *PolicyValidator) SetupEvaluationContext(
	reqObj *unstructured.Unstructured,
	oldObj *unstructured.Unstructured,
	paramObj runtime.Object,
	operation string,
) error {
	// Initialize context variables
	v.contextVars = make(map[string]interface{})

	// Convert object to map
	objectMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(reqObj)
	if err != nil {
		return fmt.Errorf("failed to convert object to map: %w", err)
	}

	// Set object in context variables
	v.contextVars["object"] = objectMap
	v.contextVars["operation"] = operation

	// Old object (for update operations)
	if oldObj != nil {
		oldObjectMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(oldObj)
		if err != nil {
			return fmt.Errorf("failed to convert old object to map: %w", err)
		}
		v.contextVars["oldObject"] = oldObjectMap
	}

	// Parameter object
	if paramObj != nil {
		paramMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(paramObj)
		if err != nil {
			return fmt.Errorf("failed to convert parameter object to map: %w", err)
		}
		v.contextVars["params"] = paramMap
	}

	return nil
}

// evaluateMessage evaluates the message for a validation failure
// It first tries messageExpression if present, otherwise falls back to static message
func (v *PolicyValidator) evaluateMessage(validation admissionregistrationv1.Validation, variableValues map[string]interface{}) (string, error) {
	// If messageExpression is provided, evaluate it
	if validation.MessageExpression != "" {
		// Set up evaluation variables
		evalVars := make(map[string]interface{})
		
		// Copy context variables (excluding authorizer as per spec)
		for k, val := range v.contextVars {
			if k != "authorizer" && k != "authorizer.requestResource" {
				evalVars[k] = val
			}
		}
		
		// Add variables namespace if available
		if variableValues != nil {
			evalVars["variables"] = variableValues
		}

		// Evaluate messageExpression
		result, err := v.celEvaluator.Evaluate(validation.MessageExpression, evalVars)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate messageExpression: %w", err)
		}

		// Ensure result is a string
		message, ok := result.(string)
		if !ok {
			return "", fmt.Errorf("messageExpression must evaluate to a string, got %T", result)
		}

		// Check if message is empty, contains only spaces, or has line breaks
		trimmed := strings.TrimSpace(message)
		if trimmed == "" || strings.Contains(message, "\n") {
			return "", fmt.Errorf("messageExpression produced invalid message: empty, whitespace-only, or contains line breaks")
		}

		return message, nil
	}

	// Fall back to static message
	if validation.Message != "" {
		return validation.Message, nil
	}

	// Default message
	return fmt.Sprintf("failed expression: %s", validation.Expression), nil
}

// evaluateMatchConditions evaluates all matchConditions for a policy
func (v *PolicyValidator) evaluateMatchConditions(conditions []admissionregistrationv1.MatchCondition) (bool, error) {
	for _, condition := range conditions {
		// Evaluate the condition expression
		result, err := v.celEvaluator.Evaluate(condition.Expression, v.contextVars)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate matchCondition %s: %w", condition.Name, err)
		}

		// Check if result is boolean
		matches, ok := result.(bool)
		if !ok {
			return false, fmt.Errorf("matchCondition %s did not return a boolean: %v (type: %s)", condition.Name, result, reflect.TypeOf(result))
		}

		// All conditions must match
		if !matches {
			return false, nil
		}
	}

	return true, nil
}