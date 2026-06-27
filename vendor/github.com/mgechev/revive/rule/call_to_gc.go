package rule

import (
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// CallToGCRule lints calls to the garbage collector.
type CallToGCRule struct{}

// Apply applies the rule to given file.
func (*CallToGCRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := lintCallToGC{onFailure}
	ast.Walk(w, file.AST)

	return failures
}

// Name returns the rule name.
func (*CallToGCRule) Name() string {
	return "call-to-gc"
}

type lintCallToGC struct {
	onFailure func(lint.Failure)
}

func (w lintCallToGC) Visit(node ast.Node) ast.Visitor {
	ce, ok := node.(*ast.CallExpr)
	if !ok {
		return w // nothing to do, the node is not a function call
	}

	if !astutils.IsPkgDotName(ce.Fun, "runtime", "GC") {
		return nil // nothing to do, the call is not a call to the Garbage Collector
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       node,
		Category:   lint.FailureCategoryBadPractice,
		Failure:    "explicit call to the garbage collector",
	})

	return w
}
