//go:build go1.18
// +build go1.18

package astcopy

import (
	"go/ast"
)

// FuncType returns x deep copy.
// Copy of nil argument is nil.
func FuncType(x *ast.FuncType) *ast.FuncType {
	if x == nil {
		return nil
	}
	cp := *x
	cp.Params = FieldList(x.Params)
	cp.Results = FieldList(x.Results)
	cp.TypeParams = FieldList(x.TypeParams)
	return &cp
}

// TypeSpec returns x deep copy.
// Copy of nil argument is nil.
func TypeSpec(x *ast.TypeSpec) *ast.TypeSpec {
	if x == nil {
		return nil
	}
	cp := *x
	cp.Name = Ident(x.Name)
	cp.Type = copyExpr(x.Type)
	cp.Doc = CommentGroup(x.Doc)
	cp.Comment = CommentGroup(x.Comment)
	cp.TypeParams = FieldList(x.TypeParams)
	return &cp
}
