package checkers

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// SuiteDontUsePkg detects situations like
//
//	func (s *MySuite) TestSomething() {
//		assert.Equal(s.T(), 42, value)
//	}
//
// and requires
//
//	func (s *MySuite) TestSomething() {
//		s.Equal(42, value)
//	}
type SuiteDontUsePkg struct{}

// NewSuiteDontUsePkg constructs SuiteDontUsePkg checker.
func NewSuiteDontUsePkg() SuiteDontUsePkg { return SuiteDontUsePkg{} }
func (SuiteDontUsePkg) Name() string      { return "suite-dont-use-pkg" }

func (checker SuiteDontUsePkg) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if !call.IsPkg {
		return nil
	}

	args := call.ArgsRaw
	if len(args) < 2 {
		return nil
	}
	t := args[0]

	ce, ok := t.(*ast.CallExpr)
	if !ok {
		return nil
	}
	se, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	if se.X == nil || !implementsTestifySuite(pass, se.X) {
		return nil
	}
	if se.Sel == nil || se.Sel.Name != "T" {
		return nil
	}
	rcv, ok := se.X.(*ast.Ident) // At this point we ensure that `s.T()` is used as the first argument of assertion.
	if !ok {
		return nil
	}

	newSelector := rcv.Name
	if !call.IsAssert {
		newSelector += "." + "Require()"
	}

	msg := fmt.Sprintf("use %s.%s", newSelector, call.Fn.Name)
	return newDiagnostic(checker.Name(), call, msg, analysis.SuggestedFix{
		Message: fmt.Sprintf("Replace `%s` with `%s`", call.SelectorXStr, newSelector),
		TextEdits: []analysis.TextEdit{
			// Replace package function with suite method.
			{
				Pos:     call.Selector.X.Pos(),
				End:     call.Selector.X.End(),
				NewText: []byte(newSelector),
			},
			// Remove `s.T()`.
			{
				Pos:     t.Pos(),
				End:     args[1].Pos(),
				NewText: []byte(""),
			},
		},
	})
}
