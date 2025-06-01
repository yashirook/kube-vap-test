package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
)

// KubernetesLib is a custom library for Kubernetes-related CEL functions
type KubernetesLib struct{}

// CompileOptions returns compile-time options
func (KubernetesLib) CompileOptions() []cel.EnvOption {
	// Use the standard CEL string library which includes endsWith
	return []cel.EnvOption{
		ext.Strings(),
	}
}

// ProgramOptions returns program execution options
func (KubernetesLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// NewKubernetesLib returns a cel.Library that implements Kubernetes-specific functions
func NewKubernetesLib() cel.Library {
	return KubernetesLib{}
}