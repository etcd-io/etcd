package rule

import (
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// ErrorReturnRule ensures that the error return parameter is the last parameter.
type ErrorReturnRule struct{}

// Apply applies the rule to given file.
func (*ErrorReturnRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	for _, decl := range file.AST.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		isFunctionWithMoreThanOneResult := ok && funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 1
		if !isFunctionWithMoreThanOneResult {
			continue
		}

		funcResults := funcDecl.Type.Results.List
		isLastResultError := astutils.IsIdent(funcResults[len(funcResults)-1].Type, "error")
		if isLastResultError {
			continue
		}

		// An error return parameter should be the last parameter.
		// Flag any error parameters found before the last.
		for _, r := range funcResults[:len(funcResults)-1] {
			if astutils.IsIdent(r.Type, "error") {
				failures = append(failures, lint.Failure{
					Category:   lint.FailureCategoryStyle,
					Confidence: 0.9,
					Node:       funcDecl,
					Failure:    "error should be the last type when returning multiple items",
				})

				break // only flag one
			}
		}
	}

	return failures
}

// Name returns the rule name.
func (*ErrorReturnRule) Name() string {
	return "error-return"
}
