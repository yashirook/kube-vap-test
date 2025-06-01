package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	vaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
)

func TestMessageExpressionIntegration(t *testing.T) {
	simulator, err := NewPolicySimulator()
	require.NoError(t, err)

	// Policy with messageExpression
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "messageexpression-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "replicaCount",
					Expression: "object.spec.replicas",
				},
				{
					Name:       "maxReplicas",
					Expression: "10",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression:        "variables.replicaCount <= variables.maxReplicas",
					Message:           "Too many replicas", // Static fallback message
					MessageExpression: "'Replica count ' + string(variables.replicaCount) + ' exceeds maximum of ' + string(variables.maxReplicas)",
				},
			},
		},
	}

	// Test case: Deployment with too many replicas
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": int64(15),
			},
		},
	}

	objJSON, _ := obj.MarshalJSON()
	testCase := vaptestv1.TestCase{
		Name: "high-replica-deployment",
		Object: runtime.RawExtension{
			Raw: objJSON,
		},
		Operation: "CREATE",
		Expected: vaptestv1.ExpectedResult{
			Allowed: false,
			Message: "Replica count 15 exceeds maximum of 10",
		},
	}

	result, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase)
	require.NoError(t, err)

	// Verify the test succeeded and the dynamic message was generated
	assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
	assert.False(t, result.ActualResponse.Allowed, "Deployment should be rejected")
	assert.Equal(t, "Replica count 15 exceeds maximum of 10", result.ActualResponse.Message, "Dynamic message should match expected")
}

func TestMessageExpressionWithComplexVariables(t *testing.T) {
	simulator, err := NewPolicySimulator()
	require.NoError(t, err)

	// Policy with complex variables and messageExpression
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "complex-messageexpression-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "containerNames",
					Expression: "object.spec.template.spec.containers.map(c, c.name)",
				},
				{
					Name:       "hasNginx",
					Expression: "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
				},
				{
					Name:       "containerCount",
					Expression: "size(object.spec.template.spec.containers)",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression:        "variables.containerCount <= 3 || variables.hasNginx",
					Message:           "Container validation failed", // Static fallback
					MessageExpression: "'Found ' + string(variables.containerCount) + ' containers. Deployments with more than 3 containers must include nginx.'",
				},
			},
		},
	}

	// Test case: Deployment with 4 containers but no nginx
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "app1"},
							map[string]interface{}{"name": "app2"},
							map[string]interface{}{"name": "sidecar1"},
							map[string]interface{}{"name": "sidecar2"},
						},
					},
				},
			},
		},
	}

	objJSON, _ := obj.MarshalJSON()
	testCase := vaptestv1.TestCase{
		Name: "multi-container-deployment",
		Object: runtime.RawExtension{
			Raw: objJSON,
		},
		Operation: "CREATE",
		Expected: vaptestv1.ExpectedResult{
			Allowed: false,
			Message: "Found 4 containers. Deployments with more than 3 containers must include nginx.",
		},
	}

	result, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase)
	require.NoError(t, err)

	// Verify the test succeeded and the complex dynamic message was generated
	assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
	assert.False(t, result.ActualResponse.Allowed, "Deployment should be rejected")
	assert.Equal(t, "Found 4 containers. Deployments with more than 3 containers must include nginx.", result.ActualResponse.Message)
}

func TestMessageExpressionFallback(t *testing.T) {
	simulator, err := NewPolicySimulator()
	require.NoError(t, err)

	// Policy with invalid messageExpression (should fall back to static message)
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fallback-messageexpression-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{
					Expression:        "object.spec.replicas <= 5",
					Message:           "Static fallback message",
					MessageExpression: "nonExistentFunction()", // Invalid expression
				},
			},
		},
	}

	// Test case: Deployment with too many replicas
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": int64(10),
			},
		},
	}

	objJSON, _ := obj.MarshalJSON()
	testCase := vaptestv1.TestCase{
		Name: "fallback-test",
		Object: runtime.RawExtension{
			Raw: objJSON,
		},
		Operation: "CREATE",
		Expected: vaptestv1.ExpectedResult{
			Allowed: false,
			Message: "Static fallback message",
		},
	}

	result, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase)
	require.NoError(t, err)

	// Verify the test succeeded and fell back to static message
	assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
	assert.False(t, result.ActualResponse.Allowed, "Deployment should be rejected")
	assert.Equal(t, "Static fallback message", result.ActualResponse.Message, "Should fall back to static message")
}

func TestMessageExpressionValidation(t *testing.T) {
	validator, err := NewPolicyValidator()
	require.NoError(t, err)

	tests := []struct {
		name        string
		validation  admissionregistrationv1.Validation
		variables   map[string]interface{}
		contextVars map[string]interface{}
		expected    string
		expectError bool
	}{
		{
			name: "Valid messageExpression",
			validation: admissionregistrationv1.Validation{
				MessageExpression: "'Value is ' + string(variables.value)",
			},
			variables: map[string]interface{}{
				"value": int64(42),
			},
			contextVars: map[string]interface{}{
				"object": map[string]interface{}{},
			},
			expected:    "Value is 42",
			expectError: false,
		},
		{
			name: "MessageExpression with object reference",
			validation: admissionregistrationv1.Validation{
				MessageExpression: "'Object name is ' + object.metadata.name",
			},
			variables: map[string]interface{}{},
			contextVars: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-object",
					},
				},
			},
			expected:    "Object name is test-object",
			expectError: false,
		},
		{
			name: "MessageExpression returning non-string",
			validation: admissionregistrationv1.Validation{
				MessageExpression: "42", // Returns int, not string
			},
			variables:   map[string]interface{}{},
			contextVars: map[string]interface{}{},
			expected:    "",
			expectError: true,
		},
		{
			name: "MessageExpression with line breaks",
			validation: admissionregistrationv1.Validation{
				MessageExpression: "'Line 1\\nLine 2'", // Contains line break
			},
			variables:   map[string]interface{}{},
			contextVars: map[string]interface{}{},
			expected:    "",
			expectError: true,
		},
		{
			name: "MessageExpression returning empty string",
			validation: admissionregistrationv1.Validation{
				MessageExpression: "''", // Empty string
			},
			variables:   map[string]interface{}{},
			contextVars: map[string]interface{}{},
			expected:    "",
			expectError: true,
		},
		{
			name: "Fallback to static message",
			validation: admissionregistrationv1.Validation{
				Message: "Static message",
			},
			variables:   map[string]interface{}{},
			contextVars: map[string]interface{}{},
			expected:    "Static message",
			expectError: false,
		},
		{
			name: "Default message when no message provided",
			validation: admissionregistrationv1.Validation{
				Expression: "object.spec.replicas > 0",
			},
			variables:   map[string]interface{}{},
			contextVars: map[string]interface{}{},
			expected:    "failed expression: object.spec.replicas > 0",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator.SetContextVariables(tt.contextVars)

			message, err := validator.evaluateMessage(tt.validation, tt.variables)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, message)
			}
		})
	}
}