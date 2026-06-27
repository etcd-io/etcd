package analyzer

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/manuelarte/funcorder/internal"
)

const (
	ConstructorCheckName  = "constructor"
	StructMethodCheckName = "struct-method"
	AlphabeticalCheckName = "alphabetical"
	FunctionCheckName     = "function"
)

func NewAnalyzer() *analysis.Analyzer {
	f := funcorder{}

	a := &analysis.Analyzer{
		Name:     "funcorder",
		Doc:      "checks the order of functions, methods, and constructors",
		URL:      "https://github.com/manuelarte/funcorder",
		Run:      f.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	a.Flags.BoolVar(&f.constructorCheck, ConstructorCheckName, true,
		"Checks that constructors are placed after the structure declaration.")
	a.Flags.BoolVar(&f.structMethodCheck, StructMethodCheckName, true,
		"Checks if the exported methods of a structure are placed before the unexported ones.")
	a.Flags.BoolVar(&f.alphabeticalCheck, AlphabeticalCheckName, false,
		"Checks if the constructors and/or structure methods are sorted alphabetically.")
	a.Flags.BoolVar(&f.functionCheck, FunctionCheckName, false,
		"Checks that exported functions are placed before unexported functions.")

	return a
}

type funcorder struct {
	constructorCheck  bool
	structMethodCheck bool
	alphabeticalCheck bool
	functionCheck     bool
}

func (f *funcorder) run(pass *analysis.Pass) (any, error) {
	insp, found := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !found {
		//nolint:nilnil // impossible case.
		return nil, nil
	}

	var enabledCheckers internal.Feature
	if f.constructorCheck {
		enabledCheckers.Enable(internal.ConstructorCheck)
	}

	if f.structMethodCheck {
		enabledCheckers.Enable(internal.StructMethodCheck)
	}

	if f.alphabeticalCheck {
		enabledCheckers.Enable(internal.AlphabeticalCheck)
	}

	if f.functionCheck {
		enabledCheckers.Enable(internal.FunctionCheck)
	}

	fp := internal.NewFileProcessor(enabledCheckers)

	nodeFilter := []ast.Node{
		(*ast.File)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.TypeSpec)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.File:
			fp.Analyze(pass)
			fp.ResetStructs()

		case *ast.FuncDecl:
			fp.AddFuncDecl(node)

		case *ast.TypeSpec:
			fp.AddTypeSpec(node)
		}
	})

	fp.Analyze(pass)

	//nolint:nilnil //any, error
	return nil, nil
}
