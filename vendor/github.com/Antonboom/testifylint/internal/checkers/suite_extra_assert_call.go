package checkers

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// SuiteExtraAssertCallMode reflects different modes of work of SuiteExtraAssertCall checker.
type SuiteExtraAssertCallMode int

const (
	SuiteExtraAssertCallModeRemove SuiteExtraAssertCallMode = iota
	SuiteExtraAssertCallModeRequire
)

const DefaultSuiteExtraAssertCallMode = SuiteExtraAssertCallModeRemove

// SuiteExtraAssertCall detects situations like
//
//	func (s *MySuite) TestSomething() {
//		s.Assert().Equal(42, value)
//	}
//
// and requires
//
//	func (s *MySuite) TestSomething() {
//		s.Equal(42, value)
//	}
//
// or vice versa (depending on the configurable mode).
type SuiteExtraAssertCall struct {
	mode SuiteExtraAssertCallMode
}

// NewSuiteExtraAssertCall constructs SuiteExtraAssertCall checker.
func NewSuiteExtraAssertCall() *SuiteExtraAssertCall {
	return &SuiteExtraAssertCall{mode: DefaultSuiteExtraAssertCallMode}
}

func (SuiteExtraAssertCall) Name() string { return "suite-extra-assert-call" }

func (checker *SuiteExtraAssertCall) SetMode(m SuiteExtraAssertCallMode) *SuiteExtraAssertCall {
	checker.mode = m
	return checker
}

func (checker SuiteExtraAssertCall) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if call.IsPkg {
		return nil
	}

	switch checker.mode {
	case SuiteExtraAssertCallModeRequire:
		x, ok := call.Selector.X.(*ast.Ident) // s.True
		if !ok || x == nil || !implementsTestifySuite(pass, x) {
			return nil
		}

		msg := fmt.Sprintf("use an explicit %s.Assert().%s", analysisutil.NodeString(pass.Fset, x), call.Fn.Name)
		return newDiagnostic(checker.Name(), call, msg, analysis.SuggestedFix{
			Message: "Add `Assert()` call",
			TextEdits: []analysis.TextEdit{{
				Pos:     x.End(),
				End:     x.End(), // Pure insertion.
				NewText: []byte(".Assert()"),
			}},
		})

	case SuiteExtraAssertCallModeRemove:
		x, ok := call.Selector.X.(*ast.CallExpr) // s.Assert().True
		if !ok {
			return nil
		}

		se, ok := x.Fun.(*ast.SelectorExpr)
		if !ok || se == nil || !implementsTestifySuite(pass, se.X) {
			return nil
		}
		if se.Sel == nil || se.Sel.Name != "Assert" {
			return nil
		}

		msg := fmt.Sprintf("need to simplify the assertion to %s.%s", analysisutil.NodeString(pass.Fset, se.X), call.Fn.Name)
		return newDiagnostic(checker.Name(), call, msg, analysis.SuggestedFix{
			Message: "Remove `Assert()` call",
			TextEdits: []analysis.TextEdit{{
				Pos:     se.Sel.Pos(),
				End:     x.End() + 1, // +1 for dot.
				NewText: []byte(""),
			}},
		})
	}

	return nil
}
