package rule

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UseFmtPrintRule proposes to replace calls to built-in `print` and `println`
// with their equivalents from [fmt] package.
type UseFmtPrintRule struct{}

// Apply applies the rule to given file.
func (r *UseFmtPrintRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	redefinesPrint, redefinesPrintln := r.analyzeRedefinitions(file.AST.Decls)
	// Here we could check if both are redefined and if it's the case return nil
	// but the check being false 99.9999999% of the cases we don't

	var failures []lint.Failure
	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := lintUseFmtPrint{onFailure, redefinesPrint, redefinesPrintln}
	ast.Walk(w, file.AST)

	return failures
}

// Name returns the rule name.
func (*UseFmtPrintRule) Name() string {
	return "use-fmt-print"
}

type lintUseFmtPrint struct {
	onFailure        func(lint.Failure)
	redefinesPrint   bool
	redefinesPrintln bool
}

func (w lintUseFmtPrint) Visit(node ast.Node) ast.Visitor {
	ce, ok := node.(*ast.CallExpr)
	if !ok {
		return w // nothing to do, the node is not a call
	}

	id, ok := (ce.Fun).(*ast.Ident)
	if !ok {
		return nil
	}

	name := id.Name
	switch name {
	case "print":
		if w.redefinesPrint {
			return nil // it's a call to user-defined print
		}
	case "println":
		if w.redefinesPrintln {
			return nil // it's a call to user-defined println
		}
	default:
		return nil // nothing to do, the call is not println(...) nor print(...)
	}

	callArgs := w.callArgsAsStr(ce.Args)
	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       node,
		Category:   lint.FailureCategoryBadPractice,
		Failure:    fmt.Sprintf(`avoid using built-in function %q, replace it by "fmt.F%s(os.Stderr, %s)"`, name, name, callArgs),
	})

	return w
}

func (lintUseFmtPrint) callArgsAsStr(args []ast.Expr) string {
	strs := []string{}
	for _, expr := range args {
		strs = append(strs, astutils.GoFmt(expr))
	}

	return strings.Join(strs, ", ")
}

func (*UseFmtPrintRule) analyzeRedefinitions(decls []ast.Decl) (redefinesPrint, redefinesPrintln bool) {
	for _, decl := range decls {
		fnDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue // not a function declaration
		}

		if fnDecl.Recv != nil {
			continue // it´s a method (not function) declaration
		}

		switch fnDecl.Name.Name {
		case "print":
			redefinesPrint = true
		case "println":
			redefinesPrintln = true
		}
	}
	return redefinesPrint, redefinesPrintln
}
