package sa4009

import (
	"fmt"
	"go/ast"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4009",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `A function argument is overwritten before its first use`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		cb := func(node ast.Node) bool {
			var typ *ast.FuncType
			var body *ast.BlockStmt
			switch fn := node.(type) {
			case *ast.FuncDecl:
				typ = fn.Type
				body = fn.Body
			case *ast.FuncLit:
				typ = fn.Type
				body = fn.Body
			}
			if body == nil {
				return true
			}
			if len(typ.Params.List) == 0 {
				return true
			}
			for _, field := range typ.Params.List {
				for _, arg := range field.Names {
					obj := pass.TypesInfo.ObjectOf(arg)
					var irobj *ir.Parameter
					for _, param := range fn.Params {
						if param.Object() == obj {
							irobj = param
							break
						}
					}
					if irobj == nil {
						continue
					}
					refs := irobj.Referrers()
					if refs == nil {
						continue
					}
					if len(irutil.FilterDebug(*refs)) != 0 {
						continue
					}

					var assignment ast.Node
					ast.Inspect(body, func(node ast.Node) bool {
						if assignment != nil {
							return false
						}
						assign, ok := node.(*ast.AssignStmt)
						if !ok {
							return true
						}
						for _, lhs := range assign.Lhs {
							ident, ok := lhs.(*ast.Ident)
							if !ok {
								continue
							}
							if pass.TypesInfo.ObjectOf(ident) == obj {
								assignment = assign
								return false
							}
						}
						return true
					})
					if assignment != nil {
						report.Report(pass, arg, fmt.Sprintf("argument %s is overwritten before first use", arg),
							report.Related(assignment, fmt.Sprintf("assignment to %s", arg)))
					}
				}
			}
			return true
		}
		if source := fn.Source(); source != nil {
			ast.Inspect(source, cb)
		}
	}
	return nil, nil
}
