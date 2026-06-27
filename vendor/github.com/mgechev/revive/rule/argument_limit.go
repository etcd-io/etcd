package rule

import (
	"errors"
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/lint"
)

// ArgumentsLimitRule lints the number of arguments a function can receive.
type ArgumentsLimitRule struct {
	max int
}

const defaultArgumentsLimit = 8

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *ArgumentsLimitRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		r.max = defaultArgumentsLimit
		return nil
	}

	maxArguments, ok := arguments[0].(int64) // Alt. non panicking version
	if !ok {
		return errors.New(`invalid value passed as argument number to the "argument-limit" rule`)
	}
	r.max = int(maxArguments)
	return nil
}

// Apply applies the rule to given file.
func (r *ArgumentsLimitRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	for _, decl := range file.AST.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		numParams := 0
		for _, l := range funcDecl.Type.Params.List {
			numParams += len(l.Names)
		}

		if numParams <= r.max {
			continue
		}

		failures = append(failures, lint.Failure{
			Confidence: 1,
			Failure:    fmt.Sprintf("maximum number of arguments per function exceeded; max %d but got %d", r.max, numParams),
			Node:       funcDecl.Type,
		})
	}

	return failures
}

// Name returns the rule name.
func (*ArgumentsLimitRule) Name() string {
	return "argument-limit"
}
