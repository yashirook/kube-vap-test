package engine

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yashirook/kube-vap-test/internal/engine/admission"
	"github.com/yashirook/kube-vap-test/internal/loader"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEvaluatePolicy(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err)
	require.NotNil(t, evaluator)

	// Create test policy
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.metadata.labels.size() >= 2",
					Message:    "At least 2 labels are required",
				},
				{
					Expression: "has(object.metadata.labels.app)",
					Message:    "app label is required",
				},
			},
		},
	}

	// Test object
	testObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"app":  "test-app",
				"tier": "frontend",
			},
		},
	}

	// Policy evaluation
	allowed, err := evaluator.EvaluatePolicy(policy, testObj)
	require.NoError(t, err)
	assert.True(t, allowed, "Valid object should be allowed")

	// Invalid object
	invalidObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"tier": "frontend",
			},
		},
	}

	// Policy evaluation
	allowed, err = evaluator.EvaluatePolicy(policy, invalidObj)
	require.NoError(t, err)
	assert.False(t, allowed, "Invalid object should be rejected")
}

func TestEvaluateInvalidPolicy(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err)
	require.NotNil(t, evaluator)

	// Test object
	testObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"app":  "test-app",
				"tier": "frontend",
			},
		},
	}

	// Test policy - invalid expression
	invalidPolicy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "invalid_function(object)",
					Message:    "Invalid function",
				},
			},
		},
	}

	// Verify that an error is returned
	_, err = evaluator.EvaluatePolicy(invalidPolicy, testObj)
	assert.Error(t, err, "Policy with invalid expression should return an error")
}

func TestEvaluatePolicyWithBinding(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err)
	require.NotNil(t, evaluator)

	// Create test policy
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.metadata.labels.size() >= 2",
					Message:    "At least 2 labels are required",
				},
			},
		},
	}

	// Create test binding
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{
				admissionregistrationv1.Deny,
			},
		},
	}

	// Test object
	testObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"app":  "test-app",
				"tier": "frontend",
			},
		},
	}

	// Evaluate policy with binding
	allowed, message, err := evaluator.EvaluatePolicyWithBinding(policy, binding, testObj)
	require.NoError(t, err)
	assert.True(t, allowed, "Valid object should be allowed")
	assert.Empty(t, message, "Error message should be empty")

	// Invalid object
	invalidObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"app": "test-app",
			},
		},
	}

	// Evaluate policy with binding
	allowed, message, err = evaluator.EvaluatePolicyWithBinding(policy, binding, invalidObj)
	require.NoError(t, err)
	assert.False(t, allowed, "Invalid object should be rejected")
	assert.NotEmpty(t, message, "Error message should be present")
}

// TestCELExpressionPatterns tests various CEL expression patterns
func TestCELExpressionPatterns(t *testing.T) {
	// Create evaluator
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err, "Failed to create CEL evaluator")

	// Test cases
	testCases := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		expectTrue bool
	}{
		// 1. Label validation tests
		{
			name:       "No app label - should be rejected",
			expression: "has(object.metadata.labels) && has(object.metadata.labels.app)",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-app",
						"labels": map[string]interface{}{
							"environment": "staging",
						},
					},
					"spec": map[string]interface{}{},
				},
			},
			expectTrue: false,
		},
		{
			name:       "Has app label - should be allowed",
			expression: "has(object.metadata.labels) && has(object.metadata.labels.app)",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-app",
						"labels": map[string]interface{}{
							"app": "test",
						},
					},
					"spec": map[string]interface{}{},
				},
			},
			expectTrue: true,
		},

		// 2. Name regex matching tests
		{
			name:       "Invalid name (contains uppercase) - should be rejected",
			expression: "object.metadata.name.matches('^[a-z0-9]([a-z0-9\\\\-]*[a-z0-9])?(\\\\.[a-z0-9]([a-z0-9\\\\-]*[a-z0-9])?)*$')",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "Test-App",
					},
				},
			},
			expectTrue: false,
		},
		{
			name:       "Valid name - should be allowed",
			expression: "object.metadata.name.matches('^[a-z0-9]([a-z0-9\\\\-]*[a-z0-9])?(\\\\.[a-z0-9]([a-z0-9\\\\-]*[a-z0-9])?)*$')",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-app.service",
					},
				},
			},
			expectTrue: true,
		},

		// 3. Replica count validation
		{
			name:       "Too many replicas - should be rejected",
			expression: "object.spec.replicas <= 10",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": 15,
					},
				},
			},
			expectTrue: false,
		},
		{
			name:       "Appropriate number of replicas - should be allowed",
			expression: "object.spec.replicas <= 10",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": 5,
					},
				},
			},
			expectTrue: true,
		},

		// 4. Container count validation
		{
			name:       "Too many containers - should be rejected",
			expression: "size(object.spec.containers) <= 5",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{}, map[string]interface{}{},
							map[string]interface{}{}, map[string]interface{}{},
							map[string]interface{}{}, map[string]interface{}{},
						},
					},
				},
			},
			expectTrue: false,
		},
		{
			name:       "Appropriate number of containers - should be allowed",
			expression: "size(object.spec.containers) <= 5",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{}, map[string]interface{}{},
							map[string]interface{}{},
						},
					},
				},
			},
			expectTrue: true,
		},

		// 5. hostNetwork validation
		{
			name:       "hostNetwork enabled - should be rejected",
			expression: "!has(object.spec.hostNetwork) || object.spec.hostNetwork == false",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"spec": map[string]interface{}{
						"hostNetwork": true,
					},
				},
			},
			expectTrue: false,
		},
		{
			name:       "hostNetwork disabled - should be allowed",
			expression: "!has(object.spec.hostNetwork) || object.spec.hostNetwork == false",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"spec": map[string]interface{}{
						"hostNetwork": false,
					},
				},
			},
			expectTrue: true,
		},

		// 6. Label count validation
		{
			name:       "Insufficient labels - should be rejected",
			expression: "has(object.metadata.labels) && object.metadata.labels.size() >= 3",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":  "test",
							"tier": "frontend",
						},
					},
				},
			},
			expectTrue: false,
		},
		{
			name:       "Sufficient labels - should be allowed",
			expression: "has(object.metadata.labels) && object.metadata.labels.size() >= 3",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":         "test",
							"tier":        "frontend",
							"environment": "staging",
						},
					},
				},
			},
			expectTrue: true,
		},
	}

	// Execute each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExpression(tc.expression, tc.variables)
			if err != nil {
				t.Fatalf("Error occurred during expression evaluation: %v", err)
			}
			assert.Equal(t, tc.expectTrue, result, "Evaluation result differs from expectation")
		})
	}
}

// createContainerWithResources is a helper function that creates a container object with resource limits
func createContainerWithResources(name, image string) map[string]interface{} {
	return map[string]interface{}{
		"name":  name,
		"image": image,
		"resources": map[string]interface{}{
			"limits": map[string]interface{}{
				"memory": "512Mi",
				"cpu":    "500m",
			},
		},
	}
}

// TestCELSimpleExpressions tests simple CEL expression patterns
func TestCELSimpleExpressions(t *testing.T) {
	// Create evaluator
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err, "Failed to create CEL evaluator")

	// Test simple CEL expressions
	testCases := []struct {
		name        string
		expression  string
		variables   map[string]interface{}
		expectTrue  bool
		expectError bool
	}{
		{
			name:       "Integer comparison (10 > 5)",
			expression: "10 > 5",
			expectTrue: true,
		},
		{
			name:       "String comparison ('apple' == 'apple')",
			expression: "'apple' == 'apple'",
			expectTrue: true,
		},
		{
			name:       "Logical operators (true && !false)",
			expression: "true && !false",
			expectTrue: true,
		},
		{
			name:       "Arithmetic operations ((5 + 5) * 2 == 20)",
			expression: "(5 + 5) * 2 == 20",
			expectTrue: true,
		},
		{
			name:       "Ternary operator",
			expression: "10 > 5 ? true : false",
			expectTrue: true,
		},
		{
			name:       "String function (size)",
			expression: "size('hello') == 5",
			expectTrue: true,
		},
		{
			name:       "String function (startsWith)",
			expression: "'hello world'.startsWith('hello')",
			expectTrue: true,
		},
		{
			name:       "String function (endsWith)",
			expression: "'hello world'.endsWith('world')",
			expectTrue: true,
		},
		{
			name:       "String function (contains)",
			expression: "'hello world'.contains('lo wo')",
			expectTrue: true,
		},
		{
			name:       "String function (matches-regex)",
			expression: "'abc123'.matches('[a-z]+[0-9]+')",
			expectTrue: true,
		},
		{
			name:        "Error case: Type mismatch",
			expression:  "'string' > 5",
			expectError: true,
		},
		{
			name:        "Error case: Undefined variable",
			expression:  "undefined_var",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExpression(tc.expression, tc.variables)

			if tc.expectError {
				assert.Error(t, err, "Expected error but succeeded")
				return
			}

			require.NoError(t, err, "Failed to evaluate CEL expression: %s", tc.expression)

			if tc.expectTrue {
				assert.True(t, result, "Expression should return true: %s", tc.expression)
			} else {
				assert.False(t, result, "Expression should return false: %s", tc.expression)
			}
		})
	}
}

// TestCELWithVariables tests CEL expression evaluation with variables
func TestCELWithVariables(t *testing.T) {
	// Create evaluator
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err, "Failed to create CEL evaluator")

	// Test cases
	testCases := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		expectTrue bool
	}{
		{
			name:       "Simple variable reference",
			expression: "value > 10",
			variables: map[string]interface{}{
				"value": 20,
			},
			expectTrue: true,
		},
		{
			name:       "Nested variable reference",
			expression: "user.age >= 18",
			variables: map[string]interface{}{
				"user": map[string]interface{}{
					"age": 25,
				},
			},
			expectTrue: true,
		},
		{
			name:       "Compound conditions",
			expression: "user.age >= 18 && user.active == true",
			variables: map[string]interface{}{
				"user": map[string]interface{}{
					"age":    25,
					"active": true,
				},
			},
			expectTrue: true,
		},
		{
			name:       "Map key existence check",
			expression: "has(data.key1)",
			variables: map[string]interface{}{
				"data": map[string]interface{}{
					"key1": "value1",
				},
			},
			expectTrue: true,
		},
		{
			name:       "Array access",
			expression: "items[0] == 'first'",
			variables: map[string]interface{}{
				"items": []interface{}{"first", "second", "third"},
			},
			expectTrue: true,
		},
		{
			name:       "Map access",
			expression: "configs['db'].host == 'localhost'",
			variables: map[string]interface{}{
				"configs": map[string]interface{}{
					"db": map[string]interface{}{
						"host": "localhost",
						"port": 5432,
					},
				},
			},
			expectTrue: true,
		},
		{
			name:       "Kubernetes resource-like object",
			expression: "object.metadata.labels.app == 'example' && object.spec.replicas <= 3",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-app",
						"labels": map[string]interface{}{
							"app": "example",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
				},
			},
			expectTrue: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExpression(tc.expression, tc.variables)
			require.NoError(t, err, "Failed to evaluate CEL expression: %s", tc.expression)

			if tc.expectTrue {
				assert.True(t, result, "Expression should return true: %s", tc.expression)
			} else {
				assert.False(t, result, "Expression should return false: %s", tc.expression)
			}
		})
	}
}

// TestCELErrorHandling tests error handling in CEL
func TestCELErrorHandling(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err, "Failed to create CEL evaluator")

	testCases := []struct {
		name        string
		expression  string
		variables   map[string]interface{}
		expectError string
	}{
		{
			name:       "Access to non-existent field",
			expression: "object.metadata.nonexistent.value == true",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{},
				},
			},
			expectError: "no such key",
		},
		{
			name:       "Null reference error",
			expression: "object.metadata.labels.app == 'test'",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": nil,
					},
				},
			},
			expectError: "no such key",
		},
		{
			name:       "Type conversion error - adding number to string",
			expression: "object.value + 10",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"value": "string",
				},
			},
			expectError: "no such overload",
		},
		{
			name:       "Type conversion error - comparing string with number",
			expression: "object.metadata.name > 100",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-name",
					},
				},
			},
			expectError: "no such overload",
		},
		{
			name:       "Array out of bounds access",
			expression: "object.spec.containers[10].name == 'container'",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name": "container1",
							},
						},
					},
				},
			},
			expectError: "index out of bounds",
		},
		{
			name:       "Invalid logical operation - AND with non-boolean",
			expression: "object.metadata.name && true",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-name",
					},
				},
			},
			expectError: "no such overload",
		},
		{
			name:       "Calling map as function",
			expression: "object.metadata()",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"metadata": map[string]interface{}{},
				},
			},
			expectError: "no such overload",
		},
		{
			name:        "Undefined variable reference",
			expression:  "nonexistent_var == true",
			variables:   map[string]interface{}{},
			expectError: "no such attribute",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evaluator.EvaluateExpression(tc.expression, tc.variables)
			assert.Error(t, err, "Expected error but succeeded")
			assert.Contains(t, err.Error(), tc.expectError,
				"Expected error message not found: %s", err.Error())
		})
	}
}

// TestCELFunctionsAndMacros tests CEL built-in functions and macros
func TestCELFunctionsAndMacros(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err, "Failed to create CEL evaluator")

	// Test data
	variables := map[string]interface{}{
		"pod": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-pod",
				"labels": map[string]interface{}{
					"app":  "myapp",
					"env":  "test",
					"team": "backend",
				},
				"annotations": map[string]interface{}{
					"example.com/skip-check":     "true",
					"example.com/config-version": "v1",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "main",
						"image": "nginx:1.19",
						"ports": []interface{}{
							map[string]interface{}{"containerPort": 80},
							map[string]interface{}{"containerPort": 443},
						},
					},
				},
			},
		},
		"stringData": "Hello World",
		"numberData": 42,
		"listData":   []interface{}{1, 2, 3, 4, 5},
		"mapData": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": 123,
		},
		"emptyList": []interface{}{},
		"mixedList": []interface{}{
			"string",
			42,
			true,
			[]interface{}{1, 2},
			map[string]interface{}{"a": "b"},
		},
		"timestamp": "2023-04-01T12:00:00Z",
	}

	testCases := []struct {
		name       string
		expression string
		expected   bool
	}{
		// has() macro tests
		{
			name:       "has() - object field existence check",
			expression: "has(pod.metadata.name)",
			expected:   true,
		},
		{
			name:       "has() - non-existent field",
			expression: "has(pod.metadata.namespace)",
			expected:   false,
		},
		{
			name:       "has() - nested map key",
			expression: "has(pod.metadata.labels.app)",
			expected:   true,
		},

		// size() function tests
		{
			name:       "size() - string",
			expression: "size(stringData) == 11",
			expected:   true,
		},
		{
			name:       "size() - list",
			expression: "size(listData) == 5",
			expected:   true,
		},
		{
			name:       "size() - map",
			expression: "size(mapData) == 3",
			expected:   true,
		},
		{
			name:       "size() - empty list",
			expression: "size(emptyList) == 0",
			expected:   true,
		},

		// String function tests
		{
			name:       "string.startsWith()",
			expression: "stringData.startsWith('Hello')",
			expected:   true,
		},
		{
			name:       "string.endsWith()",
			expression: "stringData.endsWith('World')",
			expected:   true,
		},
		{
			name:       "string.contains()",
			expression: "stringData.contains('llo Wo')",
			expected:   true,
		},
		{
			name:       "string.matches() - simple pattern",
			expression: "pod.metadata.name.matches('^test-.*$')",
			expected:   true,
		},
		{
			name:       "string.matches() - complex pattern",
			expression: "'abc123xyz'.matches('[a-z]+[0-9]+[a-z]+')",
			expected:   true,
		},

		// Type conversion function tests
		{
			name:       "string() - conversion from number",
			expression: "string(numberData) == '42'",
			expected:   true,
		},
		{
			name:       "int() - conversion from string",
			expression: "int('42') == 42",
			expected:   true,
		},
		{
			name:       "bool() - conversion from string",
			expression: "bool('true') == true",
			expected:   true,
		},
		{
			name:       "double() - conversion from int",
			expression: "double(42) == 42.0",
			expected:   true,
		},

		// in operator tests
		{
			name:       "in - map key check",
			expression: "'key1' in mapData",
			expected:   true,
		},
		{
			name:       "in - non-existent map key",
			expression: "'key4' in mapData",
			expected:   false,
		},
		{
			name:       "in - list element check",
			expression: "3 in listData",
			expected:   true,
		},
		{
			name:       "in - non-existent list element",
			expression: "10 in listData",
			expected:   false,
		},

		// Ternary operator tests
		{
			name:       "? : - simple case",
			expression: "size(pod.spec.containers) > 0 ? true : false",
			expected:   true,
		},

		// Type checking functions
		{
			name:       "Type check - string type",
			expression: "type(stringData) == string",
			expected:   true,
		},
		{
			name:       "Type check - int type",
			expression: "type(numberData) == int",
			expected:   true,
		},
		{
			name:       "Type check - list type",
			expression: "type(listData) == list",
			expected:   true,
		},
		{
			name:       "Type check - map type",
			expression: "type(mapData) == map",
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExpression(tc.expression, variables)
			require.NoError(t, err, "Failed to evaluate expression: %s", tc.expression)
			assert.Equal(t, tc.expected, result, "Expression result differs from expectation: %s", tc.expression)
		})
	}
}

// TestCELCompositeDataTypes tests functionality related to composite data types
func TestCELCompositeDataTypes(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err, "Failed to create CEL evaluator")

	// Test data
	variables := map[string]interface{}{
		"deployment": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "web-app",
				"labels": map[string]interface{}{
					"app":     "web",
					"version": "v1",
					"tier":    "frontend",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "web",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":     "web",
							"version": "v1",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "web",
								"image": "nginx:1.19",
								"ports": []interface{}{
									map[string]interface{}{"containerPort": 80},
								},
								"resources": map[string]interface{}{
									"limits": map[string]interface{}{
										"cpu":    "500m",
										"memory": "512Mi",
									},
									"requests": map[string]interface{}{
										"cpu":    "200m",
										"memory": "256Mi",
									},
								},
								"env": []interface{}{
									map[string]interface{}{
										"name":  "DEBUG",
										"value": "false",
									},
									map[string]interface{}{
										"name":  "LOG_LEVEL",
										"value": "info",
									},
								},
							},
							map[string]interface{}{
								"name":  "sidecar",
								"image": "envoy:v1",
								"ports": []interface{}{
									map[string]interface{}{"containerPort": 8080},
								},
								"resources": map[string]interface{}{
									"limits": map[string]interface{}{
										"cpu":    "100m",
										"memory": "128Mi",
									},
								},
							},
						},
					},
				},
			},
		},
		"numbers": []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		"words":   []interface{}{"apple", "banana", "cherry", "date", "elderberry"},
		"empty":   []interface{}{},
		"mixed":   []interface{}{true, "string", 42, 3.14},
		"users": []interface{}{
			map[string]interface{}{
				"name":  "Alice",
				"age":   30,
				"roles": []interface{}{"admin", "user"},
			},
			map[string]interface{}{
				"name":  "Bob",
				"age":   25,
				"roles": []interface{}{"user"},
			},
			map[string]interface{}{
				"name":  "Charlie",
				"age":   35,
				"roles": []interface{}{"user", "reviewer"},
			},
		},
		"configs": map[string]interface{}{
			"db": map[string]interface{}{
				"host":     "localhost",
				"port":     5432,
				"username": "admin",
			},
			"api": map[string]interface{}{
				"url":      "https://api.example.com",
				"timeout":  30,
				"retryMax": 3,
			},
		},
	}

	testCases := []struct {
		name       string
		expression string
		expected   bool
	}{
		// List processing functions - filter
		{
			name:       "filter() - filtering with simple condition",
			expression: "size(numbers.filter(n, n > 5)) == 5",
			expected:   true,
		},
		{
			name:       "filter() - filtering with multiple conditions",
			expression: "size(numbers.filter(n, n > 3 && n < 8)) == 4",
			expected:   true,
		},
		{
			name:       "filter() - filtering object array",
			expression: "size(users.filter(u, u.age > 25)) == 2",
			expected:   true,
		},
		{
			name:       "filter() - filtering with nested conditions",
			expression: "size(users.filter(u, u.roles.exists(r, r == 'admin'))) == 1",
			expected:   true,
		},
		{
			name:       "filter() - empty result",
			expression: "size(numbers.filter(n, n > 100)) == 0",
			expected:   true,
		},

		// List processing functions - all
		{
			name:       "all() - check all elements with simple condition",
			expression: "numbers.all(n, n > 0)",
			expected:   true,
		},
		{
			name:       "all() - failing case",
			expression: "numbers.all(n, n > 5)",
			expected:   false,
		},
		{
			name:       "all() - check all elements in object array",
			expression: "users.all(u, has(u.name) && has(u.age))",
			expected:   true,
		},
		{
			name:       "all() - empty array",
			expression: "empty.all(e, e == 0)", // For empty arrays, all() always returns true
			expected:   true,
		},

		// List processing functions - exists
		{
			name:       "exists() - existence check with simple condition",
			expression: "numbers.exists(n, n == 5)",
			expected:   true,
		},
		{
			name:       "exists() - failing case",
			expression: "numbers.exists(n, n > 100)",
			expected:   false,
		},
		{
			name:       "exists() - existence check in object array",
			expression: "users.exists(u, u.name == 'Alice' && u.age == 30)",
			expected:   true,
		},
		{
			name:       "exists() - existence check with nested conditions",
			expression: "users.exists(u, u.roles.exists(r, r == 'reviewer'))",
			expected:   true,
		},
		{
			name:       "exists() - empty array",
			expression: "empty.exists(e, e == 0)", // For empty arrays, exists() always returns false
			expected:   false,
		},

		// Map transformation functions are not supported and have been removed

		// List processing with compound conditions
		{
			name:       "filter() + exists() - compound conditions",
			expression: "deployment.spec.template.spec.containers.filter(c, c.resources.limits.memory == '512Mi').exists(c, c.name == 'web')",
			expected:   true,
		},
		{
			name:       "Multiple filter() - chaining",
			expression: "size(deployment.spec.template.spec.containers.filter(c, has(c.resources.limits)).filter(c, c.resources.limits.cpu == '500m')) == 1",
			expected:   true,
		},
		{
			name:       "filter() + all() - combination",
			expression: "deployment.spec.template.spec.containers.filter(c, c.name.startsWith('web')).all(c, has(c.resources.requests))",
			expected:   true,
		},

		// Container-related compound tests
		{
			name:       "Container condition check - resource allocation",
			expression: "deployment.spec.template.spec.containers.all(c, has(c.resources) && has(c.resources.limits))",
			expected:   true,
		},
		{
			name:       "Container condition check - specific environment variable",
			expression: "deployment.spec.template.spec.containers.exists(c, c.env.exists(e, e.name == 'DEBUG'))",
			expected:   true,
		},
		{
			name:       "Container condition check - specific port exposure",
			expression: "deployment.spec.template.spec.containers.exists(c, c.ports.exists(p, p.containerPort == 80))",
			expected:   true,
		},

		// Label matching (simple comparison only)
		{
			name:       "Label match verification",
			expression: "deployment.metadata.labels.app == deployment.spec.selector.matchLabels.app",
			expected:   true,
		},
		// Complex label inheritance verification is not supported and has been removed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExpression(tc.expression, variables)
			require.NoError(t, err, "Failed to evaluate expression: %s", tc.expression)
			assert.Equal(t, tc.expected, result, "Expression result differs from expectation: %s", tc.expression)
		})
	}
}

// TestCELBasicFunctions tests basic CEL functionality
func TestCELBasicFunctions(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err)
	require.NotNil(t, evaluator)

	testObject := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"name": "item1", "enabled": true, "priority": 10},
			map[string]interface{}{"name": "item2", "enabled": false, "priority": 5},
		},
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"example.com/url": "https://example.com",
			},
		},
	}

	testCases := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		expected   bool
	}{
		{
			name:       "Get array size - size function",
			expression: "size(items) == 2",
			variables:  map[string]interface{}{"items": testObject["items"]},
			expected:   true,
		},
		{
			name:       "Get map size - size function",
			expression: "size(metadata.annotations) == 1",
			variables:  map[string]interface{}{"metadata": testObject["metadata"]},
			expected:   true,
		},
		{
			name:       "String comparison - equality",
			expression: "metadata.annotations['example.com/url'] == 'https://example.com'",
			variables:  map[string]interface{}{"metadata": testObject["metadata"]},
			expected:   true,
		},
		{
			name:       "String operation - contains",
			expression: "metadata.annotations['example.com/url'].contains('example')",
			variables:  map[string]interface{}{"metadata": testObject["metadata"]},
			expected:   true,
		},
		{
			name:       "Array access and condition evaluation",
			expression: "items[0].name == 'item1' && items[0].enabled == true",
			variables:  map[string]interface{}{"items": testObject["items"]},
			expected:   true,
		},
		{
			name:       "Logical operator - OR",
			expression: "items[0].priority > 5 || items[1].priority > 5",
			variables:  map[string]interface{}{"items": testObject["items"]},
			expected:   true,
		},
		{
			name:       "Logical operator - AND",
			expression: "items[0].priority > 5 && items[1].priority <= 5",
			variables:  map[string]interface{}{"items": testObject["items"]},
			expected:   true,
		},
		{
			name:       "Conditional expression (ternary operator)",
			expression: "items[0].enabled ? true : false",
			variables:  map[string]interface{}{"items": testObject["items"]},
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EvaluateExpression(tc.expression, tc.variables)
			if err != nil {
				t.Fatalf("Error occurred during expression evaluation: %v", err)
			}
			assert.Equal(t, tc.expected, result, "Result does not match expectation")
		})
	}
}

func TestCELExpressionWithComplexObjects(t *testing.T) {
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err)
	require.NotNil(t, evaluator)

	// Test Pod definition as a complex JSON object
	testPod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app":   "myapp",
				"tier":  "frontend",
				"phase": "test",
			},
			"annotations": map[string]interface{}{
				"deployment.kubernetes.io/revision": "1",
				"custom.example.com/value":          "test-value",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "container1",
					"image": "nginx:1.14.2",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 80,
							"protocol":      "TCP",
						},
					},
					"securityContext": map[string]interface{}{
						"privileged": false,
					},
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "100m",
							"memory": "128Mi",
						},
						"requests": map[string]interface{}{
							"cpu":    "50m",
							"memory": "64Mi",
						},
					},
				},
				map[string]interface{}{
					"name":  "container2",
					"image": "redis:6.0.5",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 6379,
							"protocol":      "TCP",
						},
					},
					"securityContext": map[string]interface{}{
						"privileged": true,
					},
				},
			},
			"volumes": []interface{}{
				map[string]interface{}{
					"name": "data",
					"emptyDir": map[string]interface{}{
						"medium": "Memory",
					},
				},
			},
		},
	}

	// Test case definitions
	testCases := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "Verify container count is 2",
			expression: "size(object.spec.containers) == 2",
			expected:   true,
		},
		{
			name:       "Check if privileged container exists",
			expression: "object.spec.containers.exists(c, c.securityContext.privileged == true)",
			expected:   true,
		},
		{
			name:       "Check if specific label exists",
			expression: "has(object.metadata.labels.app) && object.metadata.labels.app == 'myapp'",
			expected:   true,
		},
		{
			name:       "Check if all container ports are valid",
			expression: "object.spec.containers.all(c, c.ports.all(p, p.containerPort > 0 && p.protocol == 'TCP'))",
			expected:   true,
		},
		{
			name:       "Check if container with memory resource limit exists",
			expression: "object.spec.containers.exists(c, has(c.resources) && has(c.resources.limits) && has(c.resources.limits.memory))",
			expected:   true,
		},
		{
			name:       "Check if volume is EmptyDir",
			expression: "object.spec.volumes.exists(v, has(v.emptyDir))",
			expected:   true,
		},
	}

	// Execute each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create variables needed for testing
			variables := map[string]interface{}{
				"object": testPod,
			}

			// Evaluate expression
			result, err := evaluator.EvaluateExpression(tc.expression, variables)
			if err != nil {
				t.Fatalf("Error occurred during expression evaluation: %v", err)
			}

			// Validate result
			assert.Equal(t, tc.expected, result, "CEL expression evaluation result differs from expectation")
		})
	}

	// Test policy definition
	testPolicy := &v1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-privileged-container-policy",
		},
		Spec: v1.ValidatingAdmissionPolicySpec{
			Validations: []v1.Validation{
				{
					Expression: "!object.spec.containers.exists(c, c.securityContext.privileged == true)",
					Message:    "Privileged containers are not allowed",
				},
			},
		},
	}

	// Test policy evaluation
	allowed, err := evaluator.EvaluatePolicy(testPolicy, testPod)
	require.NoError(t, err)
	assert.False(t, allowed, "Pod with privileged container should violate the policy")
}

func TestLoadPolicyBindingsForSingleFile(t *testing.T) {
	// Create local resource loader
	localLoader, err := loader.NewLocalResourceLoader()
	require.NoError(t, err, "Failed to create local resource loader")

	// Test case
	testPath := filepath.Join("test", "policy-binding-test.yaml")
	resourceSource := loader.ResourceSource{
		Type:  loader.SourceTypeLocal,
		Files: []string{testPath},
	}
	bindings, err := localLoader.LoadPolicyBindings(resourceSource)

	// Assertions
	require.NoError(t, err, "Failed to load policy binding")
	require.Len(t, bindings, 1, "Number of loaded policy bindings differs")
	binding := bindings[0]
	assert.NotNil(t, binding, "Loaded policy binding is nil")
	assert.Equal(t, "test-policy-binding", binding.Name, "Policy binding name differs")
	assert.Equal(t, "test-policy", binding.Spec.PolicyName, "Policy name differs")

	// Verify ValidationActions existence
	require.Len(t, binding.Spec.ValidationActions, 1, "ValidationActions length differs")
	assert.Equal(t, admissionregistrationv1.ValidationAction("Deny"), binding.Spec.ValidationActions[0], "ValidationAction differs")

	assert.NotNil(t, binding.Spec.MatchResources, "MatchResources is nil")
	assert.NotNil(t, binding.Spec.MatchResources.NamespaceSelector, "NamespaceSelector is nil")
}

func TestEvaluatePolicyWithBindingSelectors(t *testing.T) {
	// Create evaluation engine
	evaluator, err := NewDefaultCELEvaluator()
	require.NoError(t, err, "Failed to create CEL evaluation engine")

	// Create test policy
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.spec.containers.all(c, c.image.contains(':latest') == false)",
				},
			},
		},
	}

	// 1. Binding with matching namespace selector
	bindingWithMatchingNamespace := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding-matching-namespace",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{
				"Deny",
			},
			MatchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: "NotIn",
							Values:   []string{"kube-system", "kube-public"},
						},
					},
				},
			},
		},
	}

	// 2. Binding with non-matching namespace selector
	bindingWithNonMatchingNamespace := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding-non-matching-namespace",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{
				"Deny",
			},
			MatchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: "In",
							Values:   []string{"kube-system", "kube-public"},
						},
					},
				},
			},
		},
	}

	// 3. Binding with matching object selector
	bindingWithMatchingObjectSelector := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding-matching-object",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{
				"Deny",
			},
			MatchResources: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
		},
	}

	// 4. Binding with non-matching object selector
	bindingWithNonMatchingObjectSelector := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding-non-matching-object",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{
				"Deny",
			},
			MatchResources: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "other",
					},
				},
			},
		},
	}

	// Test Pod object (using latest tag, matching namespace)
	podInDefaultNamespace := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "test",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "nginx",
					"image": "nginx:latest",
				},
			},
		},
	}

	// Test Pod object (using latest tag, excluded namespace)
	podInKubeSystemNamespace := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "kube-system",
			"labels": map[string]interface{}{
				"app": "test",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "nginx",
					"image": "nginx:latest",
				},
			},
		},
	}

	// Test cases
	testCases := []struct {
		name           string
		policy         *admissionregistrationv1.ValidatingAdmissionPolicy
		binding        *admissionregistrationv1.ValidatingAdmissionPolicyBinding
		object         map[string]interface{}
		expectAllowed  bool
		expectMessage  string
		expectErrorNil bool
	}{
		{
			name:           "Matching namespace selector - policy violation",
			policy:         policy,
			binding:        bindingWithMatchingNamespace,
			object:         podInDefaultNamespace,
			expectAllowed:  false,
			expectMessage:  "policy violation: deny",
			expectErrorNil: true,
		},
		{
			name:           "Non-matching namespace selector - skip",
			policy:         policy,
			binding:        bindingWithNonMatchingNamespace,
			object:         podInDefaultNamespace,
			expectAllowed:  true,
			expectMessage:  "resource does not match binding selector, skipping evaluation",
			expectErrorNil: true,
		},
		{
			name:           "Matching namespace selector but excluded namespace - skip",
			policy:         policy,
			binding:        bindingWithMatchingNamespace,
			object:         podInKubeSystemNamespace,
			expectAllowed:  true,
			expectMessage:  "resource does not match binding selector, skipping evaluation",
			expectErrorNil: true,
		},
		{
			name:           "Matching object selector - policy violation",
			policy:         policy,
			binding:        bindingWithMatchingObjectSelector,
			object:         podInDefaultNamespace,
			expectAllowed:  false,
			expectMessage:  "policy violation: deny",
			expectErrorNil: true,
		},
		{
			name:           "Non-matching object selector - skip",
			policy:         policy,
			binding:        bindingWithNonMatchingObjectSelector,
			object:         podInDefaultNamespace,
			expectAllowed:  true,
			expectMessage:  "resource does not match binding selector, skipping evaluation",
			expectErrorNil: true,
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, message, err := evaluator.EvaluatePolicyWithBinding(tc.policy, tc.binding, tc.object)

			// Validate expectations
			if tc.expectErrorNil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			assert.Equal(t, tc.expectAllowed, allowed, "Allow result differs from expectation")
			assert.Contains(t, message, tc.expectMessage, "Message differs from expectation")
		})
	}
}

// TestMatchesPolicy tests the MatchesPolicy method
func TestMatchesPolicy(t *testing.T) {
	evaluator, _ := NewDefaultCELEvaluator()

	// Create test policy
	createPolicy := func(matchConstraints *admissionregistrationv1.MatchResources, matchConditions []admissionregistrationv1.MatchCondition) *admissionregistrationv1.ValidatingAdmissionPolicy {
		policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-policy",
			},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
				MatchConstraints: matchConstraints,
				MatchConditions:  matchConditions,
			},
		}
		return policy
	}

	tests := []struct {
		name     string
		policy   *admissionregistrationv1.ValidatingAdmissionPolicy
		object   map[string]interface{}
		expected bool
	}{
		{
			name:     "Nil Policy",
			policy:   nil,
			object:   map[string]interface{}{},
			expected: false,
		},
		{
			name: "Match Constraints - Resource Type Match",
			policy: createPolicy(
				&admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{"CREATE"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
				},
				nil,
			),
			object: map[string]interface{}{
				"kind":       "Pod",
				"apiVersion": "v1",
				"operation":  "CREATE",
				"resource": map[string]interface{}{
					"group":    "",
					"version":  "v1",
					"resource": "pods",
				},
			},
			expected: true,
		},
		{
			name: "Match Constraints - Resource Type No Match",
			policy: createPolicy(
				&admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{"CREATE"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
				},
				nil,
			),
			object: map[string]interface{}{
				"kind":       "Service",
				"apiVersion": "v1",
				"operation":  "CREATE",
				"resource": map[string]interface{}{
					"group":    "",
					"version":  "v1",
					"resource": "services",
				},
			},
			expected: false,
		},
		{
			name: "Match Conditions - CEL Expression Match",
			policy: createPolicy(
				nil,
				[]admissionregistrationv1.MatchCondition{
					{
						Name:       "test-condition",
						Expression: "object.metadata.name.startsWith('test-')",
					},
				},
			),
			object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-pod",
				},
			},
			expected: true,
		},
		{
			name: "Match Conditions - CEL Expression No Match",
			policy: createPolicy(
				nil,
				[]admissionregistrationv1.MatchCondition{
					{
						Name:       "test-condition",
						Expression: "object.metadata.name.startsWith('test-')",
					},
				},
			),
			object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "production-pod",
				},
			},
			expected: false,
		},
		{
			name: "Both Constraints and Conditions - All Match",
			policy: createPolicy(
				&admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{"CREATE"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
				},
				[]admissionregistrationv1.MatchCondition{
					{
						Name:       "test-condition",
						Expression: "object.metadata.name.startsWith('test-')",
					},
				},
			),
			object: map[string]interface{}{
				"kind":       "Pod",
				"apiVersion": "v1",
				"operation":  "CREATE",
				"resource": map[string]interface{}{
					"group":    "",
					"version":  "v1",
					"resource": "pods",
				},
				"metadata": map[string]interface{}{
					"name": "test-pod",
				},
			},
			expected: true,
		},
		{
			name: "Both Constraints and Conditions - One No Match",
			policy: createPolicy(
				&admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{"CREATE"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
				},
				[]admissionregistrationv1.MatchCondition{
					{
						Name:       "test-condition",
						Expression: "object.metadata.name.startsWith('test-')",
					},
				},
			),
			object: map[string]interface{}{
				"kind":       "Pod",
				"apiVersion": "v1",
				"operation":  "CREATE",
				"resource": map[string]interface{}{
					"group":    "",
					"version":  "v1",
					"resource": "pods",
				},
				"metadata": map[string]interface{}{
					"name": "production-pod",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set required fields for AdmissionTarget for testing
			// Build AdmissionTarget directly and output contents
			target := admission.AdmissionTarget{
				Object:     tt.object,
				Operation:  "", // Default is empty
				APIGroup:   "",
				APIVersion: "",
				Resource:   "",
			}

			// Set fields from object
			if operation, ok := tt.object["operation"].(string); ok {
				target.Operation = operation
			}

			if resource, ok := tt.object["resource"].(map[string]interface{}); ok {
				if group, ok := resource["group"].(string); ok {
					target.APIGroup = group
				}
				if version, ok := resource["version"].(string); ok {
					target.APIVersion = version
				}
				if resourceType, ok := resource["resource"].(string); ok {
					target.Resource = resourceType
				}
			} else {
				// If no resource, infer from apiVersion and kind
				if apiVersion, ok := tt.object["apiVersion"].(string); ok {
					parts := strings.Split(apiVersion, "/")
					if len(parts) == 2 {
						target.APIGroup = parts[0]
						target.APIVersion = parts[1]
					} else {
						target.APIGroup = ""
						target.APIVersion = apiVersion
					}
				}

				if kind, ok := tt.object["kind"].(string); ok {
					// Convert kind to resource name (lowercase plural)
					target.Resource = strings.ToLower(kind) + "s"
				}
			}

			t.Logf("AdmissionTarget for test '%s': %+v", tt.name, target)
			t.Logf("Object for test '%s': %+v", tt.name, tt.object)

			if tt.policy != nil && tt.policy.Spec.MatchConstraints != nil {
				t.Logf("MatchConstraints for test '%s': %+v", tt.name, tt.policy.Spec.MatchConstraints)
			}

			result, err := evaluator.MatchesPolicy(tt.policy, tt.object)
			if err != nil && tt.policy != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			t.Logf("Result for test '%s': %v, error: %v", tt.name, result, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}