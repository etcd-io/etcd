//go:build (go1.16 && !go1.18) || (go1.17 && !go1.18)
// +build go1.16,!go1.18 go1.17,!go1.18

package varnamelen

import "go/ast"

// isTypeParam returns true if field is a type parameter of any of the given funcs.
func isTypeParam(_ *ast.Field, _ []*ast.FuncDecl, _ []*ast.FuncLit) bool {
	return false
}
