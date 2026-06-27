package s1005

import (
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1005",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Drop unnecessary use of the blank identifier`,
		Text:  `In many cases, assigning to the blank identifier is unnecessary.`,
		Before: `
for _ = range s {}
x, _ = someMap[key]
_ = <-ch`,
		After: `
for range s{}
x = someMap[key]
<-ch`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkUnnecessaryBlankQ1 = pattern.MustParse(`
		(AssignStmt
			[_ (Ident "_")]
			_
			(Or
				(IndexExpr _ _)
				(UnaryExpr "<-" _))) `)
	checkUnnecessaryBlankQ2 = pattern.MustParse(`
		(AssignStmt
			(Ident "_") _ recv@(UnaryExpr "<-" _))`)
)

func run(pass *analysis.Pass) (any, error) {
	fn1 := func(node ast.Node) {
		if _, ok := code.Match(pass, checkUnnecessaryBlankQ1, node); ok {
			r := *node.(*ast.AssignStmt)
			r.Lhs = r.Lhs[0:1]
			report.Report(pass, node, "unnecessary assignment to the blank identifier",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Remove assignment to blank identifier", edit.ReplaceWithNode(pass.Fset, node, &r))))
		} else if m, ok := code.Match(pass, checkUnnecessaryBlankQ2, node); ok {
			report.Report(pass, node, "unnecessary assignment to the blank identifier",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Simplify channel receive operation", edit.ReplaceWithNode(pass.Fset, node, m.State["recv"].(ast.Node)))))
		}
	}

	fn3 := func(node ast.Node) {
		rs := node.(*ast.RangeStmt)

		if _, ok := pass.TypesInfo.TypeOf(rs.X).Underlying().(*types.Signature); ok {
			// iteration variables are not optional with rangefunc
			return
		}

		// for _
		if rs.Value == nil && astutil.IsBlank(rs.Key) {
			report.Report(pass, rs.Key, "unnecessary assignment to the blank identifier",
				report.FilterGenerated(),
				report.MinimumLanguageVersion("go1.4"),
				report.Fixes(edit.Fix("Remove assignment to blank identifier", edit.Delete(edit.Range{rs.Key.Pos(), rs.TokPos + 1}))))
		}

		// for _, _
		if astutil.IsBlank(rs.Key) && astutil.IsBlank(rs.Value) {
			// FIXME we should mark both key and value
			report.Report(pass, rs.Key, "unnecessary assignment to the blank identifier",
				report.FilterGenerated(),
				report.MinimumLanguageVersion("go1.4"),
				report.Fixes(edit.Fix("Remove assignment to blank identifier", edit.Delete(edit.Range{rs.Key.Pos(), rs.TokPos + 1}))))
		}

		// for x, _
		if !astutil.IsBlank(rs.Key) && astutil.IsBlank(rs.Value) {
			report.Report(pass, rs.Value, "unnecessary assignment to the blank identifier",
				report.FilterGenerated(),
				report.MinimumLanguageVersion("go1.4"),
				report.Fixes(edit.Fix("Remove assignment to blank identifier", edit.Delete(edit.Range{rs.Key.End(), rs.Value.End()}))))
		}
	}

	code.Preorder(pass, fn1, (*ast.AssignStmt)(nil))
	code.Preorder(pass, fn3, (*ast.RangeStmt)(nil))
	return nil, nil
}
