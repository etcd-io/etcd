// Package typeparams provides utilities for working with Go ASTs with support
// for type parameters when built with Go 1.18 and higher.
package typeparams

import (
	"go/ast"
)

// ReceiverType returns the named type of the method receiver, sans "*" and type
// parameters, or "invalid-type" if fn.Recv is ill formed.
func ReceiverType(fn *ast.FuncDecl) string {
	e := fn.Recv.List[0].Type
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	e = unpackIndexExpr(e)
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return "invalid-type"
}

func unpackIndexExpr(e ast.Expr) ast.Expr {
	switch e := e.(type) {
	case *ast.IndexExpr:
		return e.X
	case *ast.IndexListExpr:
		return e.X
	}
	return e
}
