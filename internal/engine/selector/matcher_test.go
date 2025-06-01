package selector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yashirook/kube-vap-test/internal/engine/admission"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test case struct
type matcherTestCase struct {
	name            string
	matchResources  *admissionregistrationv1.MatchResources
	admissionTarget admission.AdmissionTarget
	expected        bool
}

func TestMatches(t *testing.T) {
	matcher := NewDefaultMatcher()

	// Test cases
	testCases := []matcherTestCase{
		{
			name:           "Nil MatchResources",
			matchResources: nil,
			admissionTarget: admission.AdmissionTarget{
				Object: map[string]interface{}{},
			},
			expected: true,
		},
		{
			name: "Name Namespace Selector - Match",
			matchResources: &admissionregistrationv1.MatchResources{
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
			admissionTarget: admission.AdmissionTarget{
				Namespace: "default",
			},
			expected: true,
		},
		{
			name: "Name Namespace Selector - No Match",
			matchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: "NotIn",
							Values:   []string{"kube-system", "default"},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Namespace: "default",
			},
			expected: false,
		},
		{
			name: "Object Label Selector - Match",
			matchResources: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Labels: map[string]string{
					"app": "test",
				},
			},
			expected: true,
		},
		{
			name: "Object Label Selector - No Match",
			matchResources: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Labels: map[string]string{
					"app": "other",
				},
			},
			expected: false,
		},
		{
			name: "Resource Rules - Match",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true,
		},
		{
			name: "Resource Rules - No Match (wrong operation)",
			matchResources: &admissionregistrationv1.MatchResources{
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
			admissionTarget: admission.AdmissionTarget{
				Operation:  "DELETE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: false,
		},
		{
			name: "Resource Rules - No Match (wrong resource)",
			matchResources: &admissionregistrationv1.MatchResources{
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
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "services",
			},
			expected: false,
		},
		{
			name: "Multiple Conditions - All Match",
			matchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: "NotIn",
							Values:   []string{"kube-system"},
						},
					},
				},
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
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
			admissionTarget: admission.AdmissionTarget{
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true,
		},
		{
			name: "Multiple Conditions - One Not Match",
			matchResources: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: "NotIn",
							Values:   []string{"kube-system"},
						},
					},
				},
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
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
			admissionTarget: admission.AdmissionTarget{
				Namespace: "default",
				Labels: map[string]string{
					"app": "other",
				},
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: false,
		},
		{
			name: "ExcludeResourceRules - Match and Exclude",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods", "services"},
							},
						},
					},
				},
				ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"services"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "services", // Matches exclude list
			},
			expected: false,
		},
		{
			name: "ExcludeResourceRules - Match but Not Exclude",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods", "services"},
							},
						},
					},
				},
				ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"services"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods", // Does not match exclude list
			},
			expected: true,
		},
	}

	// Using MatchPolicyType
	exactMatch := admissionregistrationv1.Exact
	equivalentMatch := admissionregistrationv1.Equivalent

	// MatchPolicy related test cases
	matchPolicyTestCases := []matcherTestCase{
		{
			name: "MatchPolicy Exact - Exact Match",
			matchResources: &admissionregistrationv1.MatchResources{
				MatchPolicy: &exactMatch,
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
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true,
		},
		{
			name: "MatchPolicy Exact - No Match for Different Version",
			matchResources: &admissionregistrationv1.MatchResources{
				MatchPolicy: &exactMatch,
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1beta1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: false,
		},
		{
			name: "MatchPolicy Equivalent - Match for Different but Compatible Version",
			matchResources: &admissionregistrationv1.MatchResources{
				MatchPolicy: &equivalentMatch,
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1beta1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true,
		},
	}

	// High priority test cases
	highPriorityTests := []matcherTestCase{
		// 1. SubResource tests
		{
			name: "SubResource - Direct Match",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"UPDATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods/status"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:   "UPDATE",
				APIGroup:    "",
				APIVersion:  "v1",
				Resource:    "pods",
				SubResource: "status",
			},
			expected: true,
		},
		{
			name: "SubResource - No Match with Base Resource",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"UPDATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:   "UPDATE",
				APIGroup:    "",
				APIVersion:  "v1",
				Resource:    "pods",
				SubResource: "status",
			},
			expected: false,
		},
		// 2. Wildcard operation tests
		{
			name: "Wildcard - All APIGroups",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "apps", // Any API group
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true,
		},
		{
			name: "Wildcard - All Resources",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"*"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "services", // Any resource
			},
			expected: true,
		},
		{
			name: "Wildcard - All Operations",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"*"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "DELETE", // Any operation
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true,
		},
		// 3. Resource rules edge cases
		{
			name: "Edge Case - Empty ResourceRules",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{},
				// Don't set other selectors
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true, // When ResourceRules is an empty array, resource rule checks are skipped and thus allowed
		},
		{
			name: "Edge Case - ResourceRules with Empty Rule",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						// Empty rule
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{},
								APIVersions: []string{},
								Resources:   []string{},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: true, // Empty rules always match
		},
		{
			name: "Edge Case - Missing Resource",
			matchResources: &admissionregistrationv1.MatchResources{
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
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "", // Empty resource
			},
			expected: false,
		},
		{
			name: "Edge Case - Missing Operation",
			matchResources: &admissionregistrationv1.MatchResources{
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
			admissionTarget: admission.AdmissionTarget{
				Operation:  "", // Empty operation
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
			},
			expected: false,
		},
	}

	// Medium priority test cases
	mediumPriorityTests := []matcherTestCase{
		// 4. Complex label selectors
		{
			name: "Complex Label Selector - Exists",
			matchResources: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "environment",
							Operator: "Exists",
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Labels: map[string]string{
					"environment": "production",
				},
			},
			expected: true,
		},
		{
			name: "Complex Label Selector - DoesNotExist",
			matchResources: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "environment",
							Operator: "DoesNotExist",
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Labels: map[string]string{
					"app": "nginx",
				},
			},
			expected: true,
		},
		{
			name: "Complex Label Selector - Multiple Conditions",
			matchResources: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "environment",
							Operator: "In",
							Values:   []string{"production", "staging"},
						},
						{
							Key:      "tier",
							Operator: "NotIn",
							Values:   []string{"backend"},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Labels: map[string]string{
					"app":         "nginx",
					"environment": "production",
					"tier":        "frontend",
				},
			},
			expected: true,
		},
		// 5. Combination of MatchPolicy and ExcludeResourceRules
		{
			name: "MatchPolicy and ExcludeResourceRules - Exclude Matches with Equivalent",
			matchResources: &admissionregistrationv1.MatchResources{
				MatchPolicy: &equivalentMatch,
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
				ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1beta1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "apps",
				APIVersion: "v1",
				Resource:   "deployments",
			},
			expected: false, // v1 and v1beta1 are compatible, so they match the exclude rule
		},
		// 6. Non-empty API groups
		{
			name: "Non-empty API Group - apps",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "apps",
				APIVersion: "v1",
				Resource:   "deployments",
			},
			expected: true,
		},
		{
			name: "Non-empty API Group - networking.k8s.io",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"networking.k8s.io"},
								APIVersions: []string{"v1"},
								Resources:   []string{"ingresses"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "networking.k8s.io",
				APIVersion: "v1",
				Resource:   "ingresses",
			},
			expected: true,
		},
		{
			name: "Multiple API Groups",
			matchResources: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps", "batch", "extensions"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
			admissionTarget: admission.AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "batch",
				APIVersion: "v1",
				Resource:   "deployments",
			},
			expected: true,
		},
	}

	// Combine test cases
	testCases = append(testCases, matchPolicyTestCases...)
	testCases = append(testCases, highPriorityTests...)
	testCases = append(testCases, mediumPriorityTests...)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare metadata
			tc.admissionTarget.PrepareObject()

			result, err := matcher.Matches(tc.matchResources, tc.admissionTarget)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
