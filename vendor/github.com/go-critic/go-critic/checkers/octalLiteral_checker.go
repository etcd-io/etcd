package checkers

import (
	"go/ast"
	"go/token"
	"strings"
	"unicode"

	"github.com/go-critic/go-critic/checkers/internal/astwalk"
	"github.com/go-critic/go-critic/linter"

	"github.com/go-toolsmith/astcast"
)

func init() {
	var info linter.CheckerInfo
	info.Name = "octalLiteral"
	info.Tags = []string{linter.StyleTag, linter.ExperimentalTag, linter.OpinionatedTag}
	info.Summary = "Detects old-style octal literals"
	info.Before = `foo(02)`
	info.After = `foo(0o2)`

	collection.AddChecker(&info, func(ctx *linter.CheckerContext) (linter.FileWalker, error) {
		return astwalk.WalkerForExpr(&octalLiteralChecker{ctx: ctx}), nil
	})
}

type octalLiteralChecker struct {
	astwalk.WalkHandler
	ctx *linter.CheckerContext
}

func (c *octalLiteralChecker) VisitExpr(expr ast.Expr) {
	if !c.ctx.GoVersion.GreaterOrEqual(linter.GoVersion{Major: 1, Minor: 13}) {
		return
	}
	lit := astcast.ToBasicLit(expr)
	if lit.Kind != token.INT {
		return
	}
	if !strings.HasPrefix(lit.Value, "0") || len(lit.Value) == 1 {
		return
	}
	if unicode.IsDigit(rune(lit.Value[1])) {
		c.warn(lit)
	}
}

func (c *octalLiteralChecker) warn(lit *ast.BasicLit) {
	c.ctx.Warn(lit, "use new octal literal style, 0o%s", lit.Value[len("0"):])
}
