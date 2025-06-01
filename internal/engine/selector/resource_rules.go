package selector

import (
	"strings"

	"github.com/yashirook/kube-vap-test/internal/engine/admission"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// matchesResourceRules evaluates whether resource rules match
func (m *DefaultMatcher) matchesResourceRules(rules []admissionregistrationv1.NamedRuleWithOperations, admissionTarget admission.AdmissionTarget, matchPolicy admission.MatchPolicy) bool {
	// If rules are empty, no match
	if len(rules) == 0 {
		return false
	}

	// Validate required fields
	if admissionTarget.Resource == "" || admissionTarget.Operation == "" {
		return false
	}

	// OK if any rule matches
	for _, rule := range rules {
		// Operation type matching
		if !matchesOperationType(rule.Operations, admissionTarget.Operation) {
			continue
		}

		// API group matching
		if !matchesAPIGroups(rule.APIGroups, admissionTarget.APIGroup) {
			continue
		}

		// API version matching (considering matchPolicy)
		if !matchesAPIVersions(rule.APIVersions, admissionTarget.APIVersion, matchPolicy) {
			continue
		}

		// Resource type matching
		if !matchesResources(rule.Resources, admissionTarget.Resource, admissionTarget.SubResource) {
			continue
		}

		// If all conditions are met
		return true
	}

	return false
}

// matchesOperationType checks if the operation matches the rule's operation list
func matchesOperationType(operations []admissionregistrationv1.OperationType, operation string) bool {
	// Always match if operation list is empty
	if len(operations) == 0 {
		return true
	}

	// Match all operations if "*" is present
	for _, op := range operations {
		if string(op) == "*" || strings.EqualFold(string(op), operation) {
			return true
		}
	}

	return false
}

// matchesOperation checks if the operation matches the rule's operation list
func matchesOperation(operations []string, operation string) bool {
	// Always match if operation list is empty
	if len(operations) == 0 {
		return true
	}

	// Match all operations if "*" is present
	for _, op := range operations {
		if op == "*" || strings.EqualFold(op, operation) {
			return true
		}
	}

	return false
}

// matchesAPIGroups checks if the API group matches the rule's API group list
func matchesAPIGroups(apiGroups []string, apiGroup string) bool {
	// Always match if API group list is empty
	if len(apiGroups) == 0 {
		return true
	}

	// Match all API groups if "*" is present
	for _, group := range apiGroups {
		if group == "*" || group == apiGroup {
			return true
		}
	}

	return false
}

// matchesResources checks if the resource matches the rule's resource list
func matchesResources(resources []string, resource string, subResource string) bool {
	// Always match if resource list is empty
	if len(resources) == 0 {
		return true
	}

	resourceToMatch := resource
	if subResource != "" {
		resourceToMatch = resource + "/" + subResource
	}

	// Match all resources if "*" is present
	for _, r := range resources {
		if r == "*" {
			return true
		}

		// Check for exact match
		if r == resourceToMatch {
			return true
		}

		// If subresource exists, don't allow matching with just the base resource name
		if subResource != "" && r == resource {
			return false
		}
	}

	return false
}

// matchesAPIVersions checks if the API version matches the rule's API version list
func matchesAPIVersions(apiVersions []string, apiVersion string, matchPolicy admission.MatchPolicy) bool {
	// Always match if API version list is empty
	if len(apiVersions) == 0 {
		return true
	}

	// Match all API versions if "*" is present
	for _, version := range apiVersions {
		if version == "*" || version == apiVersion {
			return true
		}

		// For Equivalent policy, also check compatible versions
		if matchPolicy == admission.Equivalent {
			// Example: match when apiVersion="v1", version="v1beta1"
			// Actual implementation needs to follow Kubernetes version compatibility rules
			if areVersionsEquivalent(version, apiVersion) {
				return true
			}
		}
	}

	return false
}

// areVersionsEquivalent determines if two API versions are compatible
// Actual implementation needs to implement detailed Kubernetes version compatibility rules
func areVersionsEquivalent(version1, version2 string) bool {
	// Simplified implementation: consider "v1" compatible with "v1beta1", "v1alpha1", etc.
	// Actual implementation requires more detailed rules

	// Check if based on the same major version
	if strings.HasPrefix(version1, "v") && strings.HasPrefix(version2, "v") {
		v1Base := strings.Split(version1[1:], "beta")[0]
		v1Base = strings.Split(v1Base, "alpha")[0]

		v2Base := strings.Split(version2[1:], "beta")[0]
		v2Base = strings.Split(v2Base, "alpha")[0]

		return v1Base == v2Base
	}

	return false
}
