package loader

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestLoadPolicyBindingsForSingleFile(t *testing.T) {
	// Create local resource loader
	localLoader, err := NewLocalResourceLoader()
	require.NoError(t, err, "Failed to create local resource loader")

	// Test case
	testPath := filepath.Join("test", "policy-binding-test.yaml")
	resourceSource := ResourceSource{
		Type:  SourceTypeLocal,
		Files: []string{testPath},
	}
	bindings, err := localLoader.LoadPolicyBindings(resourceSource)

	// Assertions
	require.NoError(t, err, "Failed to load policy binding")
	require.Len(t, bindings, 1, "Number of loaded policy bindings is different")
	binding := bindings[0]
	assert.NotNil(t, binding, "Loaded policy binding is nil")
	assert.Equal(t, "test-policy-binding", binding.Name, "Policy binding name is different")
	assert.Equal(t, "test-policy", binding.Spec.PolicyName, "Policy name is different")

	// Verify ValidationActions exists
	require.Len(t, binding.Spec.ValidationActions, 1, "ValidationActions length is different")
	assert.Equal(t, admissionregistrationv1.ValidationAction("Deny"), binding.Spec.ValidationActions[0], "ValidationAction is different")

	assert.NotNil(t, binding.Spec.MatchResources, "MatchResources is nil")
	assert.NotNil(t, binding.Spec.MatchResources.NamespaceSelector, "NamespaceSelector is nil")
}

func TestLoadPolicyBindingsForMultipleFiles(t *testing.T) {
	// Create local resource loader
	localLoader, err := NewLocalResourceLoader()
	require.NoError(t, err, "Failed to create local resource loader")

	// Test case
	testPaths := []string{filepath.Join("test", "policy-binding-test.yaml")}
	resourceSource := ResourceSource{
		Type:  SourceTypeLocal,
		Files: testPaths,
	}
	bindings, err := localLoader.LoadPolicyBindings(resourceSource)

	// Assertions
	require.NoError(t, err, "Failed to load policy binding")
	require.Len(t, bindings, 1, "Number of loaded policy bindings is different")
	assert.Equal(t, "test-policy-binding", bindings[0].Name, "Policy binding name is different")
	assert.Equal(t, "test-policy", bindings[0].Spec.PolicyName, "Policy name is different")
}

func TestLoadPolicyBindingsFileNotFound(t *testing.T) {
	// Create local resource loader
	localLoader, err := NewLocalResourceLoader()
	require.NoError(t, err, "Failed to create local resource loader")

	// Test for non-existent file
	nonExistentPath := filepath.Join("test", "non-existent-binding.yaml")
	resourceSource := ResourceSource{
		Type:  SourceTypeLocal,
		Files: []string{nonExistentPath},
	}
	_, err = localLoader.LoadPolicyBindings(resourceSource)

	// Assertions
	assert.Error(t, err, "Loading policy binding from non-existent file did not return an error")
}
