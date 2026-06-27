package comment

import (
	"go/ast"
	"strings"
)

type Directive string

const (
	prefix                     = `//exhaustruct:`
	DirectiveIgnore  Directive = prefix + `ignore`
	DirectiveEnforce Directive = prefix + `enforce`
)

// HasDirective parses a directive from a given list of comments.
// If no directive is found, the second return value is `false`.
func HasDirective(comments []*ast.CommentGroup, expected Directive) bool {
	for _, cg := range comments {
		for _, commentLine := range cg.List {
			if strings.HasPrefix(commentLine.Text, string(expected)) {
				return true
			}
		}
	}

	return false
}
