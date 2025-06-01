package admission

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MatchPolicy represents the API version matching method
type MatchPolicy string

const (
	// Exact matches only resources that exactly match the specified API version
	Exact MatchPolicy = "Exact"

	// Equivalent matches versions that are compatible with the specified version
	Equivalent MatchPolicy = "Equivalent"
)

// AdmissionTarget holds resource information to be evaluated in admission control
type AdmissionTarget struct {
	// Resource object (map format)
	Object map[string]interface{}

	// Operation type (CREATE, UPDATE, DELETE...)
	Operation string

	// Additional information such as API group, version, and resource type
	APIGroup    string
	APIVersion  string
	Resource    string
	SubResource string

	// Convenience fields for testing
	Namespace string
	Labels    map[string]string
}

// PrepareObject sets the metadata of Object from Namespace and Labels fields
// Used as preprocessing for tests
func (a *AdmissionTarget) PrepareObject() {
	if a.Namespace == "" && len(a.Labels) == 0 {
		return
	}

	if a.Object == nil {
		a.Object = map[string]interface{}{}
	}

	metadata := map[string]interface{}{}

	if a.Namespace != "" {
		metadata["namespace"] = a.Namespace
	}

	if len(a.Labels) > 0 {
		labels := map[string]interface{}{}
		for k, v := range a.Labels {
			labels[k] = v
		}
		metadata["labels"] = labels
	}

	a.Object["metadata"] = metadata
}

// NewAdmissionTarget creates a new AdmissionTarget from an unstructured object
func NewAdmissionTarget(obj *unstructured.Unstructured, operation string) *AdmissionTarget {
	target := &AdmissionTarget{
		Operation: operation,
	}

	if obj != nil {
		// Get object as map
		target.Object = obj.UnstructuredContent()

		// Extract API information
		gvk := obj.GroupVersionKind()
		target.APIGroup = gvk.Group
		target.APIVersion = gvk.Version
		target.Resource = gvk.Kind

		// Extract namespace and labels
		target.Namespace = obj.GetNamespace()
		target.Labels = obj.GetLabels()
	}

	return target
}
