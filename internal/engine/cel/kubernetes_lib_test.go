package cel

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
)

func TestKubernetesLib_Integration(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		vars    map[string]interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name: "endsWith function - true case",
			expr: `"hello world".endsWith("world")`,
			want: true,
		},
		{
			name: "endsWith function - false case",
			expr: `"hello world".endsWith("hello")`,
			want: false,
		},
		{
			name: "endsWith with empty suffix",
			expr: `"hello".endsWith("")`,
			want: true,
		},
		{
			name: "startsWith function - true case",
			expr: `"hello world".startsWith("hello")`,
			want: true,
		},
		{
			name: "startsWith function - false case",
			expr: `"hello world".startsWith("world")`,
			want: false,
		},
		{
			name: "endsWith with variable",
			expr: `filename.endsWith(".txt")`,
			vars: map[string]interface{}{
				"filename": "document.txt",
			},
			want: true,
		},
		{
			name: "complex string operations",
			expr: `name.startsWith("test_") && name.endsWith("_spec")`,
			vars: map[string]interface{}{
				"name": "test_validation_spec",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create environment with KubernetesLib
			env, err := cel.NewEnv(
				cel.Lib(NewKubernetesLib()),
			)
			
			// Add variables if present
			if tt.vars != nil {
				opts := make([]cel.EnvOption, 0, len(tt.vars))
				for name := range tt.vars {
					opts = append(opts, cel.Variable(name, cel.DynType))
				}
				env, err = env.Extend(opts...)
				assert.NoError(t, err)
			}
			
			assert.NoError(t, err)

			// Compile expression
			ast, issues := env.Compile(tt.expr)
			if tt.wantErr {
				assert.NotNil(t, issues)
				assert.True(t, issues.Err() != nil)
				return
			}
			assert.NoError(t, issues.Err())

			// Create program
			prg, err := env.Program(ast)
			assert.NoError(t, err)

			// Evaluate
			out, _, err := prg.Eval(tt.vars)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, out.Value())
		})
	}
}

func TestKubernetesLib_CompileOptions(t *testing.T) {
	lib := NewKubernetesLib()
	opts := lib.(KubernetesLib).CompileOptions()
	assert.NotEmpty(t, opts, "CompileOptions should not be empty")
}

func TestKubernetesLib_ProgramOptions(t *testing.T) {
	lib := NewKubernetesLib()
	opts := lib.(KubernetesLib).ProgramOptions()
	assert.Empty(t, opts, "ProgramOptions should be empty for now")
}

func TestNewKubernetesLib(t *testing.T) {
	lib := NewKubernetesLib()
	assert.NotNil(t, lib)
	_, ok := lib.(cel.Library)
	assert.True(t, ok, "NewKubernetesLib should return a cel.Library")
}