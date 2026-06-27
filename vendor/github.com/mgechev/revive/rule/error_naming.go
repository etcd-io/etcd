package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// ErrorNamingRule lints naming of error variables.
type ErrorNamingRule struct{}

// Apply applies the rule to given file.
func (*ErrorNamingRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	fileAst := file.AST
	walker := lintErrors{
		file:    file,
		fileAst: fileAst,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	ast.Walk(walker, fileAst)

	return failures
}

// Name returns the rule name.
func (*ErrorNamingRule) Name() string {
	return "error-naming"
}

type lintErrors struct {
	file      *lint.File
	fileAst   *ast.File
	onFailure func(lint.Failure)
}

func (w lintErrors) Visit(_ ast.Node) ast.Visitor {
	for _, decl := range w.fileAst.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			spec := spec.(*ast.ValueSpec)
			if len(spec.Names) != 1 || len(spec.Values) != 1 {
				continue
			}
			ce, ok := spec.Values[0].(*ast.CallExpr)
			if !ok {
				continue
			}
			if !astutils.IsPkgDotName(ce.Fun, "errors", "New") && !astutils.IsPkgDotName(ce.Fun, "fmt", "Errorf") {
				continue
			}

			id := spec.Names[0]
			if id.Name == "_" {
				// avoid false positive for blank identifier

				// The fact that the error variable is not used
				// is out of the scope of the rule

				// This pattern that can be found in benchmarks and examples
				// should be allowed.
				continue
			}

			prefix := "err"
			if id.IsExported() {
				prefix = "Err"
			}
			if !strings.HasPrefix(id.Name, prefix) {
				w.onFailure(lint.Failure{
					Node:       id,
					Confidence: 0.9,
					Category:   lint.FailureCategoryNaming,
					Failure:    fmt.Sprintf("error var %s should have name of the form %sFoo", id.Name, prefix),
				})
			}
		}
	}
	return nil
}
