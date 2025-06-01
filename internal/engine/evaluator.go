package engine

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"github.com/yashirook/kube-vap-test/internal/engine/admission"
	celenv "github.com/yashirook/kube-vap-test/internal/engine/cel"
	"github.com/yashirook/kube-vap-test/internal/engine/selector"
)

// CELEvaluator is an interface for evaluating CEL expressions
type CELEvaluator interface {
	EvaluateExpression(expression string, variables map[string]interface{}) (bool, error)
}

// DefaultCELEvaluator is the default implementation for evaluating CEL expressions
type DefaultCELEvaluator struct {
	celEnv *cel.Env
}

// NewDefaultCELEvaluator creates a new DefaultCELEvaluator
func NewDefaultCELEvaluator() (*DefaultCELEvaluator, error) {
	// Use the same environment as the main evaluator
	env, err := celenv.DefaultEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &DefaultCELEvaluator{
		celEnv: env,
	}, nil
}

// EvaluateExpression evaluates a CEL expression
// This method is primarily used for testing and supports dynamic variable names
func (e *DefaultCELEvaluator) EvaluateExpression(expression string, variables map[string]interface{}) (bool, error) {
	if variables == nil {
		variables = map[string]interface{}{}
	}

	// For testing purposes, create a new CEL environment with dynamic variables
	opts := []cel.EnvOption{cel.OptionalTypes()}
	
	// Create a new CEL environment that can handle any variables
	env, err := cel.NewEnv(opts...)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Parse the expression without variable declarations
	// This allows us to evaluate expressions with any variable names
	ast, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("failed to parse expression: %w", issues.Err())
	}

	// Create program
	program, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("failed to create program: %w", err)
	}

	// Evaluate expression
	eval, _, err := program.Eval(variables)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	// Convert to boolean value
	boolResult, ok := eval.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression result is not a boolean: %v", eval.Value())
	}

	return boolResult, nil
}

// EvaluatePolicy evaluates a ValidatingAdmissionPolicy
func (e *DefaultCELEvaluator) EvaluatePolicy(policy *admissionregistrationv1.ValidatingAdmissionPolicy, object map[string]interface{}) (bool, error) {
	if policy == nil {
		return false, fmt.Errorf("policy is nil")
	}

	if policy.Spec.Validations == nil || len(policy.Spec.Validations) == 0 {
		return true, nil
	}

	// Variables used for evaluation
	variables := map[string]interface{}{
		"object": object,
	}

	// Evaluate all rules
	for _, rule := range policy.Spec.Validations {
		if rule.Expression == "" {
			continue
		}

		result, err := e.EvaluateExpression(rule.Expression, variables)
		if err != nil {
			return false, err
		}

		if !result {
			return false, nil
		}
	}

	return true, nil
}

// EvaluatePolicyWithBinding evaluates a ValidatingAdmissionPolicy and Binding
func (e *DefaultCELEvaluator) EvaluatePolicyWithBinding(policy *admissionregistrationv1.ValidatingAdmissionPolicy, binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding, object map[string]interface{}) (bool, string, error) {
	if policy == nil {
		return false, "policy is nil", fmt.Errorf("policy is nil")
	}

	if binding == nil {
		return false, "binding is nil", fmt.Errorf("binding is nil")
	}

	// Check if binding policy name matches policy name
	if binding.Spec.PolicyName != policy.Name {
		return false, fmt.Sprintf("binding policy name %s does not match policy name %s", binding.Spec.PolicyName, policy.Name), fmt.Errorf("binding policy name %s does not match policy name %s", binding.Spec.PolicyName, policy.Name)
	}

	// 1. First evaluate VAP matching conditions
	policyMatches, err := e.MatchesPolicy(policy, object)
	if err != nil {
		return false, fmt.Sprintf("policy matching evaluation error: %v", err), err
	}

	if !policyMatches {
		return true, "resource does not match policy matching conditions, skipping evaluation", nil
	}

	// 2. Evaluate binding matching
	matcher := selector.NewDefaultMatcher()
	admissionTarget := admission.AdmissionTarget{
		Object: object,
		// Note: other resource info fields are empty as they are not provided by current API
	}

	// Prepare convenience fields
	admissionTarget.PrepareObject()

	bindingMatches, err := matcher.Matches(binding.Spec.MatchResources, admissionTarget)
	if err != nil {
		return false, fmt.Sprintf("selector evaluation error: %v", err), err
	}

	if !bindingMatches {
		return true, "resource does not match binding selector, skipping evaluation", nil
	}

	// 3. Evaluate policy validation rules
	allowed, err := e.evaluatePolicyOnly(policy, object)
	if err != nil {
		return false, fmt.Sprintf("policy evaluation error: %v", err), err
	}

	// If not allowed, check binding's ValidationActions
	if !allowed {
		// If ValidationActions is not empty and has Deny action, deny
		if binding.Spec.ValidationActions != nil {
			for _, action := range binding.Spec.ValidationActions {
				if action == admissionregistrationv1.Deny {
					return false, "policy violation: deny", nil
				}
			}
			// If no Deny action, allow with warning
			return true, "Policy violation detected but allowed as warning only", nil
		}
		// If ValidationActions is empty, deny
		return false, "Policy violation detected", nil
	}

	return true, "", nil
}

// evaluatePolicyOnly evaluates only the policy (internal use)
func (e *DefaultCELEvaluator) evaluatePolicyOnly(policy *admissionregistrationv1.ValidatingAdmissionPolicy, object map[string]interface{}) (bool, error) {
	if policy.Spec.Validations == nil || len(policy.Spec.Validations) == 0 {
		return true, nil
	}

	// Variables used for evaluation
	variables := map[string]interface{}{
		"object": object,
	}

	// Evaluate all rules
	for _, rule := range policy.Spec.Validations {
		if rule.Expression == "" {
			continue
		}

		// Improved error handling for non-existent keys
		result, err := e.EvaluateExpression(rule.Expression, variables)
		if err != nil {
			// Analyze error message and return false if accessing non-existent key
			if strings.Contains(err.Error(), "no such key") {
				// Treat access to non-existent key as validation failure
				return false, nil
			}
			return false, err
		}

		if !result {
			return false, nil
		}
	}

	return true, nil
}

// MatchesPolicy evaluates the matching conditions of ValidatingAdmissionPolicy
func (e *DefaultCELEvaluator) MatchesPolicy(policy *admissionregistrationv1.ValidatingAdmissionPolicy, object map[string]interface{}) (bool, error) {
	if policy == nil {
		return false, fmt.Errorf("policy is nil")
	}

	// Build AdmissionTarget
	admissionTarget := admission.AdmissionTarget{
		Object: object,
	}

	// Get operation and resource information from object
	if operation, ok := object["operation"].(string); ok {
		admissionTarget.Operation = operation
	}

	if resource, ok := object["resource"].(map[string]interface{}); ok {
		if group, ok := resource["group"].(string); ok {
			admissionTarget.APIGroup = group
		}
		if version, ok := resource["version"].(string); ok {
			admissionTarget.APIVersion = version
		}
		if resourceType, ok := resource["resource"].(string); ok {
			admissionTarget.Resource = resourceType
		}
	} else {
		// If no resource, infer from apiVersion and kind
		if apiVersion, ok := object["apiVersion"].(string); ok {
			parts := strings.Split(apiVersion, "/")
			if len(parts) == 2 {
				admissionTarget.APIGroup = parts[0]
				admissionTarget.APIVersion = parts[1]
			} else {
				admissionTarget.APIGroup = ""
				admissionTarget.APIVersion = apiVersion
			}
		}

		if kind, ok := object["kind"].(string); ok {
			// Convert kind to resource name (lowercase plural)
			admissionTarget.Resource = strings.ToLower(kind) + "s"
		}
	}

	// Removed: convenience field preparation is unnecessary

	// 1. Evaluate MatchConstraints
	if policy.Spec.MatchConstraints != nil {
		matcher := selector.NewDefaultMatcher()
		matchConstraintsResult, err := matcher.Matches(policy.Spec.MatchConstraints, admissionTarget)
		if err != nil {
			return false, fmt.Errorf("MatchConstraints evaluation error: %w", err)
		}

		if !matchConstraintsResult {
			return false, nil
		}
	}

	// 2. Evaluate MatchConditions
	if policy.Spec.MatchConditions != nil && len(policy.Spec.MatchConditions) > 0 {
		matchConditionsResult, err := e.evaluateMatchConditions(policy.Spec.MatchConditions, object)
		if err != nil {
			return false, fmt.Errorf("MatchConditions evaluation error: %w", err)
		}

		if !matchConditionsResult {
			return false, nil
		}
	}

	// If all conditions are satisfied
	return true, nil
}

// evaluateMatchConditions evaluates MatchCondition (CEL expressions)
func (e *DefaultCELEvaluator) evaluateMatchConditions(conditions []admissionregistrationv1.MatchCondition, object map[string]interface{}) (bool, error) {
	// Variables used for evaluation
	variables := map[string]interface{}{
		"object": object,
	}

	// Evaluate all conditions
	for _, condition := range conditions {
		if condition.Expression == "" {
			continue
		}

		result, err := e.EvaluateExpression(condition.Expression, variables)
		if err != nil {
			// Treat specific errors as non-matching
			if strings.Contains(err.Error(), "no such key") {
				return false, nil
			}
			return false, err
		}

		if !result {
			return false, nil
		}
	}

	return true, nil
}
