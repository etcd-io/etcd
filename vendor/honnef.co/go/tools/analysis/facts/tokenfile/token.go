package tokenfile

import (
	"go/ast"
	"go/token"
	"reflect"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "tokenfileanalyzer",
	Doc:  "creates a mapping of *token.File to *ast.File",
	Run: func(pass *analysis.Pass) (any, error) {
		m := map[*token.File]*ast.File{}
		for _, af := range pass.Files {
			tf := pass.Fset.File(af.Pos())
			m[tf] = af
		}
		return m, nil
	},
	RunDespiteErrors: true,
	ResultType:       reflect.TypeFor[map[*token.File]*ast.File](),
}
