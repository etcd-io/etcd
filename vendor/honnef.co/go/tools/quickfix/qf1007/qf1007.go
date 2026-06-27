package qf1007

import (
	"go/ast"
	"go/token"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "QF1007",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "Merge conditional assignment into variable declaration",
		Before: `
x := false
if someCondition {
    x = true
}`,
		After:    `x := someCondition`,
		Since:    "2021.1",
		Severity: lint.SeverityHint,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkConditionalAssignmentQ = pattern.MustParse(`(AssignStmt x@(Object _) ":=" assign@(Builtin b@(Or "true" "false")))`)
var checkConditionalAssignmentIfQ = pattern.MustParse(`(IfStmt nil cond [(AssignStmt x@(Object _) "=" (Builtin b@(Or "true" "false")))] nil)`)

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		var body *ast.BlockStmt
		switch node := node.(type) {
		case *ast.FuncDecl:
			body = node.Body
		case *ast.FuncLit:
			body = node.Body
		default:
			panic("unreachable")
		}
		if body == nil {
			return
		}

		stmts := body.List
		if len(stmts) < 2 {
			return
		}
		for i, first := range stmts[:len(stmts)-1] {
			second := stmts[i+1]
			m1, ok := code.Match(pass, checkConditionalAssignmentQ, first)
			if !ok {
				continue
			}
			m2, ok := code.Match(pass, checkConditionalAssignmentIfQ, second)
			if !ok {
				continue
			}
			if m1.State["x"] != m2.State["x"] {
				continue
			}
			if m1.State["b"] == m2.State["b"] {
				continue
			}

			v := m2.State["cond"].(ast.Expr)
			if m1.State["b"] == "true" {
				v = &ast.UnaryExpr{
					Op: token.NOT,
					X:  v,
				}
			}
			report.Report(pass, first, "could merge conditional assignment into variable declaration",
				report.Fixes(edit.Fix("Merge conditional assignment into variable declaration",
					edit.ReplaceWithNode(pass.Fset, m1.State["assign"].(ast.Node), v),
					edit.Delete(second))))
		}
	}
	code.Preorder(pass, fn, (*ast.FuncDecl)(nil), (*ast.FuncLit)(nil))
	return nil, nil
}
