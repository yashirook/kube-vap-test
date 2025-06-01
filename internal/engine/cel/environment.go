package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/ext"
	"k8s.io/apiserver/pkg/cel/library"
)

// EnvironmentBuilder builds CEL environments with common configuration
type EnvironmentBuilder struct {
	withVariables bool
	customLibs    []cel.EnvOption
}

// NewEnvironmentBuilder creates a new CEL environment builder
func NewEnvironmentBuilder() *EnvironmentBuilder {
	return &EnvironmentBuilder{}
}

// WithVariables enables variables support in the CEL environment
func (b *EnvironmentBuilder) WithVariables() *EnvironmentBuilder {
	b.withVariables = true
	return b
}

// WithCustomLib adds a custom library to the CEL environment
func (b *EnvironmentBuilder) WithCustomLib(lib cel.EnvOption) *EnvironmentBuilder {
	b.customLibs = append(b.customLibs, lib)
	return b
}

// Build creates the CEL environment with the specified configuration
func (b *EnvironmentBuilder) Build() (*cel.Env, error) {
	opts := []cel.EnvOption{
		cel.OptionalTypes(),
		cel.Declarations(
			decls.NewVar("object", decls.Dyn),
			decls.NewVar("oldObject", decls.Dyn),
			decls.NewVar("request", decls.Dyn),
			decls.NewVar("params", decls.Dyn),
			decls.NewVar("operation", decls.String),
			decls.NewVar("userInfo", decls.Dyn),
		),
	}

	// Add variables declaration if enabled
	if b.withVariables {
		opts = append(opts, cel.Declarations(
			decls.NewVar("variables", decls.NewMapType(decls.String, decls.Dyn)),
		))
	}

	// Add custom libraries
	opts = append(opts, b.customLibs...)

	env, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return env, nil
}

// DefaultEnvironment creates a default CEL environment with all features enabled
func DefaultEnvironment() (*cel.Env, error) {
	return NewEnvironmentBuilder().
		WithVariables().
		WithCustomLib(cel.Lib(NewKubernetesLib())).
		WithCustomLib(library.IP()).
		WithCustomLib(library.CIDR()).
		WithCustomLib(library.Format()).
		WithCustomLib(ext.Strings()).
		Build()
}