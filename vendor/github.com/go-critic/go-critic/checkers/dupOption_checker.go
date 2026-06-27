package checkers

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/go-critic/go-critic/checkers/internal/astwalk"
	"github.com/go-critic/go-critic/linter"
	"github.com/go-toolsmith/astcast"
	"github.com/go-toolsmith/astfmt"
)

func init() {
	var info linter.CheckerInfo
	info.Name = "dupOption"
	info.Tags = []string{linter.DiagnosticTag, linter.ExperimentalTag}
	info.Summary = "Detects duplicated option function arguments in variadic function calls"
	info.Before = `doSomething(name,
		withWidth(w),
		withHeight(h),
		withWidth(w),
)`
	info.After = `doSomething(name,
		withWidth(w),
		withHeight(h),
)`

	collection.AddChecker(&info, func(ctx *linter.CheckerContext) (linter.FileWalker, error) {
		c := &dupOptionChecker{ctx: ctx}
		return astwalk.WalkerForExpr(c), nil
	})
}

type dupOptionChecker struct {
	astwalk.WalkHandler
	ctx *linter.CheckerContext
}

func (c *dupOptionChecker) VisitExpr(expr ast.Expr) {
	call := astcast.ToCallExpr(expr)
	variadicArgs, argType := c.getVariadicArgs(call)
	if len(variadicArgs) == 0 {
		return
	}

	if !c.isOptionType(argType) {
		return
	}

	dupArgs := c.findDupArgs(variadicArgs)
	for _, arg := range dupArgs {
		c.warn(arg)
	}
}

func (c *dupOptionChecker) getVariadicArgs(call *ast.CallExpr) ([]ast.Expr, types.Type) {
	if len(call.Args) == 0 {
		return nil, nil
	}

	// skip for someFunc(a, b ...)
	if call.Ellipsis != token.NoPos {
		return nil, nil
	}

	funType := c.ctx.TypeOf(call.Fun)
	sign, ok := funType.(*types.Signature)
	if !ok || !sign.Variadic() {
		return nil, nil
	}

	last := sign.Params().Len() - 1
	sliceType, ok := sign.Params().At(last).Type().(*types.Slice)
	if !ok {
		return nil, nil
	}

	if last > len(call.Args) {
		return nil, nil
	}

	argType := sliceType.Elem()
	return call.Args[last:], argType
}

func (c *dupOptionChecker) isOptionType(typeInfo types.Type) bool {
	typeInfo = typeInfo.Underlying()

	sign, ok := typeInfo.(*types.Signature)
	if !ok {
		return false
	}

	if sign.Params().Len() == 0 {
		return false
	}

	return true
}

func (c *dupOptionChecker) findDupArgs(args []ast.Expr) []ast.Expr {
	codeMap := make(map[string]bool)
	dupArgs := make([]ast.Expr, 0)
	for _, arg := range args {
		code := astfmt.Sprint(arg)
		if codeMap[code] {
			dupArgs = append(dupArgs, arg)
			continue
		}
		codeMap[code] = true
	}
	return dupArgs
}

func (c *dupOptionChecker) warn(arg ast.Node) {
	c.ctx.Warn(arg, "function argument `%s` is duplicated", arg)
}
