package s1000

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1000",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Use plain channel send or receive instead of single-case select`,
		Text: `Select statements with a single case can be replaced with a simple
send or receive.`,
		Before: `
select {
case x := <-ch:
    fmt.Println(x)
}`,
		After: `
x := <-ch
fmt.Println(x)
`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkSingleCaseSelectQ1 = pattern.MustParse(`
		(ForStmt
			nil nil nil
			select@(SelectStmt
				(CommClause
					(Or
						(UnaryExpr "<-" _)
						(AssignStmt _ _ (UnaryExpr "<-" _)))
					_)))`)
	checkSingleCaseSelectQ2 = pattern.MustParse(`(SelectStmt (CommClause _ _))`)
)

func run(pass *analysis.Pass) (any, error) {
	seen := map[ast.Node]struct{}{}
	fn := func(node ast.Node) {
		if m, ok := code.Match(pass, checkSingleCaseSelectQ1, node); ok {
			seen[m.State["select"].(ast.Node)] = struct{}{}
			report.Report(pass, node, "should use for range instead of for { select {} }", report.FilterGenerated())
		} else if _, ok := code.Match(pass, checkSingleCaseSelectQ2, node); ok {
			if _, ok := seen[node]; !ok {
				report.Report(pass, node, "should use a simple channel send/receive instead of select with a single case",
					report.ShortRange(),
					report.FilterGenerated())
			}
		}
	}
	code.Preorder(pass, fn, (*ast.ForStmt)(nil), (*ast.SelectStmt)(nil))
	return nil, nil
}
