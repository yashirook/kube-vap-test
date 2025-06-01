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

	kaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
)

// simulatorTestHelper provides helper methods for creating test resources
type simulatorTestHelper struct {
	t *testing.T
}

func newSimulatorTestHelper(t *testing.T) *simulatorTestHelper {
	return &simulatorTestHelper{t: t}
}

func (l *simulatorTestHelper) loadDefaultTestPolicy() *admissionregistrationv1.ValidatingAdmissionPolicy {
	// Create a basic policy that denies privileged containers
	failurePolicy := admissionregistrationv1.Fail
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: &failurePolicy,
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "!object.spec.containers.exists(c, has(c.securityContext) && has(c.securityContext.privileged) && c.securityContext.privileged == true)",
					Message:    "Privileged containers are not allowed",
					Reason: func() *metav1.StatusReason {
						reason := metav1.StatusReason("Prohibited")
						return &reason
					}(),
				},
			},
		},
	}
	return policy
}

func (l *simulatorTestHelper) loadDefaultTestBinding() *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	// Create a basic binding that applies to default namespace
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{
				admissionregistrationv1.Deny,
			},
			MatchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"default"},
						},
					},
				},
			},
		},
	}
	return binding
}

func TestSimulateWithPolicyBindings(t *testing.T) {
	// Create simulator
	simulator, err := NewPolicySimulator()
	require.NoError(t, err, "Failed to create policy simulator")

	// Create test resources
	helper := newSimulatorTestHelper(t)
	policy := helper.loadDefaultTestPolicy()
	binding := helper.loadDefaultTestBinding()

	// Create test object
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name": "test-container",
						"securityContext": map[string]interface{}{
							"privileged": true,
						},
					},
				},
			},
		},
	}

	objJSON, _ := obj.MarshalJSON()
	testCase := kaptestv1.TestCase{
		Name: "test-privileged-pod",
		Object: runtime.RawExtension{
			Raw: objJSON,
		},
		Operation: "CREATE",
		Expected: kaptestv1.ExpectedResult{
			Allowed: false,
			Reason:  "Prohibited",
			Message: "Privileged containers are not allowed",
		},
	}

	// Execute test
	result, err := simulator.SimulateWithPolicyBindings(
		context.Background(),
		[]*admissionregistrationv1.ValidatingAdmissionPolicy{policy},
		[]*admissionregistrationv1.ValidatingAdmissionPolicyBinding{binding},
		nil,
		testCase,
	)

	// Assertions
	require.NoError(t, err, "SimulateWithPolicyBindings failed")
	assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
	assert.Equal(t, "test-privileged-pod", result.Name)
	require.NotNil(t, result.ActualResponse)
	assert.False(t, result.ActualResponse.Allowed)
	assert.Equal(t, "Prohibited", result.ActualResponse.Reason)
	assert.Equal(t, "Privileged containers are not allowed", result.ActualResponse.Message)
}

func TestSimulatorWithBindingSelectors(t *testing.T) {
	// Test cases
	testCases := []struct {
		name            string
		namespace       string
		labels          map[string]string
		bindingModifier func(binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding) *admissionregistrationv1.ValidatingAdmissionPolicyBinding
		expectAllowed   bool
		expectReason    string
		expectMessage   string
	}{
		{
			name:      "Matching namespace selector - should be denied",
			namespace: "default",
			labels:    nil,
			bindingModifier: func(binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
				return binding // No changes
			},
			expectAllowed: false,
			expectReason:  "Prohibited",
			expectMessage: "Privileged containers are not allowed",
		},
		{
			name:      "Non-matching namespace selector - should be skipped",
			namespace: "kube-system",
			labels:    nil,
			bindingModifier: func(binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
				return binding // No changes
			},
			expectAllowed: true,
			expectReason:  "",
			expectMessage: "",
		},
		{
			name:      "Matching object selector - should be denied",
			namespace: "default",
			labels:    map[string]string{"app": "test"},
			bindingModifier: func(binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
				modifiedBinding := binding.DeepCopy()
				modifiedBinding.Spec.MatchResources.ObjectSelector = &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				}
				return modifiedBinding
			},
			expectAllowed: false,
			expectReason:  "Prohibited",
			expectMessage: "Privileged containers are not allowed",
		},
		{
			name:      "Non-matching object selector - should be skipped",
			namespace: "default",
			labels:    map[string]string{"app": "prod"},
			bindingModifier: func(binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
				modifiedBinding := binding.DeepCopy()
				modifiedBinding.Spec.MatchResources.ObjectSelector = &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				}
				return modifiedBinding
			},
			expectAllowed: true,
			expectReason:  "",
			expectMessage: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create simulator
			simulator, err := NewPolicySimulator()
			require.NoError(t, err, "Failed to create policy simulator")

			// Load test resources
			helper := newSimulatorTestHelper(t)
			policy := helper.loadDefaultTestPolicy()
			binding := tc.bindingModifier(helper.loadDefaultTestBinding())

			// Create test object
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": tc.namespace,
						"labels":    tc.labels,
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name": "test-container",
								"securityContext": map[string]interface{}{
									"privileged": true,
								},
							},
						},
					},
				},
			}

			objJSON, _ := obj.MarshalJSON()
			testCase := kaptestv1.TestCase{
				Name: "test",
				Object: runtime.RawExtension{
					Raw: objJSON,
				},
				Operation: "CREATE",
				Expected: kaptestv1.ExpectedResult{
					Allowed: tc.expectAllowed,
					Reason:  tc.expectReason,
					Message: tc.expectMessage,
				},
			}

			// Execute test
			result, err := simulator.SimulateWithPolicyBindings(
				context.Background(),
				[]*admissionregistrationv1.ValidatingAdmissionPolicy{policy},
				[]*admissionregistrationv1.ValidatingAdmissionPolicyBinding{binding},
				nil,
				testCase,
			)

			// Assertions
			require.NoError(t, err, "Policy evaluation failed")
			assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
		})
	}
}

func TestSimulatorMatchesPolicy(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		policy         *admissionregistrationv1.ValidatingAdmissionPolicy
		object         map[string]interface{}
		operation      string
		expectedMatch  bool
	}{
		{
			name:   "Nil Policy",
			policy: nil,
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
			},
			operation:     "CREATE",
			expectedMatch: false,
		},
		{
			name: "Match Constraints - Resource Type Match",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								ResourceNames: []string{},
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{
										admissionregistrationv1.Create,
									},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"apps"},
										APIVersions: []string{"v1"},
										Resources:   []string{"deployments"},
									},
								},
							},
						},
					},
				},
			},
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
			},
			operation:     "CREATE",
			expectedMatch: true,
		},
		{
			name: "Match Constraints - Resource Type No Match",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								ResourceNames: []string{},
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{
										admissionregistrationv1.Create,
									},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"apps"},
										APIVersions: []string{"v1"},
										Resources:   []string{"deployments"},
									},
								},
							},
						},
					},
				},
			},
			object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
			},
			operation:     "CREATE",
			expectedMatch: false,
		},
		{
			name: "Match Conditions - CEL Expression Match",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "namespace-match",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
			object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
			operation:     "CREATE",
			expectedMatch: true,
		},
		{
			name: "Match Conditions - CEL Expression No Match",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "namespace-match",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
			object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "kube-system",
				},
			},
			operation:     "CREATE",
			expectedMatch: false,
		},
		{
			name: "Both Constraints and Conditions - All Match",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								ResourceNames: []string{},
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{
										admissionregistrationv1.Create,
									},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"apps"},
										APIVersions: []string{"v1"},
										Resources:   []string{"deployments"},
									},
								},
							},
						},
					},
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "namespace-match",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
			operation:     "CREATE",
			expectedMatch: true,
		},
		{
			name: "Both Constraints and Conditions - One No Match",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								ResourceNames: []string{},
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{
										admissionregistrationv1.Create,
									},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"apps"},
										APIVersions: []string{"v1"},
										Resources:   []string{"deployments"},
									},
								},
							},
						},
					},
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "namespace-match",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"namespace": "kube-system",
				},
			},
			operation:     "CREATE",
			expectedMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// TODO: Add matchesPolicy test implementation when the function is exposed
			// For now, this test is a placeholder
		})
	}
}