package sa6006

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA6006",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `Using io.WriteString to write \'[]byte\'`,
		Text: `Using io.WriteString to write a slice of bytes, as in

    io.WriteString(w, string(b))

is both unnecessary and inefficient. Converting from \'[]byte\' to \'string\'
has to allocate and copy the data, and we could simply use \'w.Write(b)\'
instead.`,

		Since: "2024.1",
	},
})

var Analyzer = SCAnalyzer.Analyzer

var ioWriteStringConversion = pattern.MustParse(`(CallExpr (Symbol "io.WriteString") [_ (CallExpr (Builtin "string") [arg])])`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, ioWriteStringConversion) {
		if !code.IsOfStringConvertibleByteSlice(pass, m.State["arg"].(ast.Expr)) {
			continue
		}
		report.Report(pass, node, "use io.Writer.Write instead of converting from []byte to string to use io.WriteString")
	}
	return nil, nil
}
