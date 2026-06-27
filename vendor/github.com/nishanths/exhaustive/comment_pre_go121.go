//go:build !go1.21

package exhaustive

import (
	"go/ast"
	"regexp"
)

// For definition of generated file see:
// http://golang.org/s/generatedcode

var generatedCodeRe = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

func isGeneratedFile(file *ast.File) bool {
	for _, c := range file.Comments {
		for _, cc := range c.List {
			if cc.Pos() > file.Package {
				break
			}
			if generatedCodeRe.MatchString(cc.Text) {
				return true
			}
		}
	}
	return false
}
