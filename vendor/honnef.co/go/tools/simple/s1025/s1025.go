package s1025

import (
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"
	"honnef.co/go/tools/pattern"

	"golang.org/x/exp/typeparams"
	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name: "S1025",
		Run:  run,
		Requires: append([]*analysis.Analyzer{
			buildir.Analyzer,
			generated.Analyzer,
		}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Don't use \'fmt.Sprintf("%s", x)\' unnecessarily`,
		Text: `In many instances, there are easier and more efficient ways of getting
a value's string representation. Whenever a value's underlying type is
a string already, or the type has a String method, they should be used
directly.

Given the following shared definitions

    type T1 string
    type T2 int

    func (T2) String() string { return "Hello, world" }

    var x string
    var y T1
    var z T2

we can simplify

    fmt.Sprintf("%s", x)
    fmt.Sprintf("%s", y)
    fmt.Sprintf("%s", z)

to

    x
    string(y)
    z.String()
`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkRedundantSprintfQ = pattern.MustParse(`(CallExpr (Symbol "fmt.Sprintf") [format arg])`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkRedundantSprintfQ) {
		format := m.State["format"].(ast.Expr)
		arg := m.State["arg"].(ast.Expr)
		// TODO(dh): should we really support named constants here?
		// shouldn't we only look for string literals? to avoid false
		// positives via build tags?
		if s, ok := code.ExprToString(pass, format); !ok || s != "%s" {
			continue
		}
		typ := pass.TypesInfo.TypeOf(arg)
		if typeparams.IsTypeParam(typ) {
			continue
		}
		irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg

		if typeutil.IsTypeWithName(typ, "reflect.Value") {
			// printing with %s produces output different from using
			// the String method
			continue
		}

		if isFormatter(typ, &irpkg.Prog.MethodSets) {
			// the type may choose to handle %s in arbitrary ways
			continue
		}

		if types.Implements(typ, knowledge.Interfaces["fmt.Stringer"]) {
			replacement := &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   arg,
					Sel: &ast.Ident{Name: "String"},
				},
			}
			report.Report(pass, node, "should use String() instead of fmt.Sprintf",
				report.Fixes(edit.Fix("Replace with call to String method", edit.ReplaceWithNode(pass.Fset, node, replacement))))
		} else if types.Unalias(typ) == types.Universe.Lookup("string").Type() {
			report.Report(pass, node, "the argument is already a string, there's no need to use fmt.Sprintf",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Remove unnecessary call to fmt.Sprintf", edit.ReplaceWithNode(pass.Fset, node, arg))))
		} else if typ.Underlying() == types.Universe.Lookup("string").Type() {
			replacement := &ast.CallExpr{
				Fun:  &ast.Ident{Name: "string"},
				Args: []ast.Expr{arg},
			}
			report.Report(pass, node, "the argument's underlying type is a string, should use a simple conversion instead of fmt.Sprintf",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Replace with conversion to string", edit.ReplaceWithNode(pass.Fset, node, replacement))))
		} else if code.IsOfStringConvertibleByteSlice(pass, arg) {
			replacement := &ast.CallExpr{
				Fun:  &ast.Ident{Name: "string"},
				Args: []ast.Expr{arg},
			}
			report.Report(pass, node, "the argument's underlying type is a slice of bytes, should use a simple conversion instead of fmt.Sprintf",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Replace with conversion to string", edit.ReplaceWithNode(pass.Fset, node, replacement))))
		}

	}
	return nil, nil
}

func isFormatter(T types.Type, msCache *typeutil.MethodSetCache) bool {
	// TODO(dh): this function also exists in staticcheck/lint.go – deduplicate.

	ms := msCache.MethodSet(T)
	sel := ms.Lookup(nil, "Format")
	if sel == nil {
		return false
	}
	fn, ok := sel.Obj().(*types.Func)
	if !ok {
		// should be unreachable
		return false
	}
	sig := fn.Type().(*types.Signature)
	if sig.Params().Len() != 2 {
		return false
	}
	// TODO(dh): check the types of the arguments for more
	// precision
	if sig.Results().Len() != 0 {
		return false
	}
	return true
}
