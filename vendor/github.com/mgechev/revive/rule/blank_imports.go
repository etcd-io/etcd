package rule

import (
	"go/ast"
	"strings"

	"github.com/mgechev/revive/lint"
)

// BlankImportsRule lints blank imports.
type BlankImportsRule struct{}

// Name returns the rule name.
func (*BlankImportsRule) Name() string {
	return "blank-imports"
}

// Apply applies the rule to given file.
func (r *BlankImportsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if file.Pkg.IsMain() || file.IsTest() {
		return nil
	}

	const (
		message         = "a blank import should be only in a main or test package, or have a comment justifying it"
		embedImportPath = `"embed"`
	)

	var failures []lint.Failure

	// The first element of each contiguous group of blank imports should have
	// an explanatory comment of some kind.
	for i, imp := range file.AST.Imports {
		pos := file.ToPosition(imp.Pos())

		if !isBlank(imp.Name) {
			continue // Ignore non-blank imports.
		}

		isNotFirstElement := i > 0
		if isNotFirstElement {
			prev := file.AST.Imports[i-1]
			prevPos := file.ToPosition(prev.Pos())

			isSubsequentBlancInAGroup := prevPos.Line+1 == pos.Line && prev.Path.Value != embedImportPath && isBlank(prev.Name)
			if isSubsequentBlancInAGroup {
				continue
			}
		}

		if imp.Path.Value == embedImportPath && r.fileHasValidEmbedComment(file.AST) {
			continue
		}

		// This is the first blank import of a group.
		if imp.Doc == nil && imp.Comment == nil {
			failures = append(failures, lint.Failure{Failure: message, Category: lint.FailureCategoryImports, Node: imp, Confidence: 1})
		}
	}

	return failures
}

func (*BlankImportsRule) fileHasValidEmbedComment(fileAst *ast.File) bool {
	for _, commentGroup := range fileAst.Comments {
		for _, comment := range commentGroup.List {
			if strings.HasPrefix(comment.Text, "//go:embed ") {
				return true
			}
		}
	}

	return false
}

// isBlank returns whether id is the blank identifier "_".
// If id == nil, the answer is false.
func isBlank(id *ast.Ident) bool { return id != nil && id.Name == "_" }
