package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/yashirook/kube-vap-test/internal/engine/admission"
	"github.com/yashirook/kube-vap-test/internal/engine/selector"
	kaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
)

// PolicySimulator executes policy simulations
type PolicySimulator struct {
	validator *PolicyValidator
	evaluator *DefaultCELEvaluator
}

// NewPolicySimulator creates a new PolicySimulator
func NewPolicySimulator() (*PolicySimulator, error) {
	validator, err := NewPolicyValidator()
	if err != nil {
		return nil, fmt.Errorf("failed to create policy validator: %w", err)
	}

	evaluator, err := NewDefaultCELEvaluator()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL evaluator: %w", err)
	}

	return &PolicySimulator{
		validator: validator,
		evaluator: evaluator,
	}, nil
}

// SetContextVariables sets context variables (for backward compatibility)
func (p *PolicySimulator) SetContextVariables(vars map[string]interface{}) {
	p.validator.SetContextVariables(vars)
}

// SimulateTestCase simulates a single test case
func (p *PolicySimulator) SimulateTestCase(
	ctx context.Context,
	policy *admissionregistrationv1.ValidatingAdmissionPolicy,
	paramObj runtime.Object,
	testCase kaptestv1.TestCase,
) (*kaptestv1.TestResult, error) {
	// Initialize test result
	result := &kaptestv1.TestResult{
		Name:    testCase.Name,
		Success: false,
	}

	// Convert object
	reqObj, err := p.convertRawExtension(testCase.Object)
	if err != nil {
		return result, fmt.Errorf("failed to convert object: %w", err)
	}

	var oldObj *unstructured.Unstructured
	if testCase.OldObject != nil {
		oldObj, err = p.convertRawExtension(*testCase.OldObject)
		if err != nil {
			return result, fmt.Errorf("failed to convert old object: %w", err)
		}
	}

	// Setup evaluation context
	if err := p.validator.SetupEvaluationContext(reqObj, oldObj, paramObj, testCase.Operation); err != nil {
		return result, fmt.Errorf("failed to set up evaluation context: %w", err)
	}

	// Check if policy matches the object
	objectMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(reqObj)
	if err != nil {
		return result, fmt.Errorf("failed to convert object to map: %w", err)
	}

	matches, err := p.evaluator.MatchesPolicy(policy, objectMap)
	if err != nil {
		return result, fmt.Errorf("failed to check policy match: %w", err)
	}

	// If policy doesn't match, allow the object (policy is not applicable)
	var validationResult ValidationResult
	if !matches {
		result.ActualResponse = &kaptestv1.ResponseDetails{
			Allowed: true,
			Reason:  "",
			Message: "",
		}
		validationResult = NewValidationResult(true, nil)
	} else {
		// Validate policy
		validationResult = p.validator.ValidatePolicy(ctx, policy, true)

		// Set actual response
		result.ActualResponse = &kaptestv1.ResponseDetails{
			Allowed: validationResult.IsAllowed(),
			Reason:  validationResult.GetReason(),
			Message: validationResult.GetMessage(),
		}
	}

	// Compare with expected result
	if validationResult.IsAllowed() == testCase.Expected.Allowed {
		// Allow/deny result matches
		if !validationResult.IsAllowed() {
			// For deny, also check reason and message
			reasonMatch := testCase.Expected.Reason == "" || 
				testCase.Expected.Reason == validationResult.GetReason() || 
				strings.Contains(validationResult.GetReason(), testCase.Expected.Reason)
			
			messageMatch := true
			if testCase.Expected.Message != "" {
				messageMatch = testCase.Expected.Message == validationResult.GetMessage() || 
					strings.Contains(validationResult.GetMessage(), testCase.Expected.Message)
			}
			if testCase.Expected.MessageContains != "" {
				messageMatch = messageMatch && strings.Contains(validationResult.GetMessage(), testCase.Expected.MessageContains)
			}

			if reasonMatch && messageMatch {
				result.Success = true
			} else {
				result.Details = fmt.Sprintf(
					"actual response does not match expected. Expected: (reason=%s, message=%s, messageContains=%s), Actual: (reason=%s, message=%s)",
					testCase.Expected.Reason,
					testCase.Expected.Message,
					testCase.Expected.MessageContains,
					validationResult.GetReason(),
					validationResult.GetMessage(),
				)
			}
		} else {
			result.Success = true
		}
	} else {
		result.Success = false
		result.Details = fmt.Sprintf(
			"expected result (allowed=%t) and actual result (allowed=%t) does not match",
			testCase.Expected.Allowed,
			validationResult.IsAllowed(),
		)
	}

	return result, nil
}

// SimulateWithPolicyBindings simulates test cases with multiple policies and bindings
func (p *PolicySimulator) SimulateWithPolicyBindings(
	ctx context.Context,
	policies []*admissionregistrationv1.ValidatingAdmissionPolicy,
	bindings []*admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	paramObj runtime.Object,
	testCase kaptestv1.TestCase,
) (*kaptestv1.TestResult, error) {
	// Initialize test result
	result := &kaptestv1.TestResult{
		Name:    testCase.Name,
		Success: false,
	}

	// Convert object
	reqObj, err := p.convertRawExtension(testCase.Object)
	if err != nil {
		return result, fmt.Errorf("failed to convert object: %w", err)
	}

	var oldObj *unstructured.Unstructured
	if testCase.OldObject != nil {
		oldObj, err = p.convertRawExtension(*testCase.OldObject)
		if err != nil {
			return result, fmt.Errorf("failed to convert old object: %w", err)
		}
	}

	// Create evaluation context
	if err := p.validator.SetupEvaluationContext(reqObj, oldObj, paramObj, testCase.Operation); err != nil {
		return result, fmt.Errorf("failed to set up evaluation context: %w", err)
	}

	// Initialize policy results
	policyResults := make([]kaptestv1.PolicyResult, 0, len(policies))
	finalAllowed := true
	finalReason := ""
	finalMessage := ""

	// Create policy and binding mapping
	policyBindings := make(map[string][]*admissionregistrationv1.ValidatingAdmissionPolicyBinding)
	for _, binding := range bindings {
		policyName := binding.Spec.PolicyName
		policyBindings[policyName] = append(policyBindings[policyName], binding)
	}

	// Evaluate each policy
	for _, policy := range policies {
		// Check if policy has bindings
		relatedBindings, hasBindings := policyBindings[policy.Name]
		
		// If no bindings, evaluate policy directly
		if !hasBindings || len(relatedBindings) == 0 {
			validationResult := p.validator.ValidatePolicy(ctx, policy, true)
			
			policyResult := kaptestv1.PolicyResult{
				PolicyName: policy.Name,
				Allowed:    validationResult.IsAllowed(),
				Reason:     validationResult.GetReason(),
				Message:    validationResult.GetMessage(),
			}
			policyResults = append(policyResults, policyResult)

			if !validationResult.IsAllowed() {
				finalAllowed = false
				if finalReason == "" {
					finalReason = validationResult.GetReason()
				}
				if finalMessage == "" {
					finalMessage = validationResult.GetMessage()
				} else {
					finalMessage = fmt.Sprintf("%s; %s", finalMessage, validationResult.GetMessage())
				}
			}
			continue
		}

		// Evaluate policy with bindings
		bindingMatched := false
		for _, binding := range relatedBindings {
			// Check if binding matches the object
			if !p.matchesBinding(binding, reqObj, testCase.Operation) {
				continue
			}
			bindingMatched = true

			// Check validation action
			if !p.shouldValidate(binding.Spec.ValidationActions) {
				continue
			}

			// Evaluate policy
			validationResult := p.validator.ValidatePolicy(ctx, policy, true)
			
			policyResult := kaptestv1.PolicyResult{
				PolicyName: policy.Name,
				Allowed:    validationResult.IsAllowed(),
				Reason:     validationResult.GetReason(),
				Message:    validationResult.GetMessage(),
			}
			policyResults = append(policyResults, policyResult)

			if !validationResult.IsAllowed() {
				finalAllowed = false
				if finalReason == "" {
					finalReason = validationResult.GetReason()
				}
				if finalMessage == "" {
					finalMessage = validationResult.GetMessage()
				} else {
					finalMessage = fmt.Sprintf("%s; %s", finalMessage, validationResult.GetMessage())
				}
			}
			break // Only evaluate once per policy
		}

		// If no binding matched, skip this policy
		if !bindingMatched {
			continue
		}
	}

	// Set final result
	result.PolicyResults = policyResults
	result.ActualResponse = &kaptestv1.ResponseDetails{
		Allowed: finalAllowed,
		Reason:  finalReason,
		Message: finalMessage,
	}

	// Compare with expected result
	if finalAllowed == testCase.Expected.Allowed {
		// Allow/deny result matches
		if !finalAllowed {
			// For deny, also check reason and message
			reasonMatch := testCase.Expected.Reason == "" || testCase.Expected.Reason == finalReason || strings.Contains(finalReason, testCase.Expected.Reason)
			messageMatch := true

			if testCase.Expected.Message != "" {
				messageMatch = testCase.Expected.Message == finalMessage || strings.Contains(finalMessage, testCase.Expected.Message)
			}
			if testCase.Expected.MessageContains != "" {
				messageMatch = messageMatch && strings.Contains(finalMessage, testCase.Expected.MessageContains)
			}

			if reasonMatch && messageMatch {
				result.Success = true
			} else {
				result.Details = fmt.Sprintf(
					"actual response does not match expected. Expected: (reason=%s, message=%s, messageContains=%s), Actual: (reason=%s, message=%s)",
					testCase.Expected.Reason,
					testCase.Expected.Message,
					testCase.Expected.MessageContains,
					finalReason,
					finalMessage,
				)
			}
		} else {
			result.Success = true
		}
	} else {
		result.Success = false
		result.Details = fmt.Sprintf(
			"expected result (allowed=%t) and actual result (allowed=%t) does not match",
			testCase.Expected.Allowed,
			finalAllowed,
		)
	}

	return result, nil
}

// convertRawExtension converts a RawExtension to an Unstructured object
func (p *PolicySimulator) convertRawExtension(raw runtime.RawExtension) (*unstructured.Unstructured, error) {
	// Parse the object
	obj := &unstructured.Unstructured{}
	
	// If raw JSON is provided, parse it
	if len(raw.Raw) > 0 {
		if err := json.Unmarshal(raw.Raw, obj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw extension: %w", err)
		}
		return obj, nil
	}

	// If object is provided, convert it
	if raw.Object != nil {
		// If it's already an Unstructured, return it
		if u, ok := raw.Object.(*unstructured.Unstructured); ok {
			return u, nil
		}

		// Otherwise convert it
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(raw.Object)
		if err != nil {
			return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
		}
		obj.SetUnstructuredContent(content)
		return obj, nil
	}

	return nil, fmt.Errorf("raw extension has neither raw nor object")
}

// matchesBinding checks if a binding matches the object
func (p *PolicySimulator) matchesBinding(
	binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	obj *unstructured.Unstructured,
	operation string,
) bool {
	if binding.Spec.MatchResources == nil {
		return true
	}

	target := admission.NewAdmissionTarget(obj, operation)
	return selector.Matches(binding.Spec.MatchResources, target)
}

// shouldValidate checks if validation should be performed based on validation actions
func (p *PolicySimulator) shouldValidate(actions []admissionregistrationv1.ValidationAction) bool {
	// If no actions specified, default to Deny
	if len(actions) == 0 {
		return true
	}

	// Check if Deny action is present
	for _, action := range actions {
		if action == admissionregistrationv1.Deny {
			return true
		}
	}

	// Only Warn or Audit actions, skip validation
	return false
}

// RunPolicyTests executes policy tests
func (p *PolicySimulator) RunPolicyTests(
	ctx context.Context,
	policy *admissionregistrationv1.ValidatingAdmissionPolicy,
	paramObj runtime.Object,
	testCases []kaptestv1.TestCase,
) (*kaptestv1.ValidatingAdmissionPolicyTestStatus, error) {
	status := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: make([]kaptestv1.TestResult, 0, len(testCases)),
		Summary: kaptestv1.TestSummary{
			Total:      len(testCases),
			Successful: 0,
			Failed:     0,
		},
	}

	for _, testCase := range testCases {
		result, err := p.SimulateTestCase(ctx, policy, paramObj, testCase)
		if err != nil {
			return nil, fmt.Errorf("failed to execute test case '%s': %w", testCase.Name, err)
		}

		status.Results = append(status.Results, *result)

		if result.Success {
			status.Summary.Successful++
		} else {
			status.Summary.Failed++
		}
	}

	return status, nil
}

// RunPolicyTestsWithBindings executes tests with policies and bindings
func (p *PolicySimulator) RunPolicyTestsWithBindings(
	ctx context.Context,
	policies []*admissionregistrationv1.ValidatingAdmissionPolicy,
	bindings []*admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	paramObj runtime.Object,
	testCases []kaptestv1.TestCase,
) (*kaptestv1.ValidatingAdmissionPolicyTestStatus, error) {
	status := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: make([]kaptestv1.TestResult, 0, len(testCases)),
		Summary: kaptestv1.TestSummary{
			Total:      len(testCases),
			Successful: 0,
			Failed:     0,
		},
	}

	for _, testCase := range testCases {
		result, err := p.SimulateWithPolicyBindings(ctx, policies, bindings, paramObj, testCase)
		if err != nil {
			return nil, fmt.Errorf("failed to execute test case '%s': %w", testCase.Name, err)
		}

		status.Results = append(status.Results, *result)

		if result.Success {
			status.Summary.Successful++
		} else {
			status.Summary.Failed++
		}
	}

	return status, nil
}

// RunPolicyTestsWithMultiPolicies executes tests with multiple policies
func (p *PolicySimulator) RunPolicyTestsWithMultiPolicies(
	ctx context.Context,
	policies []*admissionregistrationv1.ValidatingAdmissionPolicy,
	paramObj runtime.Object,
	testCases []kaptestv1.TestCase,
) (*kaptestv1.ValidatingAdmissionPolicyTestStatus, error) {
	// This is a wrapper that creates empty bindings for backward compatibility
	bindings := []*admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
	return p.RunPolicyTestsWithBindings(ctx, policies, bindings, paramObj, testCases)
}

// SimulateTestCaseWithMultiPolicies simulates a single test case with multiple policies
func (p *PolicySimulator) SimulateTestCaseWithMultiPolicies(
	ctx context.Context,
	policies []*admissionregistrationv1.ValidatingAdmissionPolicy,
	paramObj runtime.Object,
	testCase kaptestv1.TestCase,
) (*kaptestv1.TestResult, error) {
	// Initialize test result
	result := &kaptestv1.TestResult{
		Name:          testCase.Name,
		Success:       false,
		PolicyResults: make([]kaptestv1.PolicyResult, 0, len(policies)),
	}

	// Convert object
	reqObj, err := p.convertRawExtension(testCase.Object)
	if err != nil {
		return result, fmt.Errorf("failed to convert object: %w", err)
	}

	var oldObj *unstructured.Unstructured
	if testCase.OldObject != nil {
		oldObj, err = p.convertRawExtension(*testCase.OldObject)
		if err != nil {
			return result, fmt.Errorf("failed to convert old object: %w", err)
		}
	}

	// Create evaluation context
	if err := p.validator.SetupEvaluationContext(reqObj, oldObj, paramObj, testCase.Operation); err != nil {
		return result, fmt.Errorf("failed to set up evaluation context: %w", err)
	}

	// Evaluate all policies
	finalAllowed := true
	var finalReason, finalMessage string

	for _, policy := range policies {
		// Evaluate each policy
		validationResult := p.validator.ValidatePolicy(ctx, policy, true)

		// Record individual policy result
		policyResult := kaptestv1.PolicyResult{
			PolicyName: policy.Name,
			Allowed:    validationResult.IsAllowed(),
			Reason:     validationResult.GetReason(),
			Message:    validationResult.GetMessage(),
		}
		result.PolicyResults = append(result.PolicyResults, policyResult)

		// If any policy denies, overall deny
		if !validationResult.IsAllowed() {
			finalAllowed = false
			if finalReason == "" {
				finalReason = validationResult.GetReason()
			}
			if finalMessage == "" {
				finalMessage = validationResult.GetMessage()
			} else {
				finalMessage = fmt.Sprintf("%s; %s", finalMessage, validationResult.GetMessage())
			}
		}
	}

	// Set final result
	result.ActualResponse = &kaptestv1.ResponseDetails{
		Allowed: finalAllowed,
		Reason:  finalReason,
		Message: finalMessage,
	}

	// Compare with expected result
	if finalAllowed == testCase.Expected.Allowed {
		// Allow/deny result matches
		if !finalAllowed {
			// For deny, also check reason and message
			reasonMatch := testCase.Expected.Reason == "" || testCase.Expected.Reason == finalReason || strings.Contains(finalReason, testCase.Expected.Reason)
			messageMatch := true

			if testCase.Expected.Message != "" {
				messageMatch = testCase.Expected.Message == finalMessage || strings.Contains(finalMessage, testCase.Expected.Message)
			}
			if testCase.Expected.MessageContains != "" {
				messageMatch = messageMatch && strings.Contains(finalMessage, testCase.Expected.MessageContains)
			}

			if reasonMatch && messageMatch {
				result.Success = true
			} else {
				result.Details = fmt.Sprintf(
					"actual response does not match expected. Expected: (reason=%s, message=%s, messageContains=%s), Actual: (reason=%s, message=%s)",
					testCase.Expected.Reason,
					testCase.Expected.Message,
					testCase.Expected.MessageContains,
					finalReason,
					finalMessage,
				)
			}
		} else {
			result.Success = true
		}
	} else {
		result.Success = false
		result.Details = fmt.Sprintf(
			"expected result (allowed=%t) and actual result (allowed=%t) does not match",
			testCase.Expected.Allowed,
			finalAllowed,
		)
	}

	return result, nil
}