package exhaustive

import (
	"go/ast"
	"go/token"
	"strings"
)

const (
	ignoreComment                     = "//exhaustive:ignore"
	enforceComment                    = "//exhaustive:enforce"
	ignoreDefaultCaseRequiredComment  = "//exhaustive:ignore-default-case-required"
	enforceDefaultCaseRequiredComment = "//exhaustive:enforce-default-case-required"
)

func hasCommentPrefix(comments []*ast.CommentGroup, comment string) bool {
	for _, c := range comments {
		for _, cc := range c.List {
			if strings.HasPrefix(cc.Text, comment) {
				return true
			}
		}
	}
	return false
}

func fileCommentMap(fset *token.FileSet, file *ast.File) ast.CommentMap {
	return ast.NewCommentMap(fset, file, file.Comments)
}
