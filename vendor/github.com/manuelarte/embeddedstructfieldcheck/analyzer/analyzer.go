package analyzer

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/manuelarte/embeddedstructfieldcheck/internal"
)

const (
	EmptyLineCheck   = "empty-line"
	ForbidMutexCheck = "forbid-mutex"
)

func NewAnalyzer() *analysis.Analyzer {
	var (
		emptyLine   bool
		forbidMutex bool
	)

	a := &analysis.Analyzer{
		Name: "embeddedstructfieldcheck",
		Doc: "Embedded types should be at the top of the field list of a struct, " +
			"and there must be an empty line separating embedded fields from regular fields.",
		URL: "https://github.com/manuelarte/embeddedstructfieldcheck",
		Run: func(pass *analysis.Pass) (any, error) {
			run(pass, emptyLine, forbidMutex)

			//nolint:nilnil // impossible case.
			return nil, nil
		},
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	a.Flags.BoolVar(&emptyLine, EmptyLineCheck, true,
		"Checks that there is an empty space between the embedded fields and regular fields.")
	a.Flags.BoolVar(&forbidMutex, ForbidMutexCheck, false,
		"Checks that sync.Mutex and sync.RWMutex are not used as an embedded fields.")

	return a
}

func run(pass *analysis.Pass, emptyLine, forbidMutex bool) {
	insp, found := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !found {
		return
	}

	nodeFilter := []ast.Node{
		(*ast.StructType)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		node, ok := n.(*ast.StructType)
		if !ok {
			return
		}

		internal.Analyze(pass, node, emptyLine, forbidMutex)
	})
}
