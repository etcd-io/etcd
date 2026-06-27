package sa4014

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4014",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `An if/else if chain has repeated conditions and no side-effects; if the condition didn't match the first time, it won't match the second time, either`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	seen := map[ast.Node]bool{}

	var collectConds func(ifstmt *ast.IfStmt, conds []ast.Expr) ([]ast.Expr, bool)
	collectConds = func(ifstmt *ast.IfStmt, conds []ast.Expr) ([]ast.Expr, bool) {
		seen[ifstmt] = true
		// Bail if any if-statement has an Init statement or side effects in its condition
		if ifstmt.Init != nil {
			return nil, false
		}
		if code.MayHaveSideEffects(pass, ifstmt.Cond, nil) {
			return nil, false
		}

		conds = append(conds, ifstmt.Cond)
		if elsestmt, ok := ifstmt.Else.(*ast.IfStmt); ok {
			return collectConds(elsestmt, conds)
		}
		return conds, true
	}
	fn := func(node ast.Node) {
		ifstmt := node.(*ast.IfStmt)
		if seen[ifstmt] {
			// this if-statement is part of an if/else-if chain that we've already processed
			return
		}
		if ifstmt.Else == nil {
			// there can be at most one condition
			return
		}
		conds, ok := collectConds(ifstmt, nil)
		if !ok {
			return
		}
		if len(conds) < 2 {
			return
		}
		counts := map[string]int{}
		for _, cond := range conds {
			s := report.Render(pass, cond)
			counts[s]++
			if counts[s] == 2 {
				report.Report(pass, cond, "this condition occurs multiple times in this if/else if chain")
			}
		}
	}
	code.Preorder(pass, fn, (*ast.IfStmt)(nil))
	return nil, nil
}
