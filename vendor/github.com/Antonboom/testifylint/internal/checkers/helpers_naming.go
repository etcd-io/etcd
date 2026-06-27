package checkers

import (
	"go/ast"
	"regexp"
)

func isStructVarNamedAfterPattern(pattern *regexp.Regexp, e ast.Expr) bool {
	s, ok := e.(*ast.SelectorExpr)
	return ok && isIdentNamedAfterPattern(pattern, s.X)
}

func isStructFieldNamedAfterPattern(pattern *regexp.Regexp, e ast.Expr) bool {
	s, ok := e.(*ast.SelectorExpr)
	return ok && isIdentNamedAfterPattern(pattern, s.Sel)
}

func isIdentNamedAfterPattern(pattern *regexp.Regexp, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && pattern.MatchString(id.Name)
}

func isIdentWithName(name string, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}
