package ginkgohandler

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	config "github.com/nunnatsa/ginkgolinter/config"
)

// nameHandler is used when importing ginkgo without name; i.e.
// import "github.com/onsi/ginkgo"
//
// or with a custom name; e.g.
// import customname "github.com/onsi/ginkgo"
type nameHandler string

func (h nameHandler) HandleGinkgoSpecs(expr ast.Expr, config config.Config, pass *analysis.Pass) bool {
	return handleGinkgoSpecs(expr, config, pass, h)
}

func (h nameHandler) getFocusContainerName(exp *ast.CallExpr) (bool, *ast.Ident) {
	if sel, ok := exp.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == string(h) {
			return isFocusContainer(sel.Sel.Name), sel.Sel
		}
	}
	return false, nil
}

func (h nameHandler) isWrapContainer(exp *ast.CallExpr) bool {
	if sel, ok := exp.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == string(h) {
			return isWrapContainer(sel.Sel.Name)
		}
	}

	return false
}

func (h nameHandler) isFocusSpec(exp ast.Expr) bool {
	if selExp, ok := exp.(*ast.SelectorExpr); ok {
		if x, ok := selExp.X.(*ast.Ident); ok && x.Name == string(h) {
			return selExp.Sel.Name == focusSpec
		}
	}

	return false
}
