package sa4031

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/pattern"
	"honnef.co/go/tools/staticcheck/sa4022"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4031",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer, inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Checking never-nil value against nil`,
		Since:    "2022.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var allocationNilCheckQ = pattern.MustParse(`(IfStmt _ cond@(BinaryExpr lhs op@(Or "==" "!=") (Builtin "nil")) _ _)`)

func run(pass *analysis.Pass) (any, error) {
	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg

	var path []ast.Node
	fn := func(node ast.Node, stack []ast.Node) {
		m, ok := code.Match(pass, allocationNilCheckQ, node)
		if !ok {
			return
		}
		cond := m.State["cond"].(ast.Node)
		if _, ok := code.Match(pass, sa4022.CheckAddressIsNilQ, cond); ok {
			// Don't duplicate diagnostics reported by SA4022
			return
		}
		lhs := m.State["lhs"].(ast.Expr)
		path = path[:0]
		for i := len(stack) - 1; i >= 0; i-- {
			path = append(path, stack[i])
		}
		irfn := ir.EnclosingFunction(irpkg, path)
		if irfn == nil {
			// For example for functions named "_", because we don't generate IR for them.
			return
		}
		v, isAddr := irfn.ValueForExpr(lhs)
		if isAddr {
			return
		}

		seen := map[ir.Value]struct{}{}
		var values []ir.Value
		var neverNil func(v ir.Value, track bool) bool
		neverNil = func(v ir.Value, track bool) bool {
			if _, ok := seen[v]; ok {
				return true
			}
			seen[v] = struct{}{}
			switch v := v.(type) {
			case *ir.MakeClosure, *ir.Function:
				if track {
					values = append(values, v)
				}
				return true
			case *ir.MakeChan, *ir.MakeMap, *ir.MakeSlice, *ir.Alloc:
				if track {
					values = append(values, v)
				}
				return true
			case *ir.Slice:
				if track {
					values = append(values, v)
				}
				return neverNil(v.X, false)
			case *ir.FieldAddr:
				if track {
					values = append(values, v)
				}
				return neverNil(v.X, false)
			case *ir.Sigma:
				return neverNil(v.X, true)
			case *ir.Phi:
				for _, e := range v.Edges {
					if !neverNil(e, true) {
						return false
					}
				}
				return true
			default:
				return false
			}
		}

		if !neverNil(v, true) {
			return
		}

		var qualifier string
		if op := m.State["op"].(token.Token); op == token.EQL {
			qualifier = "never"
		} else {
			qualifier = "always"
		}
		fallback := fmt.Sprintf("this nil check is %s true", qualifier)

		sort.Slice(values, func(i, j int) bool { return values[i].Pos() < values[j].Pos() })

		if ident, ok := m.State["lhs"].(*ast.Ident); ok {
			if _, ok := pass.TypesInfo.ObjectOf(ident).(*types.Var); ok {
				var opts []report.Option
				if v.Parent() == irfn {
					if len(values) == 1 {
						opts = append(opts, report.Related(values[0], fmt.Sprintf("this is the value of %s", ident.Name)))
					} else {
						for _, vv := range values {
							opts = append(opts, report.Related(vv, fmt.Sprintf("this is one of the value of %s", ident.Name)))
						}
					}
				}

				switch v.(type) {
				case *ir.MakeClosure, *ir.Function:
					report.Report(pass, cond, "the checked variable contains a function and is never nil; did you mean to call it?", opts...)
				default:
					report.Report(pass, cond, fallback, opts...)
				}
			} else {
				if _, ok := v.(*ir.Function); ok {
					report.Report(pass, cond, "functions are never nil; did you mean to call it?")
				} else {
					report.Report(pass, cond, fallback)
				}
			}
		} else {
			if _, ok := v.(*ir.Function); ok {
				report.Report(pass, cond, "functions are never nil; did you mean to call it?")
			} else {
				report.Report(pass, cond, fallback)
			}
		}
	}
	code.PreorderStack(pass, fn, (*ast.IfStmt)(nil))
	return nil, nil
}
