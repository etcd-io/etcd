package value

import (
	"go/ast"
	"go/constant"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/typecheck"
)

type Valuer interface {
	GetValue() constant.Value
	GetType() gotypes.Type
	GetValueExpr() ast.Expr
	IsValueZero() bool
	IsValueInt() bool
	IsValueNumeric() bool
	IsError() bool
	IsFunc() bool
	IsInterface() bool
	IsPointer() bool
}

func GetValuer(orig, clone ast.Expr, pass *analysis.Pass) Valuer {
	val := New(orig, clone, pass)
	unspecified := UnspecifiedValue{
		Value: val,
	}

	if orig == nil {
		return unspecified
	}

	if IsExprError(pass, orig) {
		return &ErrValue{
			Value: val,
			err:   clone,
		}
	}

	if val.GetValue() == nil || !val.tv.IsValue() {
		return unspecified
	}

	if val.GetValue().Kind() == constant.Int {
		num, ok := constant.Int64Val(val.GetValue())
		if !ok {
			return unspecified
		}
		return &IntValue{
			Value: val,
			val:   num,
		}
	}

	return unspecified
}

type Value struct {
	expr ast.Expr
	tv   gotypes.TypeAndValue
}

func New(orig, clone ast.Expr, pass *analysis.Pass) Value {
	tv := pass.TypesInfo.Types[orig]

	return Value{
		expr: clone,
		tv:   tv,
	}
}

func (v Value) GetValueExpr() ast.Expr {
	return v.expr
}

func (v Value) GetValue() constant.Value {
	return v.tv.Value
}

func (v Value) GetType() gotypes.Type {
	return v.tv.Type
}

func (v Value) IsInterface() bool {
	return gotypes.IsInterface(v.tv.Type)
}

func (v Value) IsPointer() bool {
	return Is[*gotypes.Pointer](v.tv.Type)
}

func (v Value) IsNil() bool {
	return v.tv.IsNil()
}

type UnspecifiedValue struct {
	Value
}

func (u UnspecifiedValue) IsValueZero() bool {
	return false
}

func (u UnspecifiedValue) IsValueInt() bool {
	return false
}

func (u UnspecifiedValue) IsValueNumeric() bool {
	return false
}

func (u UnspecifiedValue) IsError() bool {
	return false
}

func (u UnspecifiedValue) IsFunc() bool {
	return isFunc(u.GetValueExpr())
}

type ErrValue struct {
	Value
	err ast.Expr
}

func (e ErrValue) IsValueZero() bool {
	return false
}

func (e ErrValue) IsValueInt() bool {
	return false
}

func (e ErrValue) IsValueNumeric() bool {
	return false
}

func (e ErrValue) IsError() bool {
	return true
}

func (e ErrValue) IsFunc() bool {
	return isFunc(e.GetValueExpr())
}

type IntValuer interface {
	GetIntValue() int64
}

type IntValue struct {
	Value
	val int64
}

func (i IntValue) IsValueZero() bool {
	return i.val == 0
}

func (i IntValue) IsValueInt() bool {
	return i.val == 0
}

func (i IntValue) IsValueNumeric() bool {
	return true
}

func (i IntValue) IsError() bool {
	return false
}

func (i IntValue) IsFunc() bool {
	return false
}

func (i IntValue) GetIntValue() int64 {
	return i.val
}

func isFunc(exp ast.Expr) bool {
	return Is[*ast.CallExpr](exp)
}

func Is[T any](x any) bool {
	_, matchType := x.(T)
	return matchType
}

func IsExprError(pass *analysis.Pass, expr ast.Expr) bool {
	actualArgType := pass.TypesInfo.TypeOf(expr)
	switch t := actualArgType.(type) {
	case *gotypes.Named:
		return typecheck.ImplementsError(actualArgType)

	case *gotypes.Pointer:
		if typecheck.ImplementsError(t) {
			return true
		}

		if tt, ok := t.Elem().(*gotypes.Named); ok {
			return typecheck.ImplementsError(tt)
		}

	case *gotypes.Tuple:
		if t.Len() > 0 {
			switch t0 := t.At(0).Type().(type) {
			case *gotypes.Named, *gotypes.Pointer:
				if typecheck.ImplementsError(t0) {
					return true
				}
			}
		}
	}
	return false
}

func IsNil(exp ast.Expr, pass *analysis.Pass) bool {
	id, ok := exp.(*ast.Ident)
	if !ok {
		return false
	}

	return pass.TypesInfo.Types[id].IsNil()
}
