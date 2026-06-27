package sa4016

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4016",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Certain bitwise operations, such as \'x ^ 0\', do not do anything useful`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny, // MergeIfAny if we only flag literals, not named constants
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		binop := node.(*ast.BinaryExpr)
		if !typeutil.All(pass.TypesInfo.TypeOf(binop), func(term *types.Term) bool {
			b, ok := term.Type().Underlying().(*types.Basic)
			if !ok {
				return false
			}
			return (b.Info() & types.IsInteger) != 0
		}) {
			return
		}
		switch binop.Op {
		case token.AND, token.OR, token.XOR:
		default:
			// we do not flag shifts because too often, x<<0 is part
			// of a pattern, x<<0, x<<8, x<<16, ...
			return
		}
		if y, ok := binop.Y.(*ast.Ident); ok {
			obj, ok := pass.TypesInfo.ObjectOf(y).(*types.Const)
			if !ok {
				return
			}
			if obj.Pkg() != pass.Pkg {
				// identifier was dot-imported
				return
			}
			if v, _ := constant.Int64Val(obj.Val()); v != 0 {
				return
			}
			path, _ := astutil.PathEnclosingInterval(code.File(pass, obj), obj.Pos(), obj.Pos())
			if len(path) < 2 {
				return
			}
			spec, ok := path[1].(*ast.ValueSpec)
			if !ok {
				return
			}
			if len(spec.Names) != 1 || len(spec.Values) != 1 {
				// TODO(dh): we could support this
				return
			}
			ident, ok := spec.Values[0].(*ast.Ident)
			if !ok {
				return
			}
			if !isIota(pass.TypesInfo.ObjectOf(ident)) {
				return
			}
			switch binop.Op {
			case token.AND:
				report.Report(pass, node,
					fmt.Sprintf("%s always equals 0; %s is defined as iota and has value 0, maybe %s is meant to be 1 << iota?", report.Render(pass, binop), report.Render(pass, binop.Y), report.Render(pass, binop.Y)))
			case token.OR, token.XOR:
				report.Report(pass, node,
					fmt.Sprintf("%s always equals %s; %s is defined as iota and has value 0, maybe %s is meant to be 1 << iota?", report.Render(pass, binop), report.Render(pass, binop.X), report.Render(pass, binop.Y), report.Render(pass, binop.Y)))
			}
		} else if code.IsIntegerLiteral(pass, binop.Y, constant.MakeInt64(0)) {
			switch binop.Op {
			case token.AND:
				report.Report(pass, node, fmt.Sprintf("%s always equals 0", report.Render(pass, binop)))
			case token.OR, token.XOR:
				report.Report(pass, node, fmt.Sprintf("%s always equals %s", report.Render(pass, binop), report.Render(pass, binop.X)))
			}
		}
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}

func isIota(obj types.Object) bool {
	if obj.Name() != "iota" {
		return false
	}
	c, ok := obj.(*types.Const)
	if !ok {
		return false
	}
	return c.Pkg() == nil
}
