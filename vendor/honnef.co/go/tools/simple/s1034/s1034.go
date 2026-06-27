package s1034

import (
	"fmt"
	"go/ast"
	"go/types"

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
		Name:     "S1034",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title:   `Use result of type assertion to simplify cases`,
		Since:   "2019.2",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkSimplifyTypeSwitchQ = pattern.MustParse(`
		(TypeSwitchStmt
			nil
			expr@(TypeAssertExpr ident@(Ident _) _)
			body)`)
	checkSimplifyTypeSwitchR = pattern.MustParse(`(AssignStmt ident ":=" expr)`)
)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkSimplifyTypeSwitchQ) {
		stmt := node.(*ast.TypeSwitchStmt)
		expr := m.State["expr"].(ast.Node)
		ident := m.State["ident"].(*ast.Ident)

		x := pass.TypesInfo.ObjectOf(ident)
		var allOffenders []*ast.TypeAssertExpr
		canSuggestFix := true
		for _, clause := range stmt.Body.List {
			clause := clause.(*ast.CaseClause)
			if len(clause.List) != 1 {
				continue
			}
			hasUnrelatedAssertion := false
			var offenders []*ast.TypeAssertExpr
			ast.Inspect(clause, func(node ast.Node) bool {
				assert2, ok := node.(*ast.TypeAssertExpr)
				if !ok {
					return true
				}
				ident, ok := assert2.X.(*ast.Ident)
				if !ok {
					hasUnrelatedAssertion = true
					return false
				}
				if pass.TypesInfo.ObjectOf(ident) != x {
					hasUnrelatedAssertion = true
					return false
				}

				if !types.Identical(pass.TypesInfo.TypeOf(clause.List[0]), pass.TypesInfo.TypeOf(assert2.Type)) {
					hasUnrelatedAssertion = true
					return false
				}
				offenders = append(offenders, assert2)
				return true
			})
			if !hasUnrelatedAssertion {
				// don't flag cases that have other type assertions
				// unrelated to the one in the case clause. often
				// times, this is done for symmetry, when two
				// different values have to be asserted to the same
				// type.
				allOffenders = append(allOffenders, offenders...)
			}
			canSuggestFix = canSuggestFix && !hasUnrelatedAssertion
		}
		if len(allOffenders) != 0 {
			var opts []report.Option
			for _, offender := range allOffenders {
				opts = append(opts, report.Related(offender, "could eliminate this type assertion"))
			}
			opts = append(opts, report.FilterGenerated())

			msg := fmt.Sprintf("assigning the result of this type assertion to a variable (switch %s := %s.(type)) could eliminate type assertions in switch cases",
				report.Render(pass, ident), report.Render(pass, ident))
			if canSuggestFix {
				var edits []analysis.TextEdit
				edits = append(edits, edit.ReplaceWithPattern(pass.Fset, expr, checkSimplifyTypeSwitchR, m.State))
				for _, offender := range allOffenders {
					edits = append(edits, edit.ReplaceWithNode(pass.Fset, offender, offender.X))
				}
				opts = append(opts, report.Fixes(edit.Fix("Simplify type switch", edits...)))
				report.Report(pass, expr, msg, opts...)
			} else {
				report.Report(pass, expr, msg, opts...)
			}
		}
	}
	return nil, nil
}
