// Copyright 2016 PingCAP, Inc.
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

package expression

import (
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tipb/go-tipb"
	log "github.com/sirupsen/logrus"
)

// ExpressionsToPB converts expression to tipb.Expr.
func ExpressionsToPB(sc *stmtctx.StatementContext, exprs []Expression, client kv.Client) (pbCNF *tipb.Expr, pushed []Expression, remained []Expression) {
	pc := PbConverter{client: client, sc: sc}
	retTypeOfAnd := &types.FieldType{
		Tp:      mysql.TypeLonglong,
		Flen:    1,
		Decimal: 0,
		Flag:    mysql.BinaryFlag,
		Charset: charset.CharsetBin,
		Collate: charset.CollationBin,
	}

	for _, expr := range exprs {
		pbExpr := pc.ExprToPB(expr)
		if pbExpr == nil {
			remained = append(remained, expr)
			continue
		}

		pushed = append(pushed, expr)
		if pbCNF == nil {
			pbCNF = pbExpr
			continue
		}

		// Merge multiple converted pb expression into a CNF.
		pbCNF = &tipb.Expr{
			Tp:        tipb.ExprType_ScalarFunc,
			Sig:       tipb.ScalarFuncSig_LogicalAnd,
			Children:  []*tipb.Expr{pbCNF, pbExpr},
			FieldType: ToPBFieldType(retTypeOfAnd),
		}
	}
	return
}

// ExpressionsToPBList converts expressions to tipb.Expr list for new plan.
func ExpressionsToPBList(sc *stmtctx.StatementContext, exprs []Expression, client kv.Client) (pbExpr []*tipb.Expr) {
	pc := PbConverter{client: client, sc: sc}
	for _, expr := range exprs {
		v := pc.ExprToPB(expr)
		pbExpr = append(pbExpr, v)
	}
	return
}

// PbConverter supplys methods to convert TiDB expressions to TiPB.
type PbConverter struct {
	client kv.Client
	sc     *stmtctx.StatementContext
}

// NewPBConverter creates a PbConverter.
func NewPBConverter(client kv.Client, sc *stmtctx.StatementContext) PbConverter {
	return PbConverter{client: client, sc: sc}
}

// ExprToPB converts Expression to TiPB.
func (pc PbConverter) ExprToPB(expr Expression) *tipb.Expr {
	switch x := expr.(type) {
	case *Constant, *CorrelatedColumn:
		return pc.conOrCorColToPBExpr(expr)
	case *Column:
		return pc.columnToPBExpr(x)
	case *ScalarFunction:
		return pc.scalarFuncToPBExpr(x)
	}
	return nil
}

func (pc PbConverter) conOrCorColToPBExpr(expr Expression) *tipb.Expr {
	ft := expr.GetType()
	d, err := expr.Eval(chunk.Row{})
	if err != nil {
		log.Errorf("Fail to eval constant, err: %s", err.Error())
		return nil
	}
	tp, val, ok := pc.encodeDatum(d)
	if !ok {
		return nil
	}

	if !pc.client.IsRequestTypeSupported(kv.ReqTypeSelect, int64(tp)) {
		return nil
	}
	return &tipb.Expr{Tp: tp, Val: val, FieldType: ToPBFieldType(ft)}
}

func (pc *PbConverter) encodeDatum(d types.Datum) (tipb.ExprType, []byte, bool) {
	var (
		tp  tipb.ExprType
		val []byte
	)
	switch d.Kind() {
	case types.KindNull:
		tp = tipb.ExprType_Null
	case types.KindInt64:
		tp = tipb.ExprType_Int64
		val = codec.EncodeInt(nil, d.GetInt64())
	case types.KindUint64:
		tp = tipb.ExprType_Uint64
		val = codec.EncodeUint(nil, d.GetUint64())
	case types.KindString, types.KindBinaryLiteral:
		tp = tipb.ExprType_String
		val = d.GetBytes()
	case types.KindBytes:
		tp = tipb.ExprType_Bytes
		val = d.GetBytes()
	case types.KindFloat32:
		tp = tipb.ExprType_Float32
		val = codec.EncodeFloat(nil, d.GetFloat64())
	case types.KindFloat64:
		tp = tipb.ExprType_Float64
		val = codec.EncodeFloat(nil, d.GetFloat64())
	case types.KindMysqlDuration:
		tp = tipb.ExprType_MysqlDuration
		val = codec.EncodeInt(nil, int64(d.GetMysqlDuration().Duration))
	case types.KindMysqlDecimal:
		tp = tipb.ExprType_MysqlDecimal
		var err error
		val, err = codec.EncodeDecimal(nil, d.GetMysqlDecimal(), d.Length(), d.Frac())
		if err != nil {
			log.Errorf("Fail to encode value, err: %s", err.Error())
			return tp, nil, false
		}
	case types.KindMysqlTime:
		if pc.client.IsRequestTypeSupported(kv.ReqTypeDAG, int64(tipb.ExprType_MysqlTime)) {
			tp = tipb.ExprType_MysqlTime
			loc := pc.sc.TimeZone
			t := d.GetMysqlTime()
			if t.Type == mysql.TypeTimestamp && loc != time.UTC {
				err := t.ConvertTimeZone(loc, time.UTC)
				terror.Log(errors.Trace(err))
			}
			v, err := t.ToPackedUint()
			if err != nil {
				log.Errorf("Fail to encode value, err: %s", err.Error())
				return tp, nil, false
			}
			val = codec.EncodeUint(nil, v)
			return tp, val, true
		}
		return tp, nil, false
	default:
		return tp, nil, false
	}
	return tp, val, true
}

// ToPBFieldType converts *types.FieldType to *tipb.FieldType.
func ToPBFieldType(ft *types.FieldType) *tipb.FieldType {
	return &tipb.FieldType{
		Tp:      int32(ft.Tp),
		Flag:    uint32(ft.Flag),
		Flen:    int32(ft.Flen),
		Decimal: int32(ft.Decimal),
		Charset: ft.Charset,
		Collate: collationToProto(ft.Collate),
	}
}

func collationToProto(c string) int32 {
	v, ok := mysql.CollationNames[c]
	if ok {
		return int32(v)
	}
	return int32(mysql.DefaultCollationID)
}

func (pc PbConverter) columnToPBExpr(column *Column) *tipb.Expr {
	if !pc.client.IsRequestTypeSupported(kv.ReqTypeSelect, int64(tipb.ExprType_ColumnRef)) {
		return nil
	}
	switch column.GetType().Tp {
	case mysql.TypeBit, mysql.TypeSet, mysql.TypeEnum, mysql.TypeGeometry, mysql.TypeUnspecified:
		return nil
	}

	if pc.client.IsRequestTypeSupported(kv.ReqTypeDAG, kv.ReqSubTypeBasic) {
		return &tipb.Expr{
			Tp:        tipb.ExprType_ColumnRef,
			Val:       codec.EncodeInt(nil, int64(column.Index)),
			FieldType: ToPBFieldType(column.RetType),
		}
	}
	id := column.ID
	// Zero Column ID is not a column from table, can not support for now.
	if id == 0 || id == -1 {
		return nil
	}

	return &tipb.Expr{
		Tp:  tipb.ExprType_ColumnRef,
		Val: codec.EncodeInt(nil, id)}
}

func (pc PbConverter) scalarFuncToPBExpr(expr *ScalarFunction) *tipb.Expr {
	// check whether this function can be pushed.
	if !pc.canFuncBePushed(expr) {
		return nil
	}

	// check whether this function has ProtoBuf signature.
	pbCode := expr.Function.PbCode()
	if pbCode < 0 {
		return nil
	}

	// check whether all of its parameters can be pushed.
	children := make([]*tipb.Expr, 0, len(expr.GetArgs()))
	for _, arg := range expr.GetArgs() {
		pbArg := pc.ExprToPB(arg)
		if pbArg == nil {
			return nil
		}
		children = append(children, pbArg)
	}

	// construct expression ProtoBuf.
	return &tipb.Expr{
		Tp:        tipb.ExprType_ScalarFunc,
		Sig:       pbCode,
		Children:  children,
		FieldType: ToPBFieldType(expr.RetType),
	}
}

// GroupByItemToPB converts group by items to pb.
func GroupByItemToPB(sc *stmtctx.StatementContext, client kv.Client, expr Expression) *tipb.ByItem {
	pc := PbConverter{client: client, sc: sc}
	e := pc.ExprToPB(expr)
	if e == nil {
		return nil
	}
	return &tipb.ByItem{Expr: e}
}

// SortByItemToPB converts order by items to pb.
func SortByItemToPB(sc *stmtctx.StatementContext, client kv.Client, expr Expression, desc bool) *tipb.ByItem {
	pc := PbConverter{client: client, sc: sc}
	e := pc.ExprToPB(expr)
	if e == nil {
		return nil
	}
	return &tipb.ByItem{Expr: e, Desc: desc}
}

func (pc PbConverter) canFuncBePushed(sf *ScalarFunction) bool {
	switch sf.FuncName.L {
	case
		// logical functions.
		ast.LogicAnd,
		ast.LogicOr,
		ast.UnaryNot,

		// compare functions.
		ast.LT,
		ast.LE,
		ast.EQ,
		ast.NE,
		ast.GE,
		ast.GT,
		ast.NullEQ,
		ast.In,
		ast.IsNull,
		ast.Like,
		ast.IsTruth,
		ast.IsFalsity,

		// arithmetical functions.
		ast.Plus,
		ast.Minus,
		ast.Mul,
		ast.Div,
		ast.Abs,
		ast.Ceil,
		ast.Ceiling,
		ast.Floor,

		// control flow functions.
		ast.Case,
		ast.If,
		ast.Ifnull,
		ast.Coalesce,

		// json functions.
		ast.JSONType,
		ast.JSONExtract,
		ast.JSONUnquote,
		ast.JSONObject,
		ast.JSONArray,
		ast.JSONMerge,
		ast.JSONSet,
		ast.JSONInsert,
		ast.JSONReplace,
		ast.JSONRemove,

		// date functions.
		ast.DateFormat:

		return true
	}
	return false
}
