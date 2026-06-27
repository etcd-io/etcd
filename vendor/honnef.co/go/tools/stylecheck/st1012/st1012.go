package st1012

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name: "ST1012",
		Run:  run,
	},
	Doc: &lint.RawDocumentation{
		Title: `Poorly chosen name for error variable`,
		Text: `Error variables that are part of an API should be called \'errFoo\' or
\'ErrFoo\'.`,
		Since:   "2019.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				spec := spec.(*ast.ValueSpec)
				if len(spec.Names) != len(spec.Values) {
					continue
				}

				for i, name := range spec.Names {
					val := spec.Values[i]
					if !code.IsCallToAny(pass, val, "errors.New", "fmt.Errorf") {
						continue
					}

					if pass.Pkg.Path() == "net/http" && strings.HasPrefix(name.Name, "http2err") {
						// special case for internal variable names of
						// bundled HTTP 2 code in net/http
						continue
					}
					prefix := "err"
					if name.IsExported() {
						prefix = "Err"
					}
					if !strings.HasPrefix(name.Name, prefix) {
						report.Report(pass, name, fmt.Sprintf("error var %s should have name of the form %sFoo", name.Name, prefix))
					}
				}
			}
		}
	}
	return nil, nil
}
