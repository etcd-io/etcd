package qf1003

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
		Name:     "QF1003",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "Convert if/else-if chain to tagged switch",
		Text: `
A series of if/else-if checks comparing the same variable against
values can be replaced with a tagged switch.`,
		Before: `
if x == 1 || x == 2 {
    ...
} else if x == 3 {
    ...
} else {
    ...
}`,

		After: `
switch x {
case 1, 2:
    ...
case 3:
    ...
default:
    ...
}`,
		Since:    "2021.1",
		Severity: lint.SeverityInfo,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
nodeLoop:
	for c := range code.Cursor(pass).Preorder((*ast.IfStmt)(nil)) {
		node := c.Node()
		if _, ok := c.Parent().Node().(*ast.IfStmt); ok {
			// this if statement is part of an if-else chain
			continue
		}
		ifstmt := node.(*ast.IfStmt)

		m := map[ast.Expr][]*ast.BinaryExpr{}
		for item := ifstmt; item != nil; {
			if item.Init != nil {
				continue nodeLoop
			}
			if item.Body == nil {
				continue nodeLoop
			}

			skip := false
			ast.Inspect(item.Body, func(node ast.Node) bool {
				if branch, ok := node.(*ast.BranchStmt); ok && branch.Tok != token.GOTO {
					skip = true
					return false
				}
				return true
			})
			if skip {
				continue nodeLoop
			}

			var pairs []*ast.BinaryExpr
			if !findSwitchPairs(pass, item.Cond, &pairs) {
				continue nodeLoop
			}
			m[item.Cond] = pairs
			switch els := item.Else.(type) {
			case *ast.IfStmt:
				item = els
			case *ast.BlockStmt, nil:
				item = nil
			default:
				panic(fmt.Sprintf("unreachable: %T", els))
			}
		}

		var x ast.Expr
		for _, pair := range m {
			if len(pair) == 0 {
				continue
			}
			if x == nil {
				x = pair[0].X
			} else {
				if !astutil.Equal(x, pair[0].X) {
					continue nodeLoop
				}
			}
		}
		if x == nil {
			// shouldn't happen
			continue nodeLoop
		}

		// We require at least two 'if' to make this suggestion, to
		// avoid clutter in the editor.
		if len(m) < 2 {
			continue nodeLoop
		}

		// Note that we insert the switch statement as the first text edit instead of the last one so that gopls has an
		// easier time converting it to an LSP-conforming edit.
		//
		// Specifically:
		// > Text edits ranges must never overlap, that means no part of the original
		// > document must be manipulated by more than one edit. However, it is
		// > possible that multiple edits have the same start position: multiple
		// > inserts, or any number of inserts followed by a single remove or replace
		// > edit. If multiple inserts have the same position, the order in the array
		// > defines the order in which the inserted strings appear in the resulting
		// > text.
		//
		// See https://go.dev/issue/63930
		//
		// FIXME this edit forces the first case to begin in column 0 because we ignore indentation. try to fix that.
		edits := []analysis.TextEdit{edit.ReplaceWithString(edit.Range{ifstmt.If, ifstmt.If}, fmt.Sprintf("switch %s {\n", report.Render(pass, x)))}
		for item := ifstmt; item != nil; {
			var end token.Pos
			if item.Else != nil {
				end = item.Else.Pos()
			} else {
				// delete up to but not including the closing brace.
				end = item.Body.Rbrace
			}

			var conds []string
			for _, cond := range m[item.Cond] {
				y := cond.Y
				if p, ok := y.(*ast.ParenExpr); ok {
					y = p.X
				}
				conds = append(conds, report.Render(pass, y))
			}
			sconds := strings.Join(conds, ", ")
			edits = append(edits,
				edit.ReplaceWithString(edit.Range{item.If, item.Body.Lbrace + 1}, "case "+sconds+":"),
				edit.Delete(edit.Range{item.Body.Rbrace, end}))

			switch els := item.Else.(type) {
			case *ast.IfStmt:
				item = els
			case *ast.BlockStmt:
				edits = append(edits, edit.ReplaceWithString(edit.Range{els.Lbrace, els.Lbrace + 1}, "default:"))
				item = nil
			case nil:
				item = nil
			default:
				panic(fmt.Sprintf("unreachable: %T", els))
			}
		}
		report.Report(pass, ifstmt, fmt.Sprintf("could use tagged switch on %s", report.Render(pass, x)),
			report.Fixes(edit.Fix("Replace with tagged switch", edits...)),
			report.ShortRange())
	}
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
