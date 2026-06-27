package analyzer

import (
	"flag"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const InterfaceMaxMethodsFlag = "max"

const defaultMaxMethods = 10

// New returns new interfacebloat analyzer.
func New() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "interfacebloat",
		Doc:      "A linter that checks the number of methods inside an interface.",
		Run:      run,
		Flags:    flags(),
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func flags() flag.FlagSet {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Int(InterfaceMaxMethodsFlag, 10, "maximum number of methods")
	return *flags
}

func run(pass *analysis.Pass) (interface{}, error) {
	maxMethods, ok := pass.Analyzer.Flags.Lookup(InterfaceMaxMethodsFlag).Value.(flag.Getter).Get().(int)
	if !ok {
		maxMethods = defaultMaxMethods
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	filter := []ast.Node{
		(*ast.InterfaceType)(nil),
	}

	insp.Preorder(filter, func(node ast.Node) {
		i, ok := node.(*ast.InterfaceType)
		if !ok {
			return
		}

		if len(i.Methods.List) > maxMethods {
			pass.Reportf(node.Pos(), `the interface has more than %d methods: %d`, maxMethods, len(i.Methods.List))
		}
	})

	return nil, nil
}
