package s1011

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/facts/purity"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1011",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer, purity.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Use a single \'append\' to concatenate two slices`,
		Before: `
for _, e := range y {
    x = append(x, e)
}

for i := range y {
    x = append(x, y[i])
}

for i := range y {
    v := y[i]
    x = append(x, v)
}`,

		After: `
x = append(x, y...)
x = append(x, y...)
x = append(x, y...)`,
		Since: "2017.1",
		// MergeIfAll because y might not be a slice under all build tags.
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkLoopAppendQ = pattern.MustParse(`
(Or
	(RangeStmt
		(Ident "_")
		val@(Object _)
		_
		x
		[(AssignStmt [lhs] "=" [(CallExpr (Builtin "append") [lhs val])])])
	(RangeStmt
		idx@(Object _)
		nil
		_
		x
		[(AssignStmt [lhs] "=" [(CallExpr (Builtin "append") [lhs (IndexExpr x idx)])])])
	(RangeStmt
		idx@(Object _)
		nil
		_
		x
		[(AssignStmt val@(Object _) ":=" (IndexExpr x idx))
		(AssignStmt [lhs] "=" [(CallExpr (Builtin "append") [lhs val])])]))`)

func run(pass *analysis.Pass) (any, error) {
	pure := pass.ResultOf[purity.Analyzer].(purity.Result)

	for node, m := range code.Matches(pass, checkLoopAppendQ) {
		if val, ok := m.State["val"].(types.Object); ok && code.RefersTo(pass, m.State["lhs"].(ast.Expr), val) {
			continue
		}

		if m.State["idx"] != nil && code.MayHaveSideEffects(pass, m.State["x"].(ast.Expr), pure) {
			// When using an index-based loop, x gets evaluated repeatedly and thus should be pure.
			// This doesn't matter for value-based loops, because x only gets evaluated once.
			continue
		}

		if idx, ok := m.State["idx"].(types.Object); ok && code.RefersTo(pass, m.State["lhs"].(ast.Expr), idx) {
			// The lhs mustn't refer to the index loop variable.
			continue
		}

		if code.MayHaveSideEffects(pass, m.State["lhs"].(ast.Expr), pure) {
			// The lhs may be dynamic and return different values on each iteration. For example:
			//
			// 	func bar() map[int][]int { /* return one of several maps */ }
			//
			// 	func foo(x []int, y [][]int) {
			// 		for i := range x {
			// 			bar()[0] = append(bar()[0], x[i])
			// 		}
			// 	}
			//
			// The dynamic nature of the lhs might also affect the value of the index.
			continue
		}

		src := pass.TypesInfo.TypeOf(m.State["x"].(ast.Expr))
		dst := pass.TypesInfo.TypeOf(m.State["lhs"].(ast.Expr))
		if !types.Identical(src, dst) {
			continue
		}

		r := &ast.AssignStmt{
			Lhs: []ast.Expr{m.State["lhs"].(ast.Expr)},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.Ident{Name: "append"},
					Args: []ast.Expr{
						m.State["lhs"].(ast.Expr),
						m.State["x"].(ast.Expr),
					},
					Ellipsis: 1,
				},
			},
		}

		report.Report(pass, node, fmt.Sprintf("should replace loop with %s", report.Render(pass, r)),
			report.ShortRange(),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Replace loop with call to append", edit.ReplaceWithNode(pass.Fset, node, r))))
	}
	return nil, nil
}
