package s1016

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"go/version"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1016",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Use a type conversion instead of manually copying struct fields`,
		Text: `Two struct types with identical fields can be converted between each
other. In older versions of Go, the fields had to have identical
struct tags. Since Go 1.8, however, struct tags are ignored during
conversions. It is thus not necessary to manually copy every field
individually.`,
		Before: `
var x T1
y := T2{
    Field1: x.Field1,
    Field2: x.Field2,
}`,
		After: `
var x T1
y := T2(x)`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	// TODO(dh): support conversions between type parameters
	fn := func(c inspector.Cursor) {
		node := c.Node()
		if unary, ok := c.Parent().Node().(*ast.UnaryExpr); ok && unary.Op == token.AND {
			// Do not suggest type conversion between pointers
			return
		}

		lit := node.(*ast.CompositeLit)
		var typ1 types.Type
		var named1 *types.Named
		switch typ := pass.TypesInfo.TypeOf(lit.Type).(type) {
		case *types.Named:
			typ1 = typ
			named1 = typ
		case *types.Alias:
			ua := types.Unalias(typ)
			if n, ok := ua.(*types.Named); ok {
				typ1 = typ
				named1 = n
			}
		}
		if typ1 == nil {
			return
		}
		s1, ok := typ1.Underlying().(*types.Struct)
		if !ok {
			return
		}

		var typ2 types.Type
		var named2 *types.Named
		var ident *ast.Ident
		getSelType := func(expr ast.Expr) (types.Type, *ast.Ident, bool) {
			sel, ok := expr.(*ast.SelectorExpr)
			if !ok {
				return nil, nil, false
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return nil, nil, false
			}
			typ := pass.TypesInfo.TypeOf(sel.X)
			return typ, ident, typ != nil
		}
		if len(lit.Elts) == 0 {
			return
		}
		if s1.NumFields() != len(lit.Elts) {
			return
		}
		for i, elt := range lit.Elts {
			var t types.Type
			var id *ast.Ident
			var ok bool
			switch elt := elt.(type) {
			case *ast.SelectorExpr:
				t, id, ok = getSelType(elt)
				if !ok {
					return
				}
				if i >= s1.NumFields() || s1.Field(i).Name() != elt.Sel.Name {
					return
				}
			case *ast.KeyValueExpr:
				var sel *ast.SelectorExpr
				sel, ok = elt.Value.(*ast.SelectorExpr)
				if !ok {
					return
				}

				if elt.Key.(*ast.Ident).Name != sel.Sel.Name {
					return
				}
				t, id, ok = getSelType(elt.Value)
			}
			if !ok {
				return
			}
			// All fields must be initialized from the same object
			if ident != nil && pass.TypesInfo.ObjectOf(ident) != pass.TypesInfo.ObjectOf(id) {
				return
			}
			switch t := t.(type) {
			case *types.Named:
				typ2 = t
				named2 = t
			case *types.Alias:
				if n, ok := types.Unalias(t).(*types.Named); ok {
					typ2 = t
					named2 = n
				}
			}
			if typ2 == nil {
				return
			}
			ident = id
		}

		if typ2 == nil {
			return
		}

		if named1.Obj().Pkg() != named2.Obj().Pkg() {
			// Do not suggest type conversions between different
			// packages. Types in different packages might only match
			// by coincidence. Furthermore, if the dependency ever
			// adds more fields to its type, it could break the code
			// that relies on the type conversion to work.
			return
		}

		s2, ok := typ2.Underlying().(*types.Struct)
		if !ok {
			return
		}
		if typ1 == typ2 {
			return
		}
		if version.Compare(code.LanguageVersion(pass, node), "go1.8") >= 0 {
			if !types.IdenticalIgnoreTags(s1, s2) {
				return
			}
		} else {
			if !types.Identical(s1, s2) {
				return
			}
		}

		r := &ast.CallExpr{
			Fun:  lit.Type,
			Args: []ast.Expr{ident},
		}
		report.Report(pass, node,
			fmt.Sprintf("should convert %s (type %s) to %s instead of using struct literal", ident.Name, types.TypeString(typ2, types.RelativeTo(pass.Pkg)), types.TypeString(typ1, types.RelativeTo(pass.Pkg))),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Use type conversion", edit.ReplaceWithNode(pass.Fset, node, r))))
	}
	for c := range code.Cursor(pass).Preorder((*ast.CompositeLit)(nil)) {
		fn(c)
	}
	return nil, nil
}
