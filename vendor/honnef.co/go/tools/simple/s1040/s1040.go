package s1040

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1040",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "Type assertion to current type",
		Text: `The type assertion \'x.(SomeInterface)\', when \'x\' already has type
\'SomeInterface\', can only fail if \'x\' is nil. Usually, this is
left-over code from when \'x\' had a different type and you can safely
delete the type assertion. If you want to check that \'x\' is not nil,
consider being explicit and using an actual \'if x == nil\' comparison
instead of relying on the type assertion panicking.`,
		Since: "2021.1",
		// MergeIfAll because x might have different types under different build tags.
		// You shouldn't write code like that…
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		expr := node.(*ast.TypeAssertExpr)
		if expr.Type == nil {
			// skip type switches
			//
			// TODO(dh): we could flag type switches, too, when a case
			// statement has the same type as expr.X – however,
			// depending on the location of that case, it might behave
			// identically to a default branch. we need to think
			// carefully about the instances we want to flag. We also
			// have to take nil interface values into consideration.
			//
			// It might make more sense to extend SA4020 to handle
			// this.
			return
		}
		t1 := pass.TypesInfo.TypeOf(expr.Type)
		t2 := pass.TypesInfo.TypeOf(expr.X)
		if types.IsInterface(t1) && types.Identical(t1, t2) {
			report.Report(pass, expr,
				fmt.Sprintf("type assertion to the same type: %s already has type %s", report.Render(pass, expr.X), report.Render(pass, expr.Type)),
				report.FilterGenerated())
		}
	}

	// TODO(dh): add suggested fixes. we need different fixes depending on the context:
	// - assignment with 1 or 2 lhs
	// - assignment to blank identifiers (as the first, second or both lhs)
	// - initializers in if statements, with the same variations as above

	code.Preorder(pass, fn, (*ast.TypeAssertExpr)(nil))
	return nil, nil
}
