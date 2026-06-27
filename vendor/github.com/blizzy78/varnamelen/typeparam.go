//go:build go1.18
// +build go1.18

package varnamelen

import "go/ast"

// isTypeParam returns true if field is a type parameter of any of the given funcs.
func isTypeParam(field *ast.Field, funcs []*ast.FuncDecl, funcLits []*ast.FuncLit) bool { //nolint:gocognit // it's not that complicated
	for _, f := range funcs {
		if f.Type.TypeParams == nil {
			continue
		}

		for _, p := range f.Type.TypeParams.List {
			if p == field {
				return true
			}
		}
	}

	for _, f := range funcLits {
		if f.Type.TypeParams == nil {
			continue
		}

		for _, p := range f.Type.TypeParams.List {
			if p == field {
				return true
			}
		}
	}

	return false
}
