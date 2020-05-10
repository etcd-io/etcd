// testdata package holds example functions.
package testdata

import (
	// ast gives a representation of the Go abstract syntax tree.
	"go/ast"
	// types wraps ast and holds the Go type checker algorithm.
	"go/types"
)

// Foo200 compares an expression to a type.
func Foo200(x ast.Expr, t types.Type) bool { return types.ExprString(x) == Bar200(t) }

// Bar200 converts a type to a string.
func Bar200(t types.Type) string { return t.String() }
