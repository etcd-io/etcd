package qf1002

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "QF1002",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "Convert untagged switch to tagged switch",
		Text: `
An untagged switch that compares a single variable against a series of
values can be replaced with a tagged switch.`,
		Before: `
switch {
case x == 1 || x == 2, x == 3:
    ...
case x == 4:
    ...
default:
    ...
}`,

		After: `
switch x {
case 1, 2, 3:
    ...
case 4:
    ...
default:
    ...
}`,
		Since:    "2021.1",
		Severity: lint.SeverityHint,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		swtch := node.(*ast.SwitchStmt)
		if swtch.Tag != nil || len(swtch.Body.List) == 0 {
			return
		}

		pairs := make([][]*ast.BinaryExpr, len(swtch.Body.List))
		for i, stmt := range swtch.Body.List {
			stmt := stmt.(*ast.CaseClause)
			for _, cond := range stmt.List {
				if !findSwitchPairs(pass, cond, &pairs[i]) {
					return
				}
			}
		}

		var x ast.Expr
		for _, pair := range pairs {
			if len(pair) == 0 {
				continue
			}
			if x == nil {
				x = pair[0].X
			} else {
				if !astutil.Equal(x, pair[0].X) {
					return
				}
			}
		}
		if x == nil {
			// the switch only has a default case
			if len(pairs) > 1 {
				panic("found more than one case clause with no pairs")
			}
			return
		}

		edits := make([]analysis.TextEdit, 0, len(swtch.Body.List)+1)
		for i, stmt := range swtch.Body.List {
			stmt := stmt.(*ast.CaseClause)
			if stmt.List == nil {
				continue
			}

			var values []string
			for _, binexpr := range pairs[i] {
				y := binexpr.Y
				if p, ok := y.(*ast.ParenExpr); ok {
					y = p.X
				}
				values = append(values, report.Render(pass, y))
			}

			edits = append(edits, edit.ReplaceWithString(edit.Range{stmt.List[0].Pos(), stmt.Colon}, strings.Join(values, ", ")))
		}
		pos := swtch.Body.Lbrace
		edits = append(edits, edit.ReplaceWithString(edit.Range{pos, pos}, " "+report.Render(pass, x)))
		report.Report(pass, swtch, fmt.Sprintf("could use tagged switch on %s", report.Render(pass, x)),
			report.Fixes(edit.Fix("Replace with tagged switch", edits...)),
			report.ShortRange())
	}

	code.Preorder(pass, fn, (*ast.SwitchStmt)(nil))
	return nil, nil
}

func findSwitchPairs(pass *analysis.Pass, expr ast.Expr, pairs *[]*ast.BinaryExpr) bool {
	binexpr, ok := astutil.Unparen(expr).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	switch binexpr.Op {
	case token.EQL:
		if code.MayHaveSideEffects(pass, binexpr.X, nil) || code.MayHaveSideEffects(pass, binexpr.Y, nil) {
			return false
		}
		// syntactic identity should suffice. we do not allow side
		// effects in the case clauses, so there should be no way for
		// values to change.
		if len(*pairs) > 0 && !astutil.Equal(binexpr.X, (*pairs)[0].X) {
			return false
		}
		*pairs = append(*pairs, binexpr)
		return true
	case token.LOR:
		return findSwitchPairs(pass, binexpr.X, pairs) && findSwitchPairs(pass, binexpr.Y, pairs)
	default:
		return false
	}
}
