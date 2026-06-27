package st1019

import (
	"fmt"
	"go/ast"

	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1019",
		Run:      run,
		Requires: []*analysis.Analyzer{generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Importing the same package multiple times`,
		Text: `Go allows importing the same package multiple times, as long as
different import aliases are being used. That is, the following
bit of code is valid:

    import (
        "fmt"
        fumpt "fmt"
        format "fmt"
    )

However, this is very rarely done on purpose. Usually, it is a
sign of code that got refactored, accidentally adding duplicate
import statements. It is also a rarely known feature, which may
contribute to confusion.

Do note that sometimes, this feature may be used
intentionally (see for example
https://github.com/golang/go/commit/3409ce39bfd7584523b7a8c150a310cea92d879d)
– if you want to allow this pattern in your code base, you're
advised to disable this check.

It is acceptable to import the same package twice if one of the imports
uses the blank identifier. This is allowed in order to increase
resilience against erroneous changes when using the same package for its
side effects as well as its exported API.`,
		Since:   "2020.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		// Collect all imports by their import path
		imports := make(map[string][]*ast.ImportSpec, len(f.Imports))
		for _, imp := range f.Imports {
			if imp.Name != nil && imp.Name.Name == "_" {
				// Allow blank imports to coexist with one normal import.
				//
				// We don't have to count the number of blank imports,
				// goimports removes duplicates.
				continue
			}
			imports[imp.Path.Value] = append(imports[imp.Path.Value], imp)
		}

		for path, value := range imports {
			if path[1:len(path)-1] == "unsafe" {
				// Don't flag unsafe. Cgo generated code imports
				// unsafe as _cgo_unsafe, in addition to the user's import.
				continue
			}
			// If there's more than one import per path, we flag that
			if len(value) > 1 {
				s := fmt.Sprintf("package %s is being imported more than once", path)
				opts := []report.Option{report.FilterGenerated()}
				for _, imp := range value[1:] {
					opts = append(opts, report.Related(imp, fmt.Sprintf("other import of %s", path)))
				}
				report.Report(pass, value[0], s, opts...)
			}
		}
	}
	return nil, nil
}
