package checkers

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// SuiteSubtestRun detects situations like
//
//	s.T().Run("subtest", func(t *testing.T) {
//		assert.Equal(t, 42, result)
//	})
//
// and requires
//
//	s.Run("subtest", func() {
//		s.Equal(42, result)
//	})
type SuiteSubtestRun struct{}

// NewSuiteSubtestRun constructs SuiteSubtestRun checker.
func NewSuiteSubtestRun() SuiteSubtestRun { return SuiteSubtestRun{} }
func (SuiteSubtestRun) Name() string      { return "suite-subtest-run" }

func (checker SuiteSubtestRun) Check(pass *analysis.Pass, insp *inspector.Inspector) (diagnostics []analysis.Diagnostic) {
	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(node ast.Node) {
		ce := node.(*ast.CallExpr) // s.T().Run

		se, ok := ce.Fun.(*ast.SelectorExpr) // s.T() + .Run
		if !ok {
			return
		}
		if !isIdentWithName("Run", se.Sel) {
			return
		}

		tCall, ok := se.X.(*ast.CallExpr) // s.T()
		if !ok {
			return
		}
		tCallSel, ok := tCall.Fun.(*ast.SelectorExpr) // s + .T()
		if !ok {
			return
		}
		if !isIdentWithName("T", tCallSel.Sel) {
			return
		}

		if implementsTestifySuite(pass, tCallSel.X) && implementsTestingT(pass, tCall) {
			msg := fmt.Sprintf("use %s.Run to run subtest", analysisutil.NodeString(pass.Fset, tCallSel.X))
			diagnostics = append(diagnostics, *newDiagnostic(checker.Name(), ce, msg))
		}
	})
	return diagnostics
}
