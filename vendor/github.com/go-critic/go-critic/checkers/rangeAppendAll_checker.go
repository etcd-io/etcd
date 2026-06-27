package checkers

import (
	"go/ast"
	"go/token"

	"github.com/go-critic/go-critic/checkers/internal/astwalk"
	"github.com/go-critic/go-critic/linter"
	"github.com/go-toolsmith/astcast"
	"golang.org/x/tools/go/ast/astutil"
)

func init() {
	var info linter.CheckerInfo
	info.Name = "rangeAppendAll"
	info.Tags = []string{linter.DiagnosticTag, linter.ExperimentalTag}
	info.Summary = "Detects append all its data while range it"
	info.Before = `for _, n := range ns {
	...
		rs = append(rs, ns...) // append all slice data
	}
}`
	info.After = `for _, n := range ns {
	...
		rs = append(rs, n)
	}
}`

	collection.AddChecker(&info, func(ctx *linter.CheckerContext) (linter.FileWalker, error) {
		c := &rangeAppendAllChecker{ctx: ctx}
		return astwalk.WalkerForStmt(c), nil
	})
}

type rangeAppendAllChecker struct {
	astwalk.WalkHandler
	ctx *linter.CheckerContext
}

func (c *rangeAppendAllChecker) VisitStmt(stmt ast.Stmt) {
	rangeStmt, ok := stmt.(*ast.RangeStmt)
	if !ok || len(rangeStmt.Body.List) == 0 {
		return
	}
	rangeIdent, ok := rangeStmt.X.(*ast.Ident)
	if !ok {
		return
	}
	rangeObj := c.ctx.TypesInfo.ObjectOf(rangeIdent)

	astutil.Apply(rangeStmt.Body, nil, func(cur *astutil.Cursor) bool {
		appendFrom := c.getValidAppendFrom(cur.Node())
		if appendFrom != nil {
			appendFromObj := c.ctx.TypesInfo.ObjectOf(appendFrom)
			if appendFromObj == rangeObj {
				c.warn(appendFrom)
			}
		}
		return true
	})
}

func (c *rangeAppendAllChecker) getValidAppendFrom(expr ast.Node) *ast.Ident {
	call := astcast.ToCallExpr(expr)
	if len(call.Args) != 2 || call.Ellipsis == token.NoPos {
		return nil
	}
	if qualifiedName(call.Fun) != "append" {
		return nil
	}
	if c.isSliceLiteral(call.Args[0]) {
		return nil
	}
	appendFrom, ok := call.Args[1].(*ast.Ident)
	if !ok {
		return nil
	}
	return appendFrom
}

func (c *rangeAppendAllChecker) isSliceLiteral(arg ast.Expr) bool {
	switch v := arg.(type) {
	// []T{}, []T{n}
	case *ast.CompositeLit:
		return true
	// []T(nil)
	case *ast.CallExpr:
		if astcast.ToArrayType(v.Fun) != astcast.NilArrayType && len(v.Args) == 1 {
			id := astcast.ToIdent(v.Args[0])
			return id.Name == "nil" && id.Obj == nil
		}
		return false
	default:
		return false
	}
}

func (c *rangeAppendAllChecker) warn(appendFrom *ast.Ident) {
	c.ctx.Warn(appendFrom, "append all `%s` data while range it", appendFrom)
}
