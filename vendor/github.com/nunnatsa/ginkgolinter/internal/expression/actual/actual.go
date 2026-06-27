package actual

import (
	"go/ast"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/gomegahandler"
)

type Actual struct {
	Orig         *ast.CallExpr
	Clone        *ast.CallExpr
	Arg          ArgPayload
	argType      gotypes.Type
	isTuple      bool
	isAsync      bool
	asyncArg     *AsyncArg
	actualOffset int
}

func New(origExpr, cloneExpr *ast.CallExpr, clone *ast.CallExpr, pass *analysis.Pass, timePkg string, info *gomegahandler.GomegaBasicInfo) (*Actual, bool) {
	arg, actualOffset := getActualArgPayload(clone, pass, info)
	if arg == nil {
		return nil, false
	}

	argType := pass.TypesInfo.TypeOf(info.RootCall.Args[actualOffset])
	isTuple := false

	if tpl, ok := argType.(*gotypes.Tuple); ok {
		if tpl.Len() > 0 {
			argType = tpl.At(0).Type()
		} else {
			argType = nil
		}

		isTuple = tpl.Len() > 1
	}

	isAsyncExpr := info.RootCallType == gomegahandler.AsyncAssertionCall

	var asyncArg *AsyncArg
	if isAsyncExpr {
		asyncArg = newAsyncArg(origExpr, cloneExpr, info.RootCall, clone, argType, pass, actualOffset, timePkg)
	}

	return &Actual{
		Orig:         info.RootCall,
		Clone:        clone,
		Arg:          arg,
		argType:      argType,
		isTuple:      isTuple,
		isAsync:      isAsyncExpr,
		asyncArg:     asyncArg,
		actualOffset: actualOffset,
	}, true
}

func (a *Actual) ReplaceActual(newArgs ast.Expr) {
	a.Clone.Args[a.actualOffset] = newArgs
}

func (a *Actual) ReplaceActualWithItsFirstArg() {
	firstArgOfArg := a.Clone.Args[a.actualOffset].(*ast.CallExpr).Args[0]
	a.ReplaceActual(firstArgOfArg)
}

func (a *Actual) IsAsync() bool {
	return a.isAsync
}

func (a *Actual) IsTuple() bool {
	return a.isTuple
}

func (a *Actual) ArgGOType() gotypes.Type {
	return a.argType
}

func (a *Actual) GetAsyncArg() *AsyncArg {
	return a.asyncArg
}

func (a *Actual) AppendWithArgsMethod() {
	if a.asyncArg.fun != nil {
		if len(a.asyncArg.fun.Args) > 0 {
			actualOrigFunc := a.Clone.Fun
			actualOrigArgs := a.Clone.Args

			actualOrigArgs[a.actualOffset] = a.asyncArg.fun.Fun
			call := &ast.SelectorExpr{
				Sel: ast.NewIdent("WithArguments"),
				X: &ast.CallExpr{
					Fun:  actualOrigFunc,
					Args: actualOrigArgs,
				},
			}

			a.Clone.Fun = call
			a.Clone.Args = a.asyncArg.fun.Args
			a.Clone = a.Clone.Fun.(*ast.SelectorExpr).X.(*ast.CallExpr)
		} else {
			a.Clone.Args[a.actualOffset] = a.asyncArg.fun.Fun
		}
	}
}

func (a *Actual) GetActualArg() ast.Expr {
	return a.Clone.Args[a.actualOffset]
}
