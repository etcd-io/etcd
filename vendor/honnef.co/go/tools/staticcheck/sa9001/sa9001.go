package sa9001

import (
	"go/ast"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA9001",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Defers in range loops may not run when you expect them to`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		loop := node.(*ast.RangeStmt)
		typ := pass.TypesInfo.TypeOf(loop.X)
		_, ok := typeutil.CoreType(typ).(*types.Chan)
		if !ok {
			return
		}

		stmts := []*ast.DeferStmt{}
		exits := false
		fn2 := func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.DeferStmt:
				stmts = append(stmts, stmt)
			case *ast.FuncLit:
				// Don't look into function bodies
				return false
			case *ast.ReturnStmt:
				exits = true
			case *ast.BranchStmt:
				exits = node.(*ast.BranchStmt).Tok == token.BREAK
			}
			return true
		}
		ast.Inspect(loop.Body, fn2)

		if exits {
			return
		}
		for _, stmt := range stmts {
			report.Report(pass, stmt, "defers in this range loop won't run unless the channel gets closed")
		}
	}
	code.Preorder(pass, fn, (*ast.RangeStmt)(nil))
	return nil, nil
}
