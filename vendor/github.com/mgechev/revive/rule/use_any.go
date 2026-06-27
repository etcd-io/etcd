package rule

import (
	"go/ast"

	"github.com/mgechev/revive/lint"
)

// UseAnyRule proposes to replace `interface{}` with its alias `any`.
type UseAnyRule struct{}

// Apply applies the rule to given file.
func (*UseAnyRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	walker := lintUseAny{
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}
	fileAst := file.AST
	ast.Walk(walker, fileAst)

	return failures
}

// Name returns the rule name.
func (*UseAnyRule) Name() string {
	return "use-any"
}

type lintUseAny struct {
	onFailure func(lint.Failure)
}

func (w lintUseAny) Visit(n ast.Node) ast.Visitor {
	it, ok := n.(*ast.InterfaceType)
	if !ok {
		return w
	}

	if len(it.Methods.List) != 0 {
		return w // it is not and empty interface
	}

	w.onFailure(lint.Failure{
		Node:       n,
		Confidence: 1,
		Category:   lint.FailureCategoryNaming,
		Failure:    "since Go 1.18 'interface{}' can be replaced by 'any'",
	})

	return w
}
