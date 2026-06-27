package qf1001

import (
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "QF1001",
		Run:      CheckDeMorgan,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    "Apply De Morgan's law",
		Since:    "2021.1",
		Severity: lint.SeverityHint,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var demorganQ = pattern.MustParse(`(UnaryExpr "!" expr@(BinaryExpr _ _ _))`)

func CheckDeMorgan(pass *analysis.Pass) (any, error) {
	// TODO(dh): support going in the other direction, e.g. turning `!a && !b && !c` into `!(a || b || c)`

	// hasFloats reports whether any subexpression is of type float.
	hasFloats := func(expr ast.Expr) bool {
		found := false
		ast.Inspect(expr, func(node ast.Node) bool {
			if expr, ok := node.(ast.Expr); ok {
				if typ := pass.TypesInfo.TypeOf(expr); typ != nil {
					if basic, ok := typ.Underlying().(*types.Basic); ok {
						if (basic.Info() & types.IsFloat) != 0 {
							found = true
							return false
						}
					}
				}
			}
			return true
		})
		return found
	}

	for c := range code.Cursor(pass).Preorder((*ast.UnaryExpr)(nil)) {
		node := c.Node()
		matcher, ok := code.Match(pass, demorganQ, node)
		if !ok {
			continue
		}

		expr := matcher.State["expr"].(ast.Expr)

		// be extremely conservative when it comes to floats
		if hasFloats(expr) {
			continue
		}

		n := astutil.NegateDeMorgan(expr, false)
		nr := astutil.NegateDeMorgan(expr, true)
		nc, ok := astutil.CopyExpr(n)
		if !ok {
			continue
		}
		ns := astutil.SimplifyParentheses(nc)
		nrc, ok := astutil.CopyExpr(nr)
		if !ok {
			continue
		}
		nrs := astutil.SimplifyParentheses(nrc)

		var bn, bnr, bns, bnrs string
		switch c.Parent().Node().(type) {
		case *ast.BinaryExpr, *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt:
			// Always add parentheses for if, for and switch. If
			// they're unnecessary, go/printer will strip them when
			// the whole file gets formatted.

			bn = report.Render(pass, &ast.ParenExpr{X: n})
			bnr = report.Render(pass, &ast.ParenExpr{X: nr})
			bns = report.Render(pass, &ast.ParenExpr{X: ns})
			bnrs = report.Render(pass, &ast.ParenExpr{X: nrs})

		default:
			// TODO are there other types where we don't want to strip parentheses?
			bn = report.Render(pass, n)
			bnr = report.Render(pass, nr)
			bns = report.Render(pass, ns)
			bnrs = report.Render(pass, nrs)
		}

		// Note: we cannot compare the ASTs directly, because
		// simplifyParentheses might have rebalanced trees without
		// affecting the rendered form.
		var fixes []analysis.SuggestedFix
		fixes = append(fixes, edit.Fix("Apply De Morgan's law", edit.ReplaceWithString(node, bn)))
		if bn != bns {
			fixes = append(fixes, edit.Fix("Apply De Morgan's law & simplify", edit.ReplaceWithString(node, bns)))
		}
		if bn != bnr {
			fixes = append(fixes, edit.Fix("Apply De Morgan's law recursively", edit.ReplaceWithString(node, bnr)))
			if bnr != bnrs {
				fixes = append(fixes, edit.Fix("Apply De Morgan's law recursively & simplify", edit.ReplaceWithString(node, bnrs)))
			}
		}

		report.Report(pass, node, "could apply De Morgan's law", report.Fixes(fixes...))
	}

	return nil, nil
}
