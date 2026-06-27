package inamedparam

import (
	"flag"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	analyzerName = "inamedparam"

	flagSkipSingleParam = "skip-single-param"
)

var Analyzer = &analysis.Analyzer{
	Name:  analyzerName,
	Doc:   "reports interfaces with unnamed method parameters",
	Run:   run,
	Flags: flags(),
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

func flags() flag.FlagSet {
	flags := flag.NewFlagSet(analyzerName, flag.ExitOnError)

	flags.Bool(flagSkipSingleParam, false, "skip interface methods with a single unnamed parameter")

	return *flags
}

func run(pass *analysis.Pass) (any, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	types := []ast.Node{
		&ast.InterfaceType{},
	}

	var skipSingleParam bool
	if f := pass.Analyzer.Flags.Lookup(flagSkipSingleParam); f != nil {
		skipSingleParam, _ = f.Value.(flag.Getter).Get().(bool)
	}

	inspect.Preorder(types, func(n ast.Node) {
		interfaceType, ok := n.(*ast.InterfaceType)
		if !ok || interfaceType == nil || interfaceType.Methods == nil {
			return
		}

		for _, method := range interfaceType.Methods.List {
			interfaceFunc, ok := method.Type.(*ast.FuncType)
			if !ok || interfaceFunc == nil || interfaceFunc.Params == nil {
				continue
			}

			// Improvement: add test case to reproduce this. Help wanted.
			if len(method.Names) == 0 {
				continue
			}

			methodName := method.Names[0].Name

			if skipSingleParam && len(interfaceFunc.Params.List) == 1 {
				continue
			}

			for _, param := range interfaceFunc.Params.List {
				if param.Names == nil {
					var builtParamType string

					switch paramType := param.Type.(type) {
					case *ast.SelectorExpr:
						if ident := paramType.X.(*ast.Ident); ident != nil {
							builtParamType += ident.Name + "."
						}

						builtParamType += paramType.Sel.Name
					case *ast.Ident:
						builtParamType = paramType.Name
					}

					if builtParamType != "" {
						pass.Reportf(param.Pos(), "interface method %v must have named param for type %v", methodName, builtParamType)
					} else {
						pass.Reportf(param.Pos(), "interface method %v must have all named params", methodName)
					}
				}
			}
		}
	})

	return nil, nil
}
