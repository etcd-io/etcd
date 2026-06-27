package rule

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UnhandledErrorRule warns on unhandled errors returned by function calls.
type UnhandledErrorRule struct {
	ignoreList []*regexp.Regexp
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *UnhandledErrorRule) Configure(arguments lint.Arguments) error {
	for _, arg := range arguments {
		argStr, ok := arg.(string)
		if !ok {
			return fmt.Errorf("invalid argument to the unhandled-error rule. Expecting a string, got %T", arg)
		}

		argStr = strings.Trim(argStr, " ")
		if argStr == "" {
			return errors.New("invalid argument to the unhandled-error rule, expected regular expression must not be empty")
		}

		exp, err := regexp.Compile(argStr)
		if err != nil {
			return fmt.Errorf("invalid argument to the unhandled-error rule: regexp %q does not compile: %w", argStr, err)
		}

		r.ignoreList = append(r.ignoreList, exp)
	}
	return nil
}

// Apply applies the rule to given file.
func (r *UnhandledErrorRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	walker := &lintUnhandledErrors{
		ignoreList: r.ignoreList,
		pkg:        file.Pkg,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	file.Pkg.TypeCheck()
	ast.Walk(walker, file.AST)

	return failures
}

// Name returns the rule name.
func (*UnhandledErrorRule) Name() string {
	return "unhandled-error"
}

type lintUnhandledErrors struct {
	ignoreList []*regexp.Regexp
	pkg        *lint.Package
	onFailure  func(lint.Failure)
}

// Visit looks for statements that are function calls.
// If the called function returns a value of type error a failure will be created.
func (w *lintUnhandledErrors) Visit(node ast.Node) ast.Visitor {
	if n, ok := node.(*ast.ExprStmt); ok {
		fCall, ok := n.X.(*ast.CallExpr)
		if !ok {
			return nil // not a function call
		}

		funcType := w.pkg.TypeOf(fCall)
		if funcType == nil {
			return nil // skip, type info not available
		}

		switch t := funcType.(type) {
		case *types.Named:
			if !w.isTypeError(t) {
				return nil // func call does not return an error
			}

			w.addFailure(fCall)
		default:
			retTypes, ok := funcType.Underlying().(*types.Tuple)
			if !ok {
				return nil // skip, unable to retrieve return type of the called function
			}

			if w.returnsAnError(retTypes) {
				w.addFailure(fCall)
			}
		}
	}
	return w
}

func (w *lintUnhandledErrors) addFailure(n *ast.CallExpr) {
	name := w.funcName(n)
	if w.isIgnoredFunc(name) {
		return
	}

	w.onFailure(lint.Failure{
		Category:   lint.FailureCategoryBadPractice,
		Confidence: 1,
		Node:       n,
		Failure:    fmt.Sprintf("Unhandled error in call to function %v", name),
	})
}

func (w *lintUnhandledErrors) funcName(call *ast.CallExpr) string {
	fn, ok := w.getFunc(call)
	if !ok {
		return astutils.GoFmt(call.Fun)
	}

	name := fn.FullName()
	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, "*", "")

	return name
}

func (w *lintUnhandledErrors) isIgnoredFunc(funcName string) bool {
	for _, pattern := range w.ignoreList {
		if len(pattern.FindString(funcName)) == len(funcName) {
			return true
		}
	}

	return false
}

func (*lintUnhandledErrors) isTypeError(t *types.Named) bool {
	const errorTypeName = "_.error"

	return t.Obj().Id() == errorTypeName
}

func (w *lintUnhandledErrors) returnsAnError(tt *types.Tuple) bool {
	for v := range tt.Variables() {
		nt, ok := v.Type().(*types.Named)
		if ok && w.isTypeError(nt) {
			return true
		}
	}
	return false
}

func (w *lintUnhandledErrors) getFunc(call *ast.CallExpr) (*types.Func, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}

	fn, ok := w.pkg.TypesInfo().ObjectOf(sel.Sel).(*types.Func)
	if !ok {
		return nil, false
	}

	return fn, true
}
