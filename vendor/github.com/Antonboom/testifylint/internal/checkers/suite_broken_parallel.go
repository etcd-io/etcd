package checkers

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// SuiteBrokenParallel detects unsupported t.Parallel() call in suite tests
//
//	func (s *MySuite) SetupTest() {
//		s.T().Parallel()
//	}
//
//	// And other hooks...
//
//	func (s *MySuite) TestSomething() {
//		s.T().Parallel()
//
//		for _, tt := range cases {
//			s.Run(tt.name, func() {
//				s.T().Parallel()
//			})
//
//			s.T().Run(tt.name, func(t *testing.T) {
//				t.Parallel()
//			})
//		}
//	}
type SuiteBrokenParallel struct{}

// NewSuiteBrokenParallel constructs SuiteBrokenParallel checker.
func NewSuiteBrokenParallel() SuiteBrokenParallel { return SuiteBrokenParallel{} }
func (SuiteBrokenParallel) Name() string          { return "suite-broken-parallel" }

func (checker SuiteBrokenParallel) Check(pass *analysis.Pass, insp *inspector.Inspector) (diagnostics []analysis.Diagnostic) {
	const report = "testify v1 does not support suite's parallel tests and subtests"

	insp.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(node ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		ce := node.(*ast.CallExpr)

		se, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if !isIdentWithName("Parallel", se.Sel) {
			return true
		}
		if !implementsTestingT(pass, se.X) {
			return true
		}

		for i := len(stack) - 2; i >= 0; i-- {
			fd, ok := stack[i].(*ast.FuncDecl)
			if !ok {
				continue
			}

			if !isSuiteMethod(pass, fd) {
				continue
			}

			nextLine := pass.Fset.Position(ce.Pos()).Line + 1
			d := newDiagnostic(checker.Name(), ce, report, analysis.SuggestedFix{
				Message: fmt.Sprintf("Remove `%s` call", analysisutil.NodeString(pass.Fset, ce)),
				TextEdits: []analysis.TextEdit{
					{
						Pos:     ce.Pos(),
						End:     pass.Fset.File(ce.Pos()).LineStart(nextLine),
						NewText: []byte(""),
					},
				},
			})

			diagnostics = append(diagnostics, *d)
			return false
		}

		return true
	})
	return diagnostics
}
