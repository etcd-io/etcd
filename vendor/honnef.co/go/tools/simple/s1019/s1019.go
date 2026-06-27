package s1019

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1019",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Simplify \"make\" call by omitting redundant arguments`,
		Text: `The \"make\" function has default values for the length and capacity
arguments. For channels, the length defaults to zero, and for slices,
the capacity defaults to the length.`,
		Since: "2017.1",
		// MergeIfAll because the type might be different under different build tags.
		// You shouldn't write code like that…
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkMakeLenCapQ1 = pattern.MustParse(`(CallExpr (Builtin "make") [typ size@(IntegerLiteral "0")])`)
	checkMakeLenCapQ2 = pattern.MustParse(`(CallExpr (Builtin "make") [typ size size])`)
)

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		if pass.Pkg.Path() == "runtime_test" && filepath.Base(pass.Fset.Position(node.Pos()).Filename) == "map_test.go" {
			// special case of runtime tests testing map creation
			return
		}
		if m, ok := code.Match(pass, checkMakeLenCapQ1, node); ok {
			T := m.State["typ"].(ast.Expr)
			size := m.State["size"].(ast.Node)

			if _, ok := typeutil.CoreType(pass.TypesInfo.TypeOf(T)).Underlying().(*types.Chan); ok {
				report.Report(pass, size, fmt.Sprintf("should use make(%s) instead", report.Render(pass, T)), report.FilterGenerated())
			}

		} else if m, ok := code.Match(pass, checkMakeLenCapQ2, node); ok {
			// TODO(dh): don't consider sizes identical if they're
			// dynamic. for example: make(T, <-ch, <-ch).
			T := m.State["typ"].(ast.Expr)
			size := m.State["size"].(ast.Node)
			report.Report(pass, size,
				fmt.Sprintf("should use make(%s, %s) instead", report.Render(pass, T), report.Render(pass, size)),
				report.FilterGenerated())
		}
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
