package analyzer

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "goprintffuncname",
	Doc:      "Checks that printf-like functions are named with `f` at the end.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	insp.Preorder(nodeFilter, func(node ast.Node) {
		funcDecl := node.(*ast.FuncDecl)

		if res := funcDecl.Type.Results; res != nil && len(res.List) != 0 {
			return
		}

		params := funcDecl.Type.Params.List
		if len(params) < 2 { // [0] must be format (string), [1] must be args (...interface{})
			return
		}

		formatParamType, ok := params[len(params)-2].Type.(*ast.Ident)
		if !ok { // first param type isn't identificator so it can't be of type "string"
			return
		}

		if formatParamType.Name != "string" { // first param (format) type is not string
			return
		}

		formatParamNames := params[len(params)-2].Names
		if len(formatParamNames) == 0 || formatParamNames[len(formatParamNames)-1].Name != "format" {
			return
		}

		argsParamType, ok := params[len(params)-1].Type.(*ast.Ellipsis)
		if !ok {
			// args are not ellipsis (...args)
			return
		}

		if !isAny(argsParamType) {
			return
		}

		if strings.HasSuffix(funcDecl.Name.Name, "f") {
			return
		}

		pass.Reportf(node.Pos(), "printf-like formatting function '%s' should be named '%sf'",
			funcDecl.Name.Name, funcDecl.Name.Name)
	})

	return nil, nil
}

func isAny(ell *ast.Ellipsis) bool {
	switch elt := ell.Elt.(type) {
	case *ast.InterfaceType:
		if elt.Methods != nil && len(elt.Methods.List) != 0 {
			// has >= 1 method in interface, but we need an empty interface "interface{}"
			return false
		}

		return true

	case *ast.Ident:
		if elt.Name == "any" {
			return true
		}
	}

	return false
}
