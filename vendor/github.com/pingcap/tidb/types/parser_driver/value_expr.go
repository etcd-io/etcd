// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package driver

import (
	"fmt"
	"io"
	"strconv"

	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/hack"
)

// The purpose of driver package is to decompose the dependency of the parser and
// types package.
// It provides the NewValueExpr function for the ast package, so the ast package
// do not depends on the concrete definition of `types.Datum`, thus get rid of
// the dependency of the types package.
// The parser package depends on the ast package, but not the types package.
// The whole relationship:
// ast imports []
// tidb/types imports [parser/types]
// parser imports [ast, parser/types]
// driver imports [ast, tidb/types]
// tidb imports [parser, driver]

func init() {
	ast.NewValueExpr = newValueExpr
	ast.NewParamMarkerExpr = newParamMarkerExpr
	ast.NewDecimal = func(str string) (interface{}, error) {
		dec := new(types.MyDecimal)
		err := dec.FromString(hack.Slice(str))
		return dec, err
	}
	ast.NewHexLiteral = func(str string) (interface{}, error) {
		h, err := types.NewHexLiteral(str)
		return h, err
	}
	ast.NewBitLiteral = func(str string) (interface{}, error) {
		b, err := types.NewBitLiteral(str)
		return b, err
	}
}

var (
	_ ast.ParamMarkerExpr = &ParamMarkerExpr{}
	_ ast.ValueExpr       = &ValueExpr{}
)

// ValueExpr is the simple value expression.
type ValueExpr struct {
	ast.TexprNode
	types.Datum
	projectionOffset int
}

// GetDatumString implements the ast.ValueExpr interface.
func (n *ValueExpr) GetDatumString() string {
	return n.GetString()
}

// Format the ExprNode into a Writer.
func (n *ValueExpr) Format(w io.Writer) {
	var s string
	switch n.Kind() {
	case types.KindNull:
		s = "NULL"
	case types.KindInt64:
		if n.Type.Flag&mysql.IsBooleanFlag != 0 {
			if n.GetInt64() > 0 {
				s = "TRUE"
			} else {
				s = "FALSE"
			}
		} else {
			s = strconv.FormatInt(n.GetInt64(), 10)
		}
	case types.KindUint64:
		s = strconv.FormatUint(n.GetUint64(), 10)
	case types.KindFloat32:
		s = strconv.FormatFloat(n.GetFloat64(), 'e', -1, 32)
	case types.KindFloat64:
		s = strconv.FormatFloat(n.GetFloat64(), 'e', -1, 64)
	case types.KindString, types.KindBytes:
		s = strconv.Quote(n.GetString())
	case types.KindMysqlDecimal:
		s = n.GetMysqlDecimal().String()
	case types.KindBinaryLiteral:
		if n.Type.Flag&mysql.UnsignedFlag != 0 {
			s = fmt.Sprintf("x'%x'", n.GetBytes())
		} else {
			s = n.GetBinaryLiteral().ToBitLiteralString(true)
		}
	default:
		panic("Can't format to string")
	}
	fmt.Fprint(w, s)
}

// newValueExpr creates a ValueExpr with value, and sets default field type.
func newValueExpr(value interface{}) ast.ValueExpr {
	if ve, ok := value.(*ValueExpr); ok {
		return ve
	}
	ve := &ValueExpr{}
	ve.SetValue(value)
	types.DefaultTypeForValue(value, &ve.Type)
	ve.projectionOffset = -1
	return ve
}

// SetProjectionOffset sets ValueExpr.projectionOffset for logical plan builder.
func (n *ValueExpr) SetProjectionOffset(offset int) {
	n.projectionOffset = offset
}

// GetProjectionOffset returns ValueExpr.projectionOffset.
func (n *ValueExpr) GetProjectionOffset() int {
	return n.projectionOffset
}

// Accept implements Node interface.
func (n *ValueExpr) Accept(v ast.Visitor) (ast.Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*ValueExpr)
	return v.Leave(n)
}

// ParamMarkerExpr expression holds a place for another expression.
// Used in parsing prepare statement.
type ParamMarkerExpr struct {
	ValueExpr
	Offset int
	Order  int
}

func newParamMarkerExpr(offset int) ast.ParamMarkerExpr {
	return &ParamMarkerExpr{
		Offset: offset,
	}
}

// Format the ExprNode into a Writer.
func (n *ParamMarkerExpr) Format(w io.Writer) {
	panic("Not implemented")
}

// Accept implements Node Accept interface.
func (n *ParamMarkerExpr) Accept(v ast.Visitor) (ast.Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*ParamMarkerExpr)
	return v.Leave(n)
}

// SetOrder implements the ast.ParamMarkerExpr interface.
func (n *ParamMarkerExpr) SetOrder(order int) {
	n.Order = order
}
