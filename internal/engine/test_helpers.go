package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"github.com/yashirook/kube-vap-test/internal/loader"
)

// testResourceLoader is a helper struct for loading test resources
type testResourceLoader struct {
	t      *testing.T
	loader *loader.LocalResourceLoader
}

// newTestResourceLoader creates a new test resource loader
func newTestResourceLoader(t *testing.T) *testResourceLoader {
	localLoader, err := loader.NewLocalResourceLoader()
	require.NoError(t, err, "failed to create local resource loader")

	return &testResourceLoader{
		t:      t,
		loader: localLoader,
	}
}

// loadPolicy loads a policy from the specified file
func (l *testResourceLoader) loadPolicy(policyPath string) *admissionregistrationv1.ValidatingAdmissionPolicy {
	source := loader.ResourceSource{
		Type:  loader.SourceTypeLocal,
		Files: []string{policyPath},
	}
	policies, err := l.loader.LoadPolicies(source)
	require.NoError(l.t, err, "failed to load policy: %s", policyPath)
	require.Len(l.t, policies, 1, "expected 1 policy")
	return policies[0]
}

// loadPolicyBinding loads a policy binding from the specified file
func (l *testResourceLoader) loadPolicyBinding(bindingPath string) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	source := loader.ResourceSource{
		Type:  loader.SourceTypeLocal,
		Files: []string{bindingPath},
	}
	bindings, err := l.loader.LoadPolicyBindings(source)
	require.NoError(l.t, err, "failed to load policy binding: %s", bindingPath)
	require.Len(l.t, bindings, 1, "expected 1 binding")
	return bindings[0]
}

// loadDefaultTestPolicy loads the default test policy
func (l *testResourceLoader) loadDefaultTestPolicy() *admissionregistrationv1.ValidatingAdmissionPolicy {
	return l.loadPolicy(filepath.Join("test", "policy-test.yaml"))
}

// loadDefaultTestBinding loads the default test binding
func (l *testResourceLoader) loadDefaultTestBinding() *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return l.loadPolicyBinding(filepath.Join("test", "policy-binding-test.yaml"))
}

// loadInvalidTestBinding loads a binding with an invalid policy name
func (l *testResourceLoader) loadInvalidTestBinding() *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return l.loadPolicyBinding(filepath.Join("test", "policy-binding-invalid.yaml"))
}

// loadWarningTestBinding loads a binding with a warning action
func (l *testResourceLoader) loadWarningTestBinding() *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return l.loadPolicyBinding(filepath.Join("test", "policy-binding-warning.yaml"))
}
