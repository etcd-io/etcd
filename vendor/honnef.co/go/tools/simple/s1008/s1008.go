package s1008

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1008",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Simplify returning boolean expression`,
		Before: `
if <expr> {
    return true
}
return false`,
		After:   `return <expr>`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkIfReturnQIf  = pattern.MustParse(`(IfStmt nil cond [(ReturnStmt [ret@(Builtin (Or "true" "false"))])] nil)`)
	checkIfReturnQRet = pattern.MustParse(`(ReturnStmt [ret@(Builtin (Or "true" "false"))])`)
)

func run(pass *analysis.Pass) (any, error) {
	var cm ast.CommentMap
	fn := func(node ast.Node) {
		if f, ok := node.(*ast.File); ok {
			cm = ast.NewCommentMap(pass.Fset, f, f.Comments)
			return
		}

		block := node.(*ast.BlockStmt)
		l := len(block.List)
		if l < 2 {
			return
		}
		n1, n2 := block.List[l-2], block.List[l-1]

		if len(block.List) >= 3 {
			if _, ok := block.List[l-3].(*ast.IfStmt); ok {
				// Do not flag a series of if statements
				return
			}
		}
		m1, ok := code.Match(pass, checkIfReturnQIf, n1)
		if !ok {
			return
		}
		m2, ok := code.Match(pass, checkIfReturnQRet, n2)
		if !ok {
			return
		}

		if op, ok := m1.State["cond"].(*ast.BinaryExpr); ok {
			switch op.Op {
			case token.EQL, token.LSS, token.GTR, token.NEQ, token.LEQ, token.GEQ:
			default:
				return
			}
		}

		ret1 := m1.State["ret"].(*ast.Ident)
		ret2 := m2.State["ret"].(*ast.Ident)
		if ret1.Name == ret2.Name {
			// we want the function to return true and false, not the
			// same value both times.
			return
		}

		hasComments := func(n ast.Node) bool {
			cmf := cm.Filter(n)
			for _, groups := range cmf {
				for _, group := range groups {
					for _, cmt := range group.List {
						if strings.HasPrefix(cmt.Text, "//@ diag") {
							// Staticcheck test cases use comments to mark
							// expected diagnostics. Ignore these comments so we
							// can test this check.
							continue
						}
						return true
					}
				}
			}
			return false
		}

		// Don't flag if either branch is commented
		if hasComments(n1) || hasComments(n2) {
			return
		}

		cond := m1.State["cond"].(ast.Expr)
		origCond := cond
		if ret1.Name == "false" {
			cond = negate(pass, cond)
		}
		report.Report(pass, n1,
			fmt.Sprintf("should use 'return %s' instead of 'if %s { return %s }; return %s'",
				report.Render(pass, cond),
				report.Render(pass, origCond), report.Render(pass, ret1), report.Render(pass, ret2)),
			report.FilterGenerated())
	}
	code.Preorder(pass, fn, (*ast.File)(nil), (*ast.BlockStmt)(nil))
	return nil, nil
}

func negate(pass *analysis.Pass, expr ast.Expr) ast.Expr {
	switch expr := expr.(type) {
	case *ast.BinaryExpr:
		out := *expr
		switch expr.Op {
		case token.EQL:
			out.Op = token.NEQ
		case token.LSS:
			out.Op = token.GEQ
		case token.GTR:
			// Some builtins never return negative ints; "len(x) <= 0" should be "len(x) == 0".
			if call, ok := expr.X.(*ast.CallExpr); ok &&
				code.IsCallToAny(pass, call, "len", "cap", "copy") &&
				code.IsIntegerLiteral(pass, expr.Y, constant.MakeInt64(0)) {
				out.Op = token.EQL
			} else {
				out.Op = token.LEQ
			}
		case token.NEQ:
			out.Op = token.EQL
		case token.LEQ:
			out.Op = token.GTR
		case token.GEQ:
			out.Op = token.LSS
		}
		return &out
	case *ast.Ident, *ast.CallExpr, *ast.IndexExpr, *ast.StarExpr:
		return &ast.UnaryExpr{
			Op: token.NOT,
			X:  expr,
		}
	case *ast.UnaryExpr:
		if expr.Op == token.NOT {
			return expr.X
		}
		return &ast.UnaryExpr{
			Op: token.NOT,
			X:  expr,
		}
	default:
		return &ast.UnaryExpr{
			Op: token.NOT,
			X: &ast.ParenExpr{
				X: expr,
			},
		}
	}
}
