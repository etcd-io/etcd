package sa1012

import (
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1012",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `A nil \'context.Context\' is being passed to a function, consider using \'context.TODO\' instead`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkNilContextQ = pattern.MustParse(`(CallExpr fun@(Symbol _) (Builtin "nil"):_)`)

func run(pass *analysis.Pass) (any, error) {
	todo := &ast.CallExpr{
		Fun: edit.Selector("context", "TODO"),
	}
	bg := &ast.CallExpr{
		Fun: edit.Selector("context", "Background"),
	}
	for node, m := range code.Matches(pass, checkNilContextQ) {
		call := node.(*ast.CallExpr)
		fun, ok := m.State["fun"].(*types.Func)
		if !ok {
			// it might also be a builtin
			continue
		}
		sig := fun.Type().(*types.Signature)
		if sig.Params().Len() == 0 {
			// Our CallExpr might've matched a method expression, like
			// (*T).Foo(nil) – here, nil isn't the first argument of
			// the Foo method, but the method receiver.
			continue
		}
		if !typeutil.IsTypeWithName(sig.Params().At(0).Type(), "context.Context") {
			continue
		}
		report.Report(pass, call.Args[0],
			"do not pass a nil Context, even if a function permits it; pass context.TODO if you are unsure about which Context to use", report.Fixes(
				edit.Fix("Use context.TODO", edit.ReplaceWithNode(pass.Fset, call.Args[0], todo)),
				edit.Fix("Use context.Background", edit.ReplaceWithNode(pass.Fset, call.Args[0], bg))))
	}
	return nil, nil
}
