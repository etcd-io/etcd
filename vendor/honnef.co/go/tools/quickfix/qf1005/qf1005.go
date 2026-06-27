package qf1005

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "QF1005",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `Expand call to \'math.Pow\'`,
		Text:     `Some uses of \'math.Pow\' can be simplified to basic multiplication.`,
		Before:   `math.Pow(x, 2)`,
		After:    `x * x`,
		Since:    "2021.1",
		Severity: lint.SeverityHint,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var mathPowQ = pattern.MustParse(`(CallExpr (Symbol "math.Pow") [x (IntegerLiteral n)])`)

func run(pass *analysis.Pass) (any, error) {
	for node, matcher := range code.Matches(pass, mathPowQ) {
		x := matcher.State["x"].(ast.Expr)
		if code.MayHaveSideEffects(pass, x, nil) {
			continue
		}
		n, ok := constant.Int64Val(constant.ToInt(matcher.State["n"].(types.TypeAndValue).Value))
		if !ok {
			continue
		}

		needConversion := false
		if T, ok := pass.TypesInfo.Types[x]; ok && T.Value != nil {
			info := types.Info{
				Types: map[ast.Expr]types.TypeAndValue{},
			}

			// determine if the constant expression would have type float64 if used on its own
			if err := types.CheckExpr(pass.Fset, pass.Pkg, x.Pos(), x, &info); err != nil {
				// This should not happen
				continue
			}
			if T, ok := info.Types[x].Type.(*types.Basic); ok {
				if T.Kind() != types.UntypedFloat && T.Kind() != types.Float64 {
					needConversion = true
				}
			} else {
				needConversion = true
			}
		}

		var replacement ast.Expr
		switch n {
		case 0:
			replacement = &ast.BasicLit{
				Kind:  token.FLOAT,
				Value: "1.0",
			}
		case 1:
			replacement = x
		case 2, 3:
			r := &ast.BinaryExpr{
				X:  x,
				Op: token.MUL,
				Y:  x,
			}
			for i := 3; i <= int(n); i++ {
				r = &ast.BinaryExpr{
					X:  r,
					Op: token.MUL,
					Y:  x,
				}
			}

			rc, ok := astutil.CopyExpr(r)
			if !ok {
				continue
			}
			replacement = astutil.SimplifyParentheses(rc)
		default:
			continue
		}
		if needConversion && n != 0 {
			replacement = &ast.CallExpr{
				Fun:  &ast.Ident{Name: "float64"},
				Args: []ast.Expr{replacement},
			}
		}
		report.Report(pass, node, "could expand call to math.Pow",
			report.Fixes(edit.Fix("Expand call to math.Pow", edit.ReplaceWithNode(pass.Fset, node, replacement))))
	}
	return nil, nil
}
