package testableexamples

import (
	"go/ast"
	"go/doc"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// NewAnalyzer returns Analyzer that checks if examples are testable.
func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "testableexamples",
		Doc:  "linter checks if examples are testable (have an expected output)",
		Run: func(pass *analysis.Pass) (any, error) {
			testFiles := make([]*ast.File, 0, len(pass.Files))
			for _, file := range pass.Files {
				fileName := pass.Fset.File(file.Pos()).Name()
				if strings.HasSuffix(fileName, "_test.go") {
					testFiles = append(testFiles, file)
				}
			}

			for _, example := range doc.Examples(testFiles...) {
				if example.Output == "" && !example.EmptyOutput {
					pass.Reportf(example.Code.Pos(), "missing output for example, go test can't validate it")
				}
			}

			return nil, nil
		},
	}
}
