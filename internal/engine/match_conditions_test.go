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

func TestMatchConditionsIntegration(t *testing.T) {
	simulator, err := NewPolicySimulator()
	require.NoError(t, err)

	// Policy with matchConditions - only apply to production namespace
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "production-only-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "production-namespace",
					Expression: "object.metadata.namespace == 'production'",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.spec.replicas <= 3",
					Message:    "Production deployments must have at most 3 replicas",
				},
			},
		},
	}

	tests := []struct {
		name        string
		namespace   string
		replicas    int64
		expectMatch bool
		expectAllow bool
		description string
	}{
		{
			name:        "production-deployment-valid",
			namespace:   "production",
			replicas:    2,
			expectMatch: true,
			expectAllow: true,
			description: "Production deployment with valid replica count should be allowed",
		},
		{
			name:        "production-deployment-invalid",
			namespace:   "production",
			replicas:    5,
			expectMatch: true,
			expectAllow: false,
			description: "Production deployment with invalid replica count should be rejected",
		},
		{
			name:        "staging-deployment-ignored",
			namespace:   "staging",
			replicas:    10,
			expectMatch: false,
			expectAllow: true,
			description: "Staging deployment should be ignored (policy doesn't match)",
		},
		{
			name:        "default-deployment-ignored",
			namespace:   "default",
			replicas:    15,
			expectMatch: false,
			expectAllow: true,
			description: "Default namespace deployment should be ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test object
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": tt.namespace,
					},
					"spec": map[string]interface{}{
						"replicas": tt.replicas,
					},
				},
			}

			objJSON, _ := obj.MarshalJSON()
			testCase := vaptestv1.TestCase{
				Name: tt.name,
				Object: runtime.RawExtension{
					Raw: objJSON,
				},
				Operation: "CREATE",
				Expected: vaptestv1.ExpectedResult{
					Allowed: tt.expectAllow,
				},
			}

			result, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase)
			require.NoError(t, err, "Simulation should not error")

			assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
			assert.Equal(t, tt.expectAllow, result.ActualResponse.Allowed, tt.description)

			if !tt.expectMatch {
				// For non-matching cases, verify that the policy was skipped
				assert.Empty(t, result.ActualResponse.Message, "Non-matching policies should not generate messages")
			}
		})
	}
}

func TestMatchConditionsWithMultipleConditions(t *testing.T) {
	simulator, err := NewPolicySimulator()
	require.NoError(t, err)

	// Policy with multiple matchConditions - ALL must be true
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multi-condition-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "production-namespace",
					Expression: "object.metadata.namespace == 'production'",
				},
				{
					Name:       "has-app-label",
					Expression: "has(object.metadata.labels.app)",
				},
				{
					Name:       "deployment-kind",
					Expression: "object.kind == 'Deployment'",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.spec.replicas >= 2",
					Message:    "Production deployments must have at least 2 replicas for HA",
				},
			},
		},
	}

	tests := []struct {
		name        string
		object      map[string]interface{}
		expectMatch bool
		expectAllow bool
		description string
	}{
		{
			name: "all-conditions-match",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "production",
					"labels": map[string]interface{}{
						"app": "my-app",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
			expectMatch: true,
			expectAllow: true,
			description: "All conditions match, validation passes",
		},
		{
			name: "all-conditions-match-validation-fails",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "production",
					"labels": map[string]interface{}{
						"app": "my-app",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(1),
				},
			},
			expectMatch: true,
			expectAllow: false,
			description: "All conditions match, but validation fails",
		},
		{
			name: "wrong-namespace",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "staging",
					"labels": map[string]interface{}{
						"app": "my-app",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(1),
				},
			},
			expectMatch: false,
			expectAllow: true,
			description: "Wrong namespace - policy should not match",
		},
		{
			name: "missing-app-label",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "production",
					"labels":    map[string]interface{}{},
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
			expectMatch: false,
			expectAllow: true,
			description: "Missing app label - policy should not match",
		},
		{
			name: "wrong-kind",
			object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name":      "test-service",
					"namespace": "production",
					"labels": map[string]interface{}{
						"app": "my-app",
					},
				},
				"spec": map[string]interface{}{
					"type": "ClusterIP",
				},
			},
			expectMatch: false,
			expectAllow: true,
			description: "Wrong kind - policy should not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: tt.object}

			objJSON, _ := obj.MarshalJSON()
			testCase := vaptestv1.TestCase{
				Name: tt.name,
				Object: runtime.RawExtension{
					Raw: objJSON,
				},
				Operation: "CREATE",
				Expected: vaptestv1.ExpectedResult{
					Allowed: tt.expectAllow,
				},
			}

			result, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase)
			require.NoError(t, err, "Simulation should not error")

			assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
			assert.Equal(t, tt.expectAllow, result.ActualResponse.Allowed, tt.description)
		})
	}
}

func TestMatchConditionsWithComplexExpressions(t *testing.T) {
	simulator, err := NewPolicySimulator()
	require.NoError(t, err)

	// Policy with complex matchConditions using advanced CEL features
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "complex-match-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "high-privilege-workload",
					Expression: "object.spec.template.spec.containers.exists(c, has(c.securityContext.privileged) && c.securityContext.privileged == true)",
				},
				{
					Name:       "production-or-critical-namespace",
					Expression: "object.metadata.namespace in ['production', 'critical']",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.metadata.labels.exists(k, k == 'security-reviewed' && object.metadata.labels[k] == 'true')",
					Message:    "Privileged workloads in production must have security-reviewed=true label",
				},
			},
		},
	}

	tests := []struct {
		name        string
		object      map[string]interface{}
		expectMatch bool
		expectAllow bool
		description string
	}{
		{
			name: "privileged-production-with-approval",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "privileged-app",
					"namespace": "production",
					"labels": map[string]interface{}{
						"app":               "privileged-app",
						"security-reviewed": "true",
					},
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name": "app",
									"securityContext": map[string]interface{}{
										"privileged": true,
									},
								},
							},
						},
					},
				},
			},
			expectMatch: true,
			expectAllow: true,
			description: "Privileged workload in production with security approval should be allowed",
		},
		{
			name: "privileged-production-without-approval",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "privileged-app",
					"namespace": "production",
					"labels": map[string]interface{}{
						"app": "privileged-app",
					},
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name": "app",
									"securityContext": map[string]interface{}{
										"privileged": true,
									},
								},
							},
						},
					},
				},
			},
			expectMatch: true,
			expectAllow: false,
			description: "Privileged workload in production without security approval should be rejected",
		},
		{
			name: "unprivileged-production",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "normal-app",
					"namespace": "production",
					"labels": map[string]interface{}{
						"app": "normal-app",
					},
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name": "app",
									"securityContext": map[string]interface{}{
										"privileged": false,
									},
								},
							},
						},
					},
				},
			},
			expectMatch: false,
			expectAllow: true,
			description: "Non-privileged workload should not match the policy",
		},
		{
			name: "privileged-staging",
			object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "privileged-app",
					"namespace": "staging",
					"labels": map[string]interface{}{
						"app": "privileged-app",
					},
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name": "app",
									"securityContext": map[string]interface{}{
										"privileged": true,
									},
								},
							},
						},
					},
				},
			},
			expectMatch: false,
			expectAllow: true,
			description: "Privileged workload in staging namespace should not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: tt.object}

			objJSON, _ := obj.MarshalJSON()
			testCase := vaptestv1.TestCase{
				Name: tt.name,
				Object: runtime.RawExtension{
					Raw: objJSON,
				},
				Operation: "CREATE",
				Expected: vaptestv1.ExpectedResult{
					Allowed: tt.expectAllow,
				},
			}

			result, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase)
			require.NoError(t, err, "Simulation should not error")

			assert.True(t, result.Success, "Test case should succeed: %s", result.Details)
			assert.Equal(t, tt.expectAllow, result.ActualResponse.Allowed, tt.description)
		})
	}
}