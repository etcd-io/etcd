package rule

import (
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UnconditionalRecursionRule warns on function calls that will lead to infinite recursion.
type UnconditionalRecursionRule struct{}

// Apply applies the rule to given file.
func (*UnconditionalRecursionRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	// Range over global declarations of the file to detect func/method declarations and analyze them
	for _, decl := range file.AST.Decls {
		n, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue // not a func/method declaration
		}

		if n.Body == nil {
			continue // func/method with empty body => it can not be recursive
		}

		var rec *ast.Ident
		switch {
		case n.Recv == nil:
			rec = nil
		case n.Recv.NumFields() < 1 || len(n.Recv.List[0].Names) < 1:
			rec = &ast.Ident{Name: "_"}
		default:
			rec = n.Recv.List[0].Names[0]
		}

		w := &lintUnconditionalRecursionRule{
			onFailure:   onFailure,
			currentFunc: &funcStatus{&funcDesc{rec, n.Name}, false},
		}

		ast.Walk(w, n.Body)
	}

	return failures
}

// Name returns the rule name.
func (*UnconditionalRecursionRule) Name() string {
	return "unconditional-recursion"
}

type funcDesc struct {
	receiverID *ast.Ident
	id         *ast.Ident
}

func (fd *funcDesc) equal(other *funcDesc) bool {
	receiversAreEqual := (fd.receiverID == nil && other.receiverID == nil) || fd.receiverID != nil && other.receiverID != nil && fd.receiverID.Name == other.receiverID.Name
	idsAreEqual := (fd.id == nil && other.id == nil) || fd.id.Name == other.id.Name

	return receiversAreEqual && idsAreEqual
}

type funcStatus struct {
	funcDesc            *funcDesc
	seenConditionalExit bool
}

type lintUnconditionalRecursionRule struct {
	onFailure     func(lint.Failure)
	currentFunc   *funcStatus
	inGoStatement bool
}

// Visit will traverse function's body we search for calls to the function itself.
// We do not search inside conditional control structures (if, for, switch etc.)
// because any recursive call inside them is conditioned.
// We do search inside conditional control structures are statements
// that will take the control out of the function (return, exit, panic).
// If we find conditional control exits, it means the function is NOT unconditionally-recursive.
// If we find a recursive call before finding any conditional exit, a failure is generated.
// In resume: if we found a recursive call control-dependent from the entry point of
// the function then we raise a failure.
func (w *lintUnconditionalRecursionRule) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.CallExpr:
		// check if call arguments has a recursive call
		for _, arg := range n.Args {
			ast.Walk(w, arg)
		}

		var funcID *ast.Ident
		var selector *ast.Ident
		switch c := n.Fun.(type) {
		case *ast.Ident:
			selector = nil
			funcID = c
		case *ast.SelectorExpr:
			var ok bool
			selector, ok = c.X.(*ast.Ident)
			if !ok { // a.b....Foo()
				return nil
			}
			funcID = c.Sel
		case *ast.FuncLit:
			ast.Walk(w, c.Body) // analyze the body of the function literal
			return nil
		default:
			return w
		}

		if w.currentFunc != nil && // not in a func body
			!w.currentFunc.seenConditionalExit && // there is a conditional exit in the function
			w.currentFunc.funcDesc.equal(&funcDesc{selector, funcID}) {
			w.onFailure(lint.Failure{
				Category:   lint.FailureCategoryLogic,
				Confidence: 0.8,
				Node:       n,
				Failure:    "unconditional recursive call",
			})
		}
		return nil
	case *ast.IfStmt:
		w.updateFuncStatus(n.Body)
		w.updateFuncStatus(n.Else)
		return nil
	case *ast.SelectStmt:
		w.updateFuncStatus(n.Body)
		return nil
	case *ast.RangeStmt:
		w.updateFuncStatus(n.Body)
		return nil
	case *ast.TypeSwitchStmt:
		w.updateFuncStatus(n.Body)
		return nil
	case *ast.SwitchStmt:
		w.updateFuncStatus(n.Body)
		return nil
	case *ast.GoStmt:
		w.inGoStatement = true
		ast.Walk(w, n.Call)
		w.inGoStatement = false
		return nil
	case *ast.ForStmt:
		if n.Cond != nil {
			return nil
		}
		// unconditional loop
		return w
	case *ast.FuncLit:
		if w.inGoStatement {
			return w
		}
		return nil // literal call (closure) is not necessarily an issue
	}

	return w
}

func (w *lintUnconditionalRecursionRule) updateFuncStatus(node ast.Node) {
	if node == nil || w.currentFunc == nil || w.currentFunc.seenConditionalExit {
		return
	}

	w.currentFunc.seenConditionalExit = w.hasControlExit(node)
}

func (*lintUnconditionalRecursionRule) hasControlExit(node ast.Node) bool {
	// isExit returns true if the given node makes control exit the function
	isExit := func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.CallExpr:
			if astutils.IsIdent(n.Fun, "panic") {
				return true
			}
			se, ok := n.Fun.(*ast.SelectorExpr)
			if !ok {
				return false
			}

			id, ok := se.X.(*ast.Ident)
			if !ok {
				return false
			}

			functionName := se.Sel.Name
			pkgName := id.Name
			return isCallToExitFunction(pkgName, functionName, n.Args)
		}

		return false
	}

	return astutils.SeekNode[ast.Node](node, isExit) != nil
}
