package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ValidatingAdmissionPolicyTest is a resource definition for testing validation policies
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ValidatingAdmissionPolicyTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ValidatingAdmissionPolicyTestSpec   `json:"spec"`
	Status ValidatingAdmissionPolicyTestStatus `json:"status,omitempty"`
}

// ValidatingAdmissionPolicyTestSpec defines the test specification
type ValidatingAdmissionPolicyTestSpec struct {
	// Source defines where to load policies and bindings from
	Source SourceConfig `json:"source"`

	// IncludeParameters indicates whether to load parameters from the same source
	// +optional
	IncludeParameters bool `json:"includeParameters,omitempty"`

	// TestCases is a list of test cases
	TestCases []TestCase `json:"testCases"`
}

// SourceType defines the type of source
type SourceType string

const (
	// SourceTypeLocal indicates resources are loaded from local files
	SourceTypeLocal SourceType = "local"
	// SourceTypeCluster indicates resources are loaded from cluster
	SourceTypeCluster SourceType = "cluster"
)

// SourceConfig defines a unified source for policies, bindings, and parameters
type SourceConfig struct {
	// Type specifies the source type: "local" (default) or "cluster"
	// +optional
	Type SourceType `json:"type,omitempty"`

	// Files is a list of file paths for local source type
	// Can include directories, which will be expanded to all YAML files
	// +optional
	Files []string `json:"files,omitempty"`
}


// TestCase represents a single test case
type TestCase struct {
	// Name is the name of the test case
	Name string `json:"name"`

	// Description is the description of the test case
	// +optional
	Description string `json:"description,omitempty"`

	// Object is the object to test
	// +optional
	Object runtime.RawExtension `json:"object,omitempty"`

	// ObjectFile is the path to a file containing the object to test
	// +optional
	ObjectFile string `json:"objectFile,omitempty"`

	// OldObject is the object before update (used when testing update operations)
	// +optional
	OldObject *runtime.RawExtension `json:"oldObject,omitempty"`

	// OldObjectFile is the path to a file containing the object before update
	// +optional
	OldObjectFile string `json:"oldObjectFile,omitempty"`

	// Operation is the operation to test (CREATE, UPDATE, DELETE, etc.)
	Operation string `json:"operation"`

	// Expected is the expected result of the test
	Expected ExpectedResult `json:"expected"`
}

// ExpectedResult defines the expected result of a test
type ExpectedResult struct {
	// Allowed indicates whether the operation should be allowed
	Allowed bool `json:"allowed"`

	// Reason is the denial reason (if the operation is denied)
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is the denial message (if the operation is denied)
	// +optional
	Message string `json:"message,omitempty"`

	// MessageContains is a substring that should be contained in the message
	// +optional
	MessageContains string `json:"messageContains,omitempty"`
}

// ValidatingAdmissionPolicyTestStatus holds the status of test execution
type ValidatingAdmissionPolicyTestStatus struct {
	// Results is the results of test cases
	// +optional
	Results []TestResult `json:"results,omitempty"`

	// Summary is a summary of test execution
	// +optional
	Summary TestSummary `json:"summary,omitempty"`
}

// TestResult represents the result of a single test case
type TestResult struct {
	// Name is the name of the test case
	Name string `json:"name"`

	// Success indicates whether the test succeeded
	Success bool `json:"success"`

	// Details is the details of the test result
	// +optional
	Details string `json:"details,omitempty"`

	// ActualResponse is the actual response
	// +optional
	ActualResponse *ResponseDetails `json:"actualResponse,omitempty"`

	// PolicyResults is the evaluation results for each policy (for multiple policies)
	// +optional
	PolicyResults []PolicyResult `json:"policyResults,omitempty"`

	// Metadata is additional metadata information (such as resource type)
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PolicyResult represents the evaluation result of a single policy
type PolicyResult struct {
	// PolicyName is the name of the policy
	PolicyName string `json:"policyName"`

	// Allowed indicates whether the operation was allowed
	Allowed bool `json:"allowed"`

	// Reason is the response reason
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is the response message
	// +optional
	Message string `json:"message,omitempty"`
}

// ResponseDetails is the actual response details of policy evaluation
type ResponseDetails struct {
	// Allowed indicates whether the operation was allowed
	Allowed bool `json:"allowed"`

	// Reason is the response reason
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is the response message
	// +optional
	Message string `json:"message,omitempty"`
}

// TestSummary represents a summary of test execution
type TestSummary struct {
	// Total is the total number of test cases
	Total int `json:"total"`

	// Successful is the number of successful test cases
	Successful int `json:"successful"`

	// Failed is the number of failed test cases
	Failed int `json:"failed"`
}
