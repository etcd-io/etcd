package actual

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
	"github.com/nunnatsa/ginkgolinter/internal/ginkgoinfo"
	"github.com/nunnatsa/ginkgolinter/internal/gomegahandler"
	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"
	"github.com/nunnatsa/ginkgolinter/internal/reverseassertion"
)

type ArgType uint64

const (
	UnknownActualArgType ArgType = 1 << iota
	ErrActualArgType
	LenFuncActualArgType
	CapFuncActualArgType
	ComparisonActualArgType
	LenComparisonActualArgType
	CapComparisonActualArgType
	NilComparisonActualArgType
	BinaryComparisonActualArgType
	FuncSigArgType
	ErrFuncActualArgType
	GomegaParamArgType
	TBParamArgType
	MultiRetsArgType
	ErrorMethodArgType

	ErrorTypeArgType

	EqualZero
	GreaterThanZero
)

func (a ArgType) Is(val ArgType) bool {
	return a&val != 0
}

func getActualArgPayload(actualExprClone *ast.CallExpr, pass *analysis.Pass, info *gomegahandler.GomegaBasicInfo) (ArgPayload, int) {
	origArgExpr, argExprClone, actualOffset, isGomegaExpr := getActualArg(actualExprClone, info, pass)
	if !isGomegaExpr {
		return nil, 0
	}

	var arg ArgPayload

	if info.HasErrorMethod {
		arg = &ErrorMethodPayload{}
	} else if value.IsExprError(pass, origArgExpr) {
		arg = newErrPayload(origArgExpr, argExprClone, pass)
	} else {
		switch expr := origArgExpr.(type) {
		case *ast.CallExpr:
			arg = newFuncCallArgPayload(expr, argExprClone.(*ast.CallExpr))

		case *ast.BinaryExpr:
			arg = parseBinaryExpr(expr, argExprClone.(*ast.BinaryExpr), pass)
		}
	}

	if arg != nil {
		return arg, actualOffset
	}

	t := pass.TypesInfo.TypeOf(origArgExpr)
	if sig, ok := t.(*gotypes.Signature); ok {
		arg = getAsyncFuncArg(sig)
		if arg != nil {
			return arg, actualOffset
		}
	}

	return newRegularArgPayload(origArgExpr, argExprClone, pass), actualOffset
}

func getActualArg(actualExprClone *ast.CallExpr, info *gomegahandler.GomegaBasicInfo, pass *analysis.Pass) (ast.Expr, ast.Expr, int, bool) {
	var (
		origArgExpr    ast.Expr
		argExprClone   ast.Expr
		origActualExpr = info.RootCall
	)

	funcOffset := gomegainfo.ActualArgOffset(info.MethodName)
	if funcOffset < 0 {
		return nil, nil, 0, false
	}

	if len(origActualExpr.Args) <= funcOffset {
		return nil, nil, 0, false
	}

	origArgExpr = origActualExpr.Args[funcOffset]
	argExprClone = actualExprClone.Args[funcOffset]

	if info.RootCallType == gomegahandler.AsyncAssertionCall {
		if ginkgoinfo.IsGinkgoContext(pass.TypesInfo.TypeOf(origArgExpr)) {
			funcOffset++
			if len(origActualExpr.Args) <= funcOffset {
				return nil, nil, 0, false
			}

			origArgExpr = origActualExpr.Args[funcOffset]
			argExprClone = actualExprClone.Args[funcOffset]
		}
	}

	return origArgExpr, argExprClone, funcOffset, true
}

type ArgPayload interface {
	ArgType() ArgType
}

type RegularArgPayload struct {
	value.Value
}

func newRegularArgPayload(orig, clone ast.Expr, pass *analysis.Pass) *RegularArgPayload {
	return &RegularArgPayload{
		Value: value.New(orig, clone, pass),
	}
}

func (*RegularArgPayload) ArgType() ArgType {
	return UnknownActualArgType
}

type FuncCallArgPayload struct {
	argType ArgType

	origFunc  *ast.CallExpr
	cloneFunc *ast.CallExpr

	origVal  ast.Expr
	cloneVal ast.Expr
}

func newFuncCallArgPayload(orig, clone *ast.CallExpr) ArgPayload {
	funcName, ok := builtinFuncName(orig)
	if !ok {
		return nil
	}

	if len(orig.Args) != 1 {
		return nil
	}

	var argType ArgType
	switch funcName {
	case "len":
		argType = LenFuncActualArgType
	case "cap":
		argType = CapFuncActualArgType
	default:
		return nil
	}

	return &FuncCallArgPayload{
		argType:   argType,
		origFunc:  orig,
		cloneFunc: clone,
		origVal:   orig.Args[0],
		cloneVal:  clone.Args[0],
	}
}

func (f *FuncCallArgPayload) ArgType() ArgType {
	return f.argType
}

type ErrPayload struct {
	value.Valuer
}

func newErrPayload(orig, clone ast.Expr, pass *analysis.Pass) *ErrPayload {
	return &ErrPayload{
		Valuer: value.GetValuer(orig, clone, pass),
	}
}

func (*ErrPayload) ArgType() ArgType {
	return ErrActualArgType | ErrorTypeArgType
}

type ErrorMethodPayload struct{}

func (ErrorMethodPayload) ArgType() ArgType {
	return ErrorMethodArgType | ErrorTypeArgType
}

func parseBinaryExpr(origActualExpr, argExprClone *ast.BinaryExpr, pass *analysis.Pass) ArgPayload {
	left, right, op := origActualExpr.X, origActualExpr.Y, origActualExpr.Op
	replace := false
	switch realFirst := left.(type) {
	case *ast.Ident: // check if const
		info, ok := pass.TypesInfo.Types[realFirst]
		if ok {
			if value.Is[*gotypes.Basic](info.Type) && (info.Value != nil || info.IsNil()) {
				replace = true
			}
		}

	case *ast.BasicLit:
		replace = true
	}

	if replace {
		left, right = right, left
	}

	switch op {
	case token.EQL:
	case token.NEQ:
	case token.GTR, token.GEQ, token.LSS, token.LEQ:
		if replace {
			op = reverseassertion.ChangeCompareOperator(op)
		}
	default:
		return nil
	}

	leftClone, rightClone := argExprClone.X, argExprClone.Y
	if replace {
		leftClone, rightClone = rightClone, leftClone
	}

	leftVal := value.GetValuer(left, leftClone, pass)
	rightVal := value.GetValuer(right, rightClone, pass)

	if value.IsNil(right, pass) {
		return newNilComparisonPayload(leftVal, rightVal, op)
	}

	leftVal.IsFunc()
	if firstFunc, ok := left.(*ast.CallExpr); ok {
		if payload, ok := newFuncComparisonPayload(firstFunc, leftClone.(*ast.CallExpr), right, rightClone, op, pass); ok {
			return payload
		}
	}

	return newComparisonArgPayload(leftVal, rightVal, op)
}
