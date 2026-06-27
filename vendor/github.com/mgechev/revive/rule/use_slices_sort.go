package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UseSlicesSort spots calls to sort.* that can be replaced by [slices] package methods.
type UseSlicesSort struct{}

// Apply applies the rule to given file.
func (*UseSlicesSort) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if !file.Pkg.IsAtLeastGoVersion(lint.Go121) {
		return nil // nothing to do, the package slices was added in version 1.21
	}

	var failures []lint.Failure

	walker := lintSort{
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	ast.Walk(walker, file.AST)

	return failures
}

// Name returns the rule name.
func (*UseSlicesSort) Name() string {
	return "use-slices-sort"
}

type lintSort struct {
	onFailure func(lint.Failure)
}

func (w lintSort) Visit(n ast.Node) ast.Visitor {
	funcCall, ok := n.(*ast.CallExpr)
	if !ok {
		return w // not a function call
	}

	isCallToSort, sortMethod, sliceMethod := findCallToSortReplacement(funcCall.Fun)
	if !isCallToSort {
		return w
	}

	w.onFailure(lint.Failure{
		Category:   lint.FailureCategoryMaintenance,
		Node:       n,
		Confidence: 1,
		Failure:    fmt.Sprintf("replace sort.%s by slices.%s", sortMethod, sliceMethod),
	})

	return nil
}

// findCallToSortReplacement returns true if the given function call is a call to a sort method that
// can be replaced with a call to a slices package method, false otherwise.
// Alongside with the boolean, the function returns the sort method name and the name of its
// replacement method from the slices package.
func findCallToSortReplacement(expr ast.Expr) (isCallToSort bool, sortMethod, slicesMethod string) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false, "", ""
	}

	if !astutils.IsIdent(sel.X, "sort") {
		return false, "", ""
	}

	sortMethod = sel.Sel.Name
	switch sortMethod {
	case "Float64s", "Ints", "Strings":
		slicesMethod = "Sort"
	case "Slice", "Sort":
		slicesMethod = "SortFunc"
	case "SliceStable", "Stable":
		slicesMethod = "SortStableFunc"
	case "Float64sAreSorted", "IntsAreSorted", "StringsAreSorted":
		slicesMethod = "IsSorted"
	case "IsSorted", "SliceIsSorted":
		slicesMethod = "IsSortedFunc"
	default:
		return false, "", ""
	}

	return true, sortMethod, slicesMethod
}
