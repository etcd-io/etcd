package ginkgohandler

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/config"
)

// dotHandler is used when importing ginkgo with dot; i.e.
// import . "github.com/onsi/ginkgo"
type dotHandler struct{}

func (h dotHandler) HandleGinkgoSpecs(expr ast.Expr, config config.Config, pass *analysis.Pass) bool {
	return handleGinkgoSpecs(expr, config, pass, h)
}

func (h dotHandler) getFocusContainerName(exp *ast.CallExpr) (bool, *ast.Ident) {
	if fun, ok := exp.Fun.(*ast.Ident); ok {
		return isFocusContainer(fun.Name), fun
	}
	return false, nil
}

func (h dotHandler) isWrapContainer(exp *ast.CallExpr) bool {
	if fun, ok := exp.Fun.(*ast.Ident); ok {
		return isWrapContainer(fun.Name)
	}
	return false
}

func (h dotHandler) isFocusSpec(exp ast.Expr) bool {
	id, ok := exp.(*ast.Ident)
	return ok && id.Name == focusSpec
}
