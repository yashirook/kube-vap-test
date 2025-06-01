package selector

import (
	"github.com/yashirook/kube-vap-test/internal/engine/admission"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Matcher evaluates whether resources match the matching conditions
type Matcher interface {
	// Matches evaluates whether a resource matches the specified matching conditions
	// matchResources: VAPB matching conditions
	// admissionTarget: Resource information to be evaluated (object, namespace, operation, etc.)
	Matches(matchResources *admissionregistrationv1.MatchResources, admissionTarget admission.AdmissionTarget) (bool, error)
}

// DefaultMatcher implements the standard Matcher
type DefaultMatcher struct{}

// NewDefaultMatcher creates a standard Matcher
func NewDefaultMatcher() Matcher {
	return &DefaultMatcher{}
}

// Matches evaluates whether a resource matches the specified matching conditions
func (m *DefaultMatcher) Matches(matchResources *admissionregistrationv1.MatchResources, admissionTarget admission.AdmissionTarget) (bool, error) {
	// Always match if matchResources is not set
	if matchResources == nil {
		return true, nil
	}

	// Check namespace selector
	if matchResources.NamespaceSelector != nil && !m.matchesNamespaceSelector(matchResources.NamespaceSelector, admissionTarget.Object) {
		return false, nil
	}

	// Check object selector
	if matchResources.ObjectSelector != nil && !m.matchesObjectSelector(matchResources.ObjectSelector, admissionTarget.Object) {
		return false, nil
	}

	// Default value for MatchPolicy is Exact
	matchPolicy := admission.Exact
	if matchResources.MatchPolicy != nil {
		if *matchResources.MatchPolicy == "Equivalent" {
			matchPolicy = admission.Equivalent
		}
	}

	// Check resource rules
	// Note: If resource rules are completely empty, they do not match (consistent with Kubernetes behavior)
	// However, skip this check if ResourceRules is not specified
	if len(matchResources.ResourceRules) > 0 {
		if !m.matchesResourceRules(matchResources.ResourceRules, admissionTarget, matchPolicy) {
			return false, nil
		}
	}

	// Check ExcludeResourceRules
	if len(matchResources.ExcludeResourceRules) > 0 {
		// Exclude if it matches the exclusion rules
		if m.matchesResourceRules(matchResources.ExcludeResourceRules, admissionTarget, matchPolicy) {
			return false, nil
		}
	}

	// If all conditions are met
	return true, nil
}

// matchesNamespaceSelector evaluates whether it matches the namespace selector
func (m *DefaultMatcher) matchesNamespaceSelector(selector *metav1.LabelSelector, object map[string]interface{}) bool {
	// Does not match if the object is not a map
	metadata, ok := object["metadata"].(map[string]interface{})
	if !ok {
		return false
	}

	// Get namespace
	namespace, ok := metadata["namespace"].(string)
	if !ok {
		// If there is no namespace, consider it a cluster-scoped resource
		return true // Or decide based on namespace selector settings
	}

	// Evaluation by MatchExpressions
	if len(selector.MatchExpressions) > 0 {
		for _, expr := range selector.MatchExpressions {
			// Especially evaluation regarding namespace name
			if expr.Key == "kubernetes.io/metadata.name" {
				switch expr.Operator {
				case "In":
					matched := false
					for _, value := range expr.Values {
						if namespace == value {
							matched = true
							break
						}
					}
					if !matched {
						return false
					}
				case "NotIn":
					for _, value := range expr.Values {
						if namespace == value {
							return false // If namespace is in the exclusion list
						}
					}
				}
			}
		}
	}

	return true
}

// matchesObjectSelector evaluates whether it matches the object selector
func (m *DefaultMatcher) matchesObjectSelector(selector *metav1.LabelSelector, object map[string]interface{}) bool {
	// Does not match if the object is not a map
	metadata, ok := object["metadata"].(map[string]interface{})
	if !ok {
		return false
	}

	// Get labels
	labels, ok := metadata["labels"].(map[string]interface{})
	if !ok {
		// If there are no labels, does not match unless the selector is empty
		return len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 0
	}

	// Evaluation by matchLabels
	for key, value := range selector.MatchLabels {
		labelValue, exists := labels[key]
		if !exists || labelValue != value {
			return false
		}
	}

	// Evaluation by matchExpressions
	for _, expr := range selector.MatchExpressions {
		labelValue, exists := labels[expr.Key]

		switch expr.Operator {
		case "In":
			if !exists {
				return false
			}
			matched := false
			for _, value := range expr.Values {
				if labelValue == value {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		case "NotIn":
			if exists {
				for _, value := range expr.Values {
					if labelValue == value {
						return false
					}
				}
			}
		case "Exists":
			if !exists {
				return false
			}
		case "DoesNotExist":
			if exists {
				return false
			}
		}
	}

	return true
}


// Matches is a convenience function that uses the default matcher
func Matches(matchResources *admissionregistrationv1.MatchResources, admissionTarget *admission.AdmissionTarget) bool {
	if matchResources == nil || admissionTarget == nil {
		return true
	}
	
	matcher := NewDefaultMatcher()
	result, _ := matcher.Matches(matchResources, *admissionTarget)
	return result
}
