package dogsled

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.DogsledSettings) *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: "dogsled",
			Doc:  "Checks assignments with too many blank identifiers (e.g. x, _, _, _, := f())",
			Run: func(pass *analysis.Pass) (any, error) {
				return run(pass, settings.MaxBlankIdentifiers)
			},
			Requires: []*analysis.Analyzer{inspect.Analyzer},
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}

func run(pass *analysis.Pass, maxBlanks int) (any, error) {
	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	for node := range insp.PreorderSeq((*ast.FuncDecl)(nil)) {
		funcDecl, ok := node.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if funcDecl.Body == nil {
			continue
		}

		for _, expr := range funcDecl.Body.List {
			assgnStmt, ok := expr.(*ast.AssignStmt)
			if !ok {
				continue
			}

			numBlank := 0
			for _, left := range assgnStmt.Lhs {
				ident, ok := left.(*ast.Ident)
				if !ok {
					continue
				}
				if ident.Name == "_" {
					numBlank++
				}
			}

			if numBlank > maxBlanks {
				pass.Reportf(assgnStmt.Pos(), "declaration has %v blank identifiers", numBlank)
			}
		}
	}

	return nil, nil
}
