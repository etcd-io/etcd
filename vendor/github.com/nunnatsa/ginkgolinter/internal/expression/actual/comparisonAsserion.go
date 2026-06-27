package actual

import (
	"go/ast"
	"go/constant"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
)

type ComparisonActualPayload interface {
	GetOp() token.Token
	GetLeft() value.Valuer
	GetRight() value.Valuer
}

type FuncComparisonPayload struct {
	op      token.Token
	argType ArgType
	val     value.Valuer
	left    value.Valuer
	arg     ast.Expr
}

func newFuncComparisonPayload(origLeft, leftClone *ast.CallExpr, origRight, rightClone ast.Expr, op token.Token, pass *analysis.Pass) (*FuncComparisonPayload, bool) {
	funcName, ok := builtinFuncName(origLeft)
	if !ok {
		return nil, false
	}

	if len(origLeft.Args) != 1 {
		return nil, false
	}

	left := value.GetValuer(origLeft, leftClone, pass)
	val := value.GetValuer(origRight, rightClone, pass)

	argType := ComparisonActualArgType
	switch funcName {
	case "len":
		argType |= LenComparisonActualArgType

		if val.IsValueNumeric() {
			if val.IsValueZero() {
				switch op {
				case token.EQL:
					argType |= EqualZero

				case token.NEQ, token.GTR:
					argType |= GreaterThanZero
				}
			} else if val.GetValue().String() == "1" && op == token.GEQ {
				argType |= GreaterThanZero
			}
		}

		if !argType.Is(GreaterThanZero) && op != token.EQL && op != token.NEQ {
			return nil, false
		}

	case "cap":
		if op != token.EQL && op != token.NEQ {
			return nil, false
		}
		argType |= CapComparisonActualArgType

	default:
		return nil, false
	}

	return &FuncComparisonPayload{
		op:      op,
		argType: argType,
		val:     val,
		left:    left,
		arg:     leftClone.Args[0],
	}, true
}

func (f *FuncComparisonPayload) GetLeft() value.Valuer {
	return f.left
}

func (f *FuncComparisonPayload) GetRight() value.Valuer {
	return f.val
}

func (f *FuncComparisonPayload) ArgType() ArgType {
	return f.argType
}

func (f *FuncComparisonPayload) GetOp() token.Token {
	return f.op
}

func (f *FuncComparisonPayload) GetValue() constant.Value {
	return f.val.GetValue()
}

func (f *FuncComparisonPayload) GetType() gotypes.Type {
	return f.val.GetType()
}

func (f *FuncComparisonPayload) GetValueExpr() ast.Expr {
	return f.val.GetValueExpr()
}

func (f *FuncComparisonPayload) IsError() bool {
	return f.val.IsError()
}

func (f *FuncComparisonPayload) IsValueZero() bool {
	return f.val.IsValueZero()
}

func (f *FuncComparisonPayload) IsFunc() bool {
	return true
}

func (f *FuncComparisonPayload) IsValueNumeric() bool {
	return f.val.IsValueNumeric()
}

func (f *FuncComparisonPayload) IsValueInt() bool {
	return f.val.IsValueInt()
}

func (f *FuncComparisonPayload) IsInterface() bool {
	return f.val.IsInterface()
}

func (f *FuncComparisonPayload) IsPointer() bool {
	return f.val.IsPointer()
}

func (f *FuncComparisonPayload) GetFuncArg() ast.Expr {
	return f.arg
}

type ComparisonArgPayload struct {
	left  value.Valuer
	right value.Valuer
	op    token.Token
}

func newComparisonArgPayload(left, right value.Valuer, op token.Token) *ComparisonArgPayload {
	return &ComparisonArgPayload{
		left:  left,
		right: right,
		op:    op,
	}
}

func (*ComparisonArgPayload) ArgType() ArgType {
	return BinaryComparisonActualArgType | ComparisonActualArgType
}

func (c *ComparisonArgPayload) GetOp() token.Token {
	return c.op
}

func (c *ComparisonArgPayload) GetLeft() value.Valuer {
	return c.left
}

func (c *ComparisonArgPayload) GetRight() value.Valuer {
	return c.right
}

type NilComparisonPayload struct {
	val   value.Valuer
	right value.Valuer
	op    token.Token
}

func newNilComparisonPayload(val, right value.Valuer, op token.Token) *NilComparisonPayload {
	return &NilComparisonPayload{
		val:   val,
		right: right,
		op:    op,
	}
}

func (*NilComparisonPayload) ArgType() ArgType {
	return NilComparisonActualArgType
}

func (n *NilComparisonPayload) GetLeft() value.Valuer {
	return n.val
}

func (n *NilComparisonPayload) GetRight() value.Valuer {
	return n.right
}

func (n *NilComparisonPayload) GetType() gotypes.Type {
	return n.val.GetType()
}

func (n *NilComparisonPayload) GetValue() constant.Value {
	return n.val.GetValue()
}

func (n *NilComparisonPayload) GetValueExpr() ast.Expr {
	return n.val.GetValueExpr()
}

func (n *NilComparisonPayload) IsValueInt() bool {
	return n.val.IsValueInt()
}

func (n *NilComparisonPayload) IsError() bool {
	return n.val.IsError()
}

func (n *NilComparisonPayload) IsValueNumeric() bool {
	return n.val.IsValueNumeric()
}

func (n *NilComparisonPayload) IsFunc() bool {
	return n.val.IsFunc()
}

func (n *NilComparisonPayload) IsValueZero() bool {
	return n.val.IsValueZero()
}

func (n *NilComparisonPayload) IsInterface() bool {
	return n.val.IsInterface()
}

func (n *NilComparisonPayload) IsPointer() bool {
	return n.val.IsPointer()
}

func (n *NilComparisonPayload) GetOp() token.Token {
	return n.op
}

func builtinFuncName(callExpr *ast.CallExpr) (string, bool) {
	argFunc, ok := callExpr.Fun.(*ast.Ident)
	if !ok {
		return "", false
	}

	if len(callExpr.Args) != 1 {
		return "", false
	}

	switch name := argFunc.Name; name {
	case "len", "cap", "min", "max":
		return name, true
	default:
		return "", false
	}
}
