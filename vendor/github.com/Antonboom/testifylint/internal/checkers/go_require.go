package checkers

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

const (
	goRequireFnReportFormat          = "%s contains assertions that must only be used in the goroutine running the test function"
	goRequireCallReportFormat        = "%s must only be used in the goroutine running the test function"
	goRequireHTTPHandlerReportFormat = "do not use %s in http handlers"
)

// GoRequire takes idea from go vet's "testinggoroutine" check
// and detects usage of require package's functions or assert.FailNow in the non-test goroutines
//
//	go func() {
//		conn, err = lis.Accept()
//		require.NoError(t, err)
//
//		if assert.Error(err) {
//			assert.FailNow(t, msg)
//		}
//	}()
type GoRequire struct {
	ignoreHTTPHandlers bool
}

// NewGoRequire constructs GoRequire checker.
func NewGoRequire() *GoRequire { return new(GoRequire) }
func (GoRequire) Name() string { return "go-require" }

func (checker *GoRequire) SetIgnoreHTTPHandlers(v bool) *GoRequire {
	checker.ignoreHTTPHandlers = v
	return checker
}

// Check should be consistent with
// https://cs.opensource.google/go/x/tools/+/master:go/analysis/passes/testinggoroutine/testinggoroutine.go
//
// But due to the fact that the Check covers cases missed by go vet,
// the implementation turned out to be terribly complicated.
//
// In simple words, the algorithm is as follows:
//   - we walk along the call tree and store the status, whether we are in the test goroutine or not;
//   - if we are in a test goroutine, then require is allowed, otherwise not;
//   - when we encounter the launch of a subtest or `go` statement, the status changes;
//   - in order to correctly handle the return to the correct status when exiting the current function,
//     we have to store a stack of statuses (inGoroutineRunningTestFunc).
//
// Other test functions called in the test function are also analyzed to make a verdict about the current function.
// This leads to recursion, which the cache of processed functions (processedFuncs) helps reduce the impact of.
// Also, because of this, we have to pre-collect a list of test function declarations (testsDecls).
func (checker GoRequire) Check(pass *analysis.Pass, insp *inspector.Inspector) (diagnostics []analysis.Diagnostic) {
	testsDecls := make(funcDeclarations)
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(node ast.Node) {
		fd := node.(*ast.FuncDecl)

		if isTestingFuncOrMethod(pass, fd) {
			if tf, ok := pass.TypesInfo.ObjectOf(fd.Name).(*types.Func); ok {
				testsDecls[tf] = fd
			}
		}
	})

	var inGoroutineRunningTestFunc boolStack
	processedFuncs := make(map[*ast.FuncDecl]goRequireVerdict)

	nodesFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncType)(nil),
		(*ast.GoStmt)(nil),
		(*ast.CallExpr)(nil),
	}
	insp.Nodes(nodesFilter, func(node ast.Node, push bool) bool {
		if fd, ok := node.(*ast.FuncDecl); ok {
			if !isTestingFuncOrMethod(pass, fd) {
				return false
			}

			if push {
				inGoroutineRunningTestFunc.Push(true)
			} else {
				inGoroutineRunningTestFunc.Pop()
			}
			return true
		}

		if ft, ok := node.(*ast.FuncType); ok {
			if !isTestingAnonymousFunc(pass, ft) {
				return false
			}

			if push {
				inGoroutineRunningTestFunc.Push(true)
			} else {
				inGoroutineRunningTestFunc.Pop()
			}
			return true
		}

		if _, ok := node.(*ast.GoStmt); ok {
			if push {
				inGoroutineRunningTestFunc.Push(false)
			} else {
				inGoroutineRunningTestFunc.Pop()
			}
			return true
		}

		ce := node.(*ast.CallExpr)
		if isSubTestRun(pass, ce) {
			if push {
				// t.Run spawns the new testing goroutine and declines
				// possible warnings from previous "simple" goroutine.
				inGoroutineRunningTestFunc.Push(true)
			} else {
				inGoroutineRunningTestFunc.Pop()
			}
			return true
		}

		if !push {
			return false
		}
		if inGoroutineRunningTestFunc.Len() == 0 {
			// Insufficient info.
			return true
		}
		if inGoroutineRunningTestFunc.Last() {
			// We are in testing goroutine and can skip any assertion checks.
			return true
		}

		testifyCall := NewCallMeta(pass, ce)
		if testifyCall != nil {
			switch checker.checkCall(testifyCall) {
			case goRequireVerdictRequire:
				d := newDiagnostic(checker.Name(), testifyCall, fmt.Sprintf(goRequireCallReportFormat, "require"))
				diagnostics = append(diagnostics, *d)

			case goRequireVerdictAssertFailNow:
				d := newDiagnostic(checker.Name(), testifyCall, fmt.Sprintf(goRequireCallReportFormat, testifyCall))
				diagnostics = append(diagnostics, *d)

			case goRequireVerdictNoExit:
			}
			return false
		}

		// Case of nested function call.
		{
			calledFd := testsDecls.Get(pass, ce)
			if calledFd == nil {
				return true
			}

			if v := checker.checkFunc(pass, calledFd, testsDecls, processedFuncs); v != goRequireVerdictNoExit {
				caller := analysisutil.NodeString(pass.Fset, ce.Fun)
				d := newDiagnostic(checker.Name(), ce, fmt.Sprintf(goRequireFnReportFormat, caller))
				diagnostics = append(diagnostics, *d)
			}
		}
		return true
	})

	if !checker.ignoreHTTPHandlers {
		diagnostics = append(diagnostics, checker.checkHTTPHandlers(pass, insp)...)
	}

	return diagnostics
}

func (checker GoRequire) checkHTTPHandlers(pass *analysis.Pass, insp *inspector.Inspector) (diagnostics []analysis.Diagnostic) {
	insp.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(node ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		if len(stack) < 3 {
			return true
		}

		fID := findSurroundingFunc(pass, stack)
		if fID == nil || !fID.meta.isHTTPHandler {
			return true
		}

		testifyCall := NewCallMeta(pass, node.(*ast.CallExpr))
		if testifyCall == nil {
			return true
		}

		switch checker.checkCall(testifyCall) {
		case goRequireVerdictRequire:
			d := newDiagnostic(checker.Name(), testifyCall, fmt.Sprintf(goRequireHTTPHandlerReportFormat, "require"))
			diagnostics = append(diagnostics, *d)

		case goRequireVerdictAssertFailNow:
			d := newDiagnostic(checker.Name(), testifyCall, fmt.Sprintf(goRequireHTTPHandlerReportFormat, testifyCall))
			diagnostics = append(diagnostics, *d)

		case goRequireVerdictNoExit:
		}
		return false
	})
	return diagnostics
}

func (checker GoRequire) checkFunc(
	pass *analysis.Pass,
	fd *ast.FuncDecl,
	testsDecls funcDeclarations,
	processedFuncs map[*ast.FuncDecl]goRequireVerdict,
) (result goRequireVerdict) {
	if v, ok := processedFuncs[fd]; ok {
		return v
	}

	ast.Inspect(fd, func(node ast.Node) bool {
		if result != goRequireVerdictNoExit {
			return false
		}

		if _, ok := node.(*ast.GoStmt); ok {
			return false
		}

		ce, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		testifyCall := NewCallMeta(pass, ce)
		if testifyCall != nil {
			if v := checker.checkCall(testifyCall); v != goRequireVerdictNoExit {
				result, processedFuncs[fd] = v, v
			}
			return false
		}

		// Case of nested function call.
		{
			calledFd := testsDecls.Get(pass, ce)
			if calledFd == nil {
				return true
			}
			if calledFd == fd {
				// Recursion.
				return true
			}

			if v := checker.checkFunc(pass, calledFd, testsDecls, processedFuncs); v != goRequireVerdictNoExit {
				result = v
				return false
			}
			return true
		}
	})

	return result
}

type goRequireVerdict int

const (
	goRequireVerdictNoExit goRequireVerdict = iota
	goRequireVerdictRequire
	goRequireVerdictAssertFailNow
)

func (checker GoRequire) checkCall(call *CallMeta) goRequireVerdict {
	if !call.IsAssert {
		return goRequireVerdictRequire
	}
	if call.Fn.NameFTrimmed == "FailNow" {
		return goRequireVerdictAssertFailNow
	}
	return goRequireVerdictNoExit
}

type funcDeclarations map[*types.Func]*ast.FuncDecl

// Get returns the declaration of a called function or method.
// Currently, only static calls within the same package are supported, otherwise returns nil.
func (fd funcDeclarations) Get(pass *analysis.Pass, ce *ast.CallExpr) *ast.FuncDecl {
	var obj types.Object

	switch fun := ce.Fun.(type) {
	case *ast.SelectorExpr:
		obj = pass.TypesInfo.ObjectOf(fun.Sel)

	case *ast.Ident:
		obj = pass.TypesInfo.ObjectOf(fun)

	case *ast.IndexExpr:
		if id, ok := fun.X.(*ast.Ident); ok {
			obj = pass.TypesInfo.ObjectOf(id)
		}

	case *ast.IndexListExpr:
		if id, ok := fun.X.(*ast.Ident); ok {
			obj = pass.TypesInfo.ObjectOf(id)
		}
	}

	if tf, ok := obj.(*types.Func); ok {
		return fd[tf]
	}
	return nil
}

type boolStack []bool

func (s boolStack) Len() int {
	return len(s)
}

func (s *boolStack) Push(v bool) {
	*s = append(*s, v)
}

func (s *boolStack) Pop() bool {
	n := len(*s)
	if n == 0 {
		return false
	}

	last := (*s)[n-1]
	*s = (*s)[:n-1]
	return last
}

func (s boolStack) Last() bool {
	n := len(s)
	if n == 0 {
		return false
	}
	return s[n-1]
}
