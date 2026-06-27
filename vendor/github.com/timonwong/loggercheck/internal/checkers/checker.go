package checkers

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

type Config struct {
	RequireStringKey bool
	NoPrintfLike     bool
}

type CallContext struct {
	Expr      *ast.CallExpr
	Func      *types.Func
	Signature *types.Signature
}

type Checker interface {
	FilterKeyAndValues(pass *analysis.Pass, keyAndValues []ast.Expr) []ast.Expr
	CheckLoggingKey(pass *analysis.Pass, keyAndValues []ast.Expr)
	CheckPrintfLikeSpecifier(pass *analysis.Pass, args []ast.Expr)
}

func ExecuteChecker(c Checker, pass *analysis.Pass, call CallContext, cfg Config) {
	params := call.Signature.Params()
	nparams := params.Len() // variadic => nonzero
	startIndex := nparams - 1

	iface, ok := types.Unalias(params.At(startIndex).Type().(*types.Slice).Elem()).(*types.Interface)
	if !ok || !iface.Empty() {
		return // final (args) param is not ...interface{}
	}

	keyValuesArgs := c.FilterKeyAndValues(pass, call.Expr.Args[startIndex:])

	if len(keyValuesArgs)%2 != 0 {
		firstArg := keyValuesArgs[0]
		lastArg := keyValuesArgs[len(keyValuesArgs)-1]
		pass.Report(analysis.Diagnostic{
			Pos:      firstArg.Pos(),
			End:      lastArg.End(),
			Category: DiagnosticCategory,
			Message:  "odd number of arguments passed as key-value pairs for logging",
		})
	}

	if cfg.RequireStringKey {
		c.CheckLoggingKey(pass, keyValuesArgs)
	}

	if cfg.NoPrintfLike {
		// Check all args
		c.CheckPrintfLikeSpecifier(pass, call.Expr.Args)
	}
}
