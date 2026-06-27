package s1001

import (
	"fmt"
	"go/ast"
	"go/token"
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
		Name:     "S1001",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Replace for loop with call to copy`,
		Text: `
Use \'copy()\' for copying elements from one slice to another. For
arrays of identical size, you can use simple assignment.`,
		Before: `
for i, x := range src {
    dst[i] = x
}`,
		After: `copy(dst, src)`,
		Since: "2017.1",
		// MergeIfAll because the types of src and dst might be different under different build tags.
		// You shouldn't write code like that…
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkLoopCopyQ = pattern.MustParse(`
		(Or
			(RangeStmt
				key@(Ident _) value@(Ident _) ":=" src
				[(AssignStmt (IndexExpr dst key) "=" value)])
			(RangeStmt
				key@(Ident _) nil ":=" src
				[(AssignStmt (IndexExpr dst key) "=" (IndexExpr src key))])
			(ForStmt
				(AssignStmt key@(Ident _) ":=" (IntegerLiteral "0"))
				(BinaryExpr key "<" (CallExpr (Symbol "len") [src]))
				(IncDecStmt key "++")
				[(AssignStmt (IndexExpr dst key) "=" (IndexExpr src key))]))`)
)

func run(pass *analysis.Pass) (any, error) {
	// TODO revisit once range doesn't require a structural type

	isInvariant := func(k, v types.Object, node ast.Expr) bool {
		if code.MayHaveSideEffects(pass, node, nil) {
			return false
		}
		invariant := true
		ast.Inspect(node, func(node ast.Node) bool {
			if node, ok := node.(*ast.Ident); ok {
				obj := pass.TypesInfo.ObjectOf(node)
				if obj == k || obj == v {
					// don't allow loop bodies like 'a[i][i] = v'
					invariant = false
					return false
				}
			}
			return true
		})
		return invariant
	}

	var elType func(T types.Type) (el types.Type, isArray bool, isArrayPointer bool, ok bool)
	elType = func(T types.Type) (el types.Type, isArray bool, isArrayPointer bool, ok bool) {
		switch typ := T.Underlying().(type) {
		case *types.Slice:
			return typ.Elem(), false, false, true
		case *types.Array:
			return typ.Elem(), true, false, true
		case *types.Pointer:
			el, isArray, _, ok = elType(typ.Elem())
			return el, isArray, true, ok
		default:
			return nil, false, false, false
		}
	}

	for node, m := range code.Matches(pass, checkLoopCopyQ) {
		src := m.State["src"].(ast.Expr)
		dst := m.State["dst"].(ast.Expr)

		k := pass.TypesInfo.ObjectOf(m.State["key"].(*ast.Ident))
		var v types.Object
		if value, ok := m.State["value"]; ok {
			v = pass.TypesInfo.ObjectOf(value.(*ast.Ident))
		}
		if !isInvariant(k, v, dst) {
			continue
		}
		if !isInvariant(k, v, src) {
			// For example: 'for i := range foo()'
			continue
		}

		Tsrc := pass.TypesInfo.TypeOf(src)
		Tdst := pass.TypesInfo.TypeOf(dst)
		TsrcElem, TsrcArray, TsrcPointer, ok := elType(Tsrc)
		if !ok {
			continue
		}
		if TsrcPointer {
			Tsrc = Tsrc.Underlying().(*types.Pointer).Elem()
		}
		TdstElem, TdstArray, TdstPointer, ok := elType(Tdst)
		if !ok {
			continue
		}
		if TdstPointer {
			Tdst = Tdst.Underlying().(*types.Pointer).Elem()
		}

		if !types.Identical(TsrcElem, TdstElem) {
			continue
		}

		if TsrcArray && TdstArray && types.Identical(Tsrc, Tdst) {
			if TsrcPointer {
				src = &ast.StarExpr{
					X: src,
				}
			}
			if TdstPointer {
				dst = &ast.StarExpr{
					X: dst,
				}
			}
			r := &ast.AssignStmt{
				Lhs: []ast.Expr{dst},
				Rhs: []ast.Expr{src},
				Tok: token.ASSIGN,
			}

			report.Report(pass, node, "should copy arrays using assignment instead of using a loop",
				report.FilterGenerated(),
				report.ShortRange(),
				report.Fixes(edit.Fix("Replace loop with assignment", edit.ReplaceWithNode(pass.Fset, node, r))))
		} else {
			tv, err := types.Eval(pass.Fset, pass.Pkg, node.Pos(), "copy")
			if err == nil && tv.IsBuiltin() {
				to := "to"
				from := "from"
				src := m.State["src"].(ast.Expr)
				if TsrcArray {
					from = "from[:]"
					src = &ast.SliceExpr{
						X: src,
					}
				}
				dst := m.State["dst"].(ast.Expr)
				if TdstArray {
					to = "to[:]"
					dst = &ast.SliceExpr{
						X: dst,
					}
				}

				r := &ast.CallExpr{
					Fun:  &ast.Ident{Name: "copy"},
					Args: []ast.Expr{dst, src},
				}
				opts := []report.Option{
					report.ShortRange(),
					report.FilterGenerated(),
					report.Fixes(edit.Fix("Replace loop with call to copy()", edit.ReplaceWithNode(pass.Fset, node, r))),
				}
				report.Report(pass, node, fmt.Sprintf("should use copy(%s, %s) instead of a loop", to, from), opts...)
			}
		}
	}
	return nil, nil
}
