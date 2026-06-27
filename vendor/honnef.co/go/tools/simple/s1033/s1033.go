package s1033

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1033",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title:   `Unnecessary guard around call to \"delete\"`,
		Text:    `Calling \'delete\' on a nil map is a no-op.`,
		Since:   "2019.2",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkGuardedDeleteQ = pattern.MustParse(`
	(IfStmt
		(AssignStmt
			[(Ident "_") ok@(Ident _)]
			":="
			(IndexExpr m key))
		ok
		[call@(CallExpr (Builtin "delete") [m key])]
		nil)`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkGuardedDeleteQ) {
		report.Report(pass, node, "unnecessary guard around call to delete",
			report.ShortRange(),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Remove guard", edit.ReplaceWithNode(pass.Fset, node, m.State["call"].(ast.Node)))))
	}
	return nil, nil
}
