// Copyright 2015-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/godoctor/godoctor/text"
)

// FindInComments searches the comments of the given packages' source files for
// occurrences of the given name (as a word, not a subword) and returns their
// source locations.  Position information is obtained from the given FileSet.
func FindInComments(name string, f *ast.File, scope *types.Scope, fset *token.FileSet) []*text.Extent {
	result := []*text.Extent{}
	for _, commentGroup := range f.Comments {
		for _, comment := range commentGroup.List {
			if isInScope(comment.Slash, scope) {
				slashIdx := fset.Position(comment.Slash).Offset
				whitespaceIdx := 1
				regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/\\*%s[\\PL]|[\\PL]%s$", name, name, name, name)
				re := regexp.MustCompile(regexpstring)
				matchcount := strings.Count(comment.Text, name)
				for _, idx := range re.FindAllStringIndex(comment.Text, matchcount) {
					var offset int
					if idx[0] == 0 {
						offset = slashIdx + idx[0] + whitespaceIdx + 1
					} else {
						offset = slashIdx + idx[0] + whitespaceIdx
					}
					result = append(result, &text.Extent{offset, len(name)})
				}
			}
		}

	}
	return result
}

func isInScope(pos token.Pos, scope *types.Scope) bool {
	// Object.Parent() is nil for methods and struct fields
	if scope == nil {
		return true
	}
	// Positions are undefined for Universe & package scopes (see Scope.Pos)
	if scope == types.Universe || scope.Parent() == types.Universe {
		return true
	}
	return scope.Contains(pos)
}
