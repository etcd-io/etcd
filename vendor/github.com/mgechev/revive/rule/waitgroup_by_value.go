package rule

import (
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// WaitGroupByValueRule lints [sync.WaitGroup] passed by copy in functions.
type WaitGroupByValueRule struct{}

// Apply applies the rule to given file.
func (*WaitGroupByValueRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := lintWaitGroupByValueRule{onFailure: onFailure}
	ast.Walk(w, file.AST)
	return failures
}

// Name returns the rule name.
func (*WaitGroupByValueRule) Name() string {
	return "waitgroup-by-value"
}

type lintWaitGroupByValueRule struct {
	onFailure func(lint.Failure)
}

func (w lintWaitGroupByValueRule) Visit(node ast.Node) ast.Visitor {
	// look for function declarations
	fd, ok := node.(*ast.FuncDecl)
	if !ok {
		return w
	}

	// Check all function parameters
	for _, field := range fd.Type.Params.List {
		if !astutils.IsPkgDotName(field.Type, "sync", "WaitGroup") {
			continue
		}

		w.onFailure(lint.Failure{
			Confidence: 1,
			Node:       field,
			Failure:    "sync.WaitGroup passed by value, the function will get a copy of the original one",
		})
	}

	return nil // skip visiting function body
}
