package engine

import (
	"context"
	"testing"

	vaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestVariablesIntegration tests the full integration with variables support
func TestVariablesIntegration(t *testing.T) {
	// Test the full integration with SimulateTestCase
	simulator, err := NewPolicySimulator()
	if err != nil {
		t.Fatalf("Failed to create simulator: %v", err)
	}

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy-with-variables",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "replicaCount",
					Expression: "object.spec.replicas",
				},
				{
					Name:       "isHighReplica",
					Expression: "variables.replicaCount > 10",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "!variables.isHighReplica || object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
					Message:    "High replica deployments must have nginx container",
				},
			},
		},
	}

	// Test case 1: Low replica count (should pass)
	obj1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "app"},
						},
					},
				},
			},
		},
	}

	// Convert to JSON for test case
	objJSON1, _ := obj1.MarshalJSON()
	testCase1 := vaptestv1.TestCase{
		Name: "low-replica-deployment",
		Object: runtime.RawExtension{
			Raw: objJSON1,
		},
		Operation: "CREATE",
		Expected: vaptestv1.ExpectedResult{
			Allowed: true,
		},
	}

	result1, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase1)
	if err != nil {
		t.Fatalf("Failed to simulate test case 1: %v", err)
	}

	if !result1.Success {
		t.Errorf("Test case 1 failed: %s", result1.Details)
	}

	// Test case 2: High replica count without nginx (should fail)
	obj2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": int64(15),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "app"},
						},
					},
				},
			},
		},
	}

	objJSON2, _ := obj2.MarshalJSON()
	testCase2 := vaptestv1.TestCase{
		Name: "high-replica-deployment-without-nginx",
		Object: runtime.RawExtension{
			Raw: objJSON2,
		},
		Operation: "CREATE",
		Expected: vaptestv1.ExpectedResult{
			Allowed: false,
			Message: "High replica deployments must have nginx container",
		},
	}

	result2, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase2)
	if err != nil {
		t.Fatalf("Failed to simulate test case 2: %v", err)
	}

	if !result2.Success {
		t.Errorf("Test case 2 failed: %s", result2.Details)
	}

	// Test case 3: High replica count with nginx (should pass)
	obj3 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": int64(15),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "nginx"},
							map[string]interface{}{"name": "app"},
						},
					},
				},
			},
		},
	}

	objJSON3, _ := obj3.MarshalJSON()
	testCase3 := vaptestv1.TestCase{
		Name: "high-replica-deployment-with-nginx",
		Object: runtime.RawExtension{
			Raw: objJSON3,
		},
		Operation: "CREATE",
		Expected: vaptestv1.ExpectedResult{
			Allowed: true,
		},
	}

	result3, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase3)
	if err != nil {
		t.Fatalf("Failed to simulate test case 3: %v", err)
	}

	if !result3.Success {
		t.Errorf("Test case 3 failed: %s", result3.Details)
	}
}

// TestComplexVariables tests more complex variable scenarios
func TestComplexVariables(t *testing.T) {
	simulator, err := NewPolicySimulator()
	if err != nil {
		t.Fatalf("Failed to create simulator: %v", err)
	}

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "complex-variables-policy",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "containerCount",
					Expression: "size(object.spec.template.spec.containers)",
				},
				{
					Name:       "hasNginx",
					Expression: "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
				},
				{
					Name:       "requiresNginx",
					Expression: "variables.containerCount > 2",
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "!variables.requiresNginx || variables.hasNginx",
					Message:    "Deployments with more than 2 containers must include nginx",
				},
			},
		},
	}

	// Test case: 3 containers without nginx (should fail)
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "app1"},
							map[string]interface{}{"name": "app2"},
							map[string]interface{}{"name": "app3"},
						},
					},
				},
			},
		},
	}

	objJSON, _ := obj.MarshalJSON()
	testCase := vaptestv1.TestCase{
		Name: "multi-container-without-nginx",
		Object: runtime.RawExtension{
			Raw: objJSON,
		},
		Operation: "CREATE",
		Expected: vaptestv1.ExpectedResult{
			Allowed: false,
			Message: "Deployments with more than 2 containers must include nginx",
		},
	}

	result, err := simulator.SimulateTestCase(context.Background(), policy, nil, testCase)
	if err != nil {
		t.Fatalf("Failed to simulate test case: %v", err)
	}

	if !result.Success {
		t.Errorf("Test case failed: %s", result.Details)
	}
}