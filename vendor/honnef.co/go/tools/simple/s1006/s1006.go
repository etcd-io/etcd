package s1006

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1006",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:   `Use \"for { ... }\" for infinite loops`,
		Text:    `For infinite loops, using \'for { ... }\' is the most idiomatic choice.`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		loop := node.(*ast.ForStmt)
		if loop.Init != nil || loop.Post != nil {
			return
		}
		if !code.IsBoolConst(pass, loop.Cond) || !code.BoolConst(pass, loop.Cond) {
			return
		}
		report.Report(pass, loop, "should use for {} instead of for true {}",
			report.ShortRange(),
			report.FilterGenerated())
	}
	code.Preorder(pass, fn, (*ast.ForStmt)(nil))
	return nil, nil
}
