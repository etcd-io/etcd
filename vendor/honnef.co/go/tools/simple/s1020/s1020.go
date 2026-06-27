package s1020

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1020",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title:   `Omit redundant nil check in type assertion`,
		Before:  `if _, ok := i.(T); ok && i != nil {}`,
		After:   `if _, ok := i.(T); ok {}`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkAssertNotNilFn1Q = pattern.MustParse(`
		(IfStmt
			(AssignStmt [(Ident "_") ok@(Object _)] _ [(TypeAssertExpr assert@(Object _) _)])
			(Or
				(BinaryExpr ok "&&" (BinaryExpr assert "!=" (Builtin "nil")))
				(BinaryExpr (BinaryExpr assert "!=" (Builtin "nil")) "&&" ok))
			_
			_)`)
	checkAssertNotNilFn2Q = pattern.MustParse(`
		(IfStmt
			nil
			(BinaryExpr lhs@(Object _) "!=" (Builtin "nil"))
			[
				ifstmt@(IfStmt
					(AssignStmt [(Ident "_") ok@(Object _)] _ [(TypeAssertExpr lhs _)])
					ok
					_
					nil)
			]
			nil)`)
)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkAssertNotNilFn1Q) {
		assert := m.State["assert"].(types.Object)
		assign := m.State["ok"].(types.Object)
		report.Report(pass, node, fmt.Sprintf("when %s is true, %s can't be nil", assign.Name(), assert.Name()),
			report.ShortRange(),
			report.FilterGenerated())
	}
	for _, m := range code.Matches(pass, checkAssertNotNilFn2Q) {
		ifstmt := m.State["ifstmt"].(*ast.IfStmt)
		lhs := m.State["lhs"].(types.Object)
		assignIdent := m.State["ok"].(types.Object)
		report.Report(pass, ifstmt, fmt.Sprintf("when %s is true, %s can't be nil", assignIdent.Name(), lhs.Name()),
			report.ShortRange(),
			report.FilterGenerated())
	}
	return nil, nil
}
