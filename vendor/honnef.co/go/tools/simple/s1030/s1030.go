package s1030

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1030",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Use \'bytes.Buffer.String\' or \'bytes.Buffer.Bytes\'`,
		Text: `\'bytes.Buffer\' has both a \'String\' and a \'Bytes\' method. It is almost never
necessary to use \'string(buf.Bytes())\' or \'[]byte(buf.String())\' – simply
use the other method.

The only exception to this are map lookups. Due to a compiler optimization,
\'m[string(buf.Bytes())]\' is more efficient than \'m[buf.String()]\'.
`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkBytesBufferConversionsQ  = pattern.MustParse(`(CallExpr _ [(CallExpr sel@(SelectorExpr recv _) [])])`)
	checkBytesBufferConversionsRs = pattern.MustParse(`(CallExpr (SelectorExpr recv (Ident "String")) [])`)
	checkBytesBufferConversionsRb = pattern.MustParse(`(CallExpr (SelectorExpr recv (Ident "Bytes")) [])`)
)

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Path() == "bytes" || pass.Pkg.Path() == "bytes_test" {
		// The bytes package can use itself however it wants
		return nil, nil
	}
	fn := func(c inspector.Cursor) {
		node := c.Node()
		m, ok := code.Match(pass, checkBytesBufferConversionsQ, node)
		if !ok {
			return
		}
		call := node.(*ast.CallExpr)
		sel := m.State["sel"].(*ast.SelectorExpr)

		typ := pass.TypesInfo.TypeOf(call.Fun)
		if types.Unalias(typ) == types.Universe.Lookup("string").Type() && code.IsCallTo(pass, call.Args[0], "(*bytes.Buffer).Bytes") {
			if _, ok := c.Parent().Node().(*ast.IndexExpr); ok {
				// Don't flag m[string(buf.Bytes())] – thanks to a
				// compiler optimization, this is actually faster than
				// m[buf.String()]
				return
			}

			report.Report(pass, call, fmt.Sprintf("should use %v.String() instead of %v", report.Render(pass, sel.X), report.Render(pass, call)),
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Simplify conversion", edit.ReplaceWithPattern(pass.Fset, node, checkBytesBufferConversionsRs, m.State))))
		} else if typ, ok := types.Unalias(typ).(*types.Slice); ok &&
			types.Unalias(typ.Elem()) == types.Universe.Lookup("byte").Type() &&
			code.IsCallTo(pass, call.Args[0], "(*bytes.Buffer).String") {
			report.Report(pass, call, fmt.Sprintf("should use %v.Bytes() instead of %v", report.Render(pass, sel.X), report.Render(pass, call)),
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Simplify conversion", edit.ReplaceWithPattern(pass.Fset, node, checkBytesBufferConversionsRb, m.State))))
		}

	}
	for c := range code.Cursor(pass).Preorder((*ast.CallExpr)(nil)) {
		fn(c)
	}
	return nil, nil
}
