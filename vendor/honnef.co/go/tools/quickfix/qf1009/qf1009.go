package qf1009

import (
	"go/ast"
	"go/token"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "QF1009",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Use \'time.Time.Equal\' instead of \'==\' operator`,
		Since:    "2021.1",
		Severity: lint.SeverityInfo,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var timeEqualR = pattern.MustParse(`(CallExpr (SelectorExpr lhs (Ident "Equal")) rhs)`)

func run(pass *analysis.Pass) (any, error) {
	// FIXME(dh): create proper suggested fix for renamed import

	fn := func(node ast.Node) {
		expr := node.(*ast.BinaryExpr)
		if expr.Op != token.EQL {
			return
		}
		if !code.IsOfTypeWithName(pass, expr.X, "time.Time") || !code.IsOfTypeWithName(pass, expr.Y, "time.Time") {
			return
		}
		report.Report(pass, node, "probably want to use time.Time.Equal instead",
			report.Fixes(edit.Fix("Use time.Time.Equal method",
				edit.ReplaceWithPattern(pass.Fset, node, timeEqualR, pattern.State{"lhs": expr.X, "rhs": expr.Y}))))
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
