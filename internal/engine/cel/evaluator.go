package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

// Evaluator evaluates CEL expressions with a pre-configured environment
type Evaluator struct {
	env *cel.Env
}

// NewEvaluator creates a new CEL evaluator with the default environment
func NewEvaluator() (*Evaluator, error) {
	env, err := DefaultEnvironment()
	if err != nil {
		return nil, err
	}
	return &Evaluator{env: env}, nil
}

// NewEvaluatorWithEnv creates a new CEL evaluator with a custom environment
func NewEvaluatorWithEnv(env *cel.Env) *Evaluator {
	return &Evaluator{env: env}
}

// Evaluate evaluates a CEL expression with the given variables
func (e *Evaluator) Evaluate(expression string, vars map[string]interface{}) (interface{}, error) {
	// Compile expression
	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", issues.Err())
	}

	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create program: %w", err)
	}

	// Evaluate expression
	out, _, err := program.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	return out.Value(), nil
}

// CompileAndCache compiles an expression and returns a reusable program
func (e *Evaluator) CompileAndCache(expression string) (cel.Program, error) {
	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", issues.Err())
	}

	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create program: %w", err)
	}

	return program, nil
}

// EvaluateProgram evaluates a pre-compiled CEL program
func (e *Evaluator) EvaluateProgram(program cel.Program, vars map[string]interface{}) (interface{}, error) {
	out, _, err := program.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	return out.Value(), nil
}