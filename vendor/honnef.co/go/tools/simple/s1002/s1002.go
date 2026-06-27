package s1002

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1002",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:  `Omit comparison with boolean constant`,
		Before: `if x == true {}`,
		After:  `if x {}`,
		Since:  "2017.1",
		// MergeIfAll because 'true' might not be the builtin constant under all build tags.
		// You shouldn't write code like that…
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		if code.IsInTest(pass, node) {
			return
		}

		expr := node.(*ast.BinaryExpr)
		if expr.Op != token.EQL && expr.Op != token.NEQ {
			return
		}
		x := code.IsBoolConst(pass, expr.X)
		y := code.IsBoolConst(pass, expr.Y)
		if !x && !y {
			return
		}
		var other ast.Expr
		var val bool
		if x {
			val = code.BoolConst(pass, expr.X)
			other = expr.Y
		} else {
			val = code.BoolConst(pass, expr.Y)
			other = expr.X
		}

		ok := typeutil.All(pass.TypesInfo.TypeOf(other), func(term *types.Term) bool {
			basic, ok := term.Type().Underlying().(*types.Basic)
			return ok && basic.Kind() == types.Bool
		})
		if !ok {
			return
		}
		op := ""
		if (expr.Op == token.EQL && !val) || (expr.Op == token.NEQ && val) {
			op = "!"
		}
		r := op + report.Render(pass, other)
		l1 := len(r)
		r = strings.TrimLeft(r, "!")
		if (l1-len(r))%2 == 1 {
			r = "!" + r
		}
		report.Report(pass, expr, fmt.Sprintf("should omit comparison to bool constant, can be simplified to %s", r),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Simplify bool comparison", edit.ReplaceWithString(expr, r))))
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
