package s1003

import (
	"fmt"
	"go/ast"
	"go/token"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1003",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:   `Replace call to \'strings.Index\' with \'strings.Contains\'`,
		Before:  `if strings.Index(x, y) != -1 {}`,
		After:   `if strings.Contains(x, y) {}`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	// map of value to token to bool value
	allowed := map[int64]map[token.Token]bool{
		-1: {token.GTR: true, token.NEQ: true, token.EQL: false},
		0:  {token.GEQ: true, token.LSS: false},
	}
	fn := func(node ast.Node) {
		expr := node.(*ast.BinaryExpr)
		switch expr.Op {
		case token.GEQ, token.GTR, token.NEQ, token.LSS, token.EQL:
		default:
			return
		}

		value, ok := code.ExprToInt(pass, expr.Y)
		if !ok {
			return
		}

		allowedOps, ok := allowed[value]
		if !ok {
			return
		}
		b, ok := allowedOps[expr.Op]
		if !ok {
			return
		}

		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			return
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return
		}
		funIdent := sel.Sel
		if pkgIdent.Name != "strings" && pkgIdent.Name != "bytes" {
			return
		}

		var r ast.Expr
		switch funIdent.Name {
		case "IndexRune":
			r = &ast.SelectorExpr{
				X:   pkgIdent,
				Sel: &ast.Ident{Name: "ContainsRune"},
			}
		case "IndexAny":
			r = &ast.SelectorExpr{
				X:   pkgIdent,
				Sel: &ast.Ident{Name: "ContainsAny"},
			}
		case "Index":
			r = &ast.SelectorExpr{
				X:   pkgIdent,
				Sel: &ast.Ident{Name: "Contains"},
			}
		default:
			return
		}

		r = &ast.CallExpr{
			Fun:  r,
			Args: call.Args,
		}
		if !b {
			r = &ast.UnaryExpr{
				Op: token.NOT,
				X:  r,
			}
		}

		report.Report(pass, node, fmt.Sprintf("should use %s instead", report.Render(pass, r)),
			report.FilterGenerated(),
			report.Fixes(edit.Fix(fmt.Sprintf("Simplify use of %s", report.Render(pass, call.Fun)), edit.ReplaceWithNode(pass.Fset, node, r))))
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
