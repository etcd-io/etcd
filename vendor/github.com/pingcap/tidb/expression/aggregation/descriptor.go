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

package aggregation

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cznic/mathutil"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/types"
)

// AggFuncDesc describes an aggregation function signature, only used in planner.
type AggFuncDesc struct {
	// Name represents the aggregation function name.
	Name string
	// Args represents the arguments of the aggregation function.
	Args []expression.Expression
	// RetTp represents the return type of the aggregation function.
	RetTp *types.FieldType
	// Mode represents the execution mode of the aggregation function.
	Mode AggFunctionMode
	// HasDistinct represents whether the aggregation function contains distinct attribute.
	HasDistinct bool
}

// NewAggFuncDesc creates an aggregation function signature descriptor.
func NewAggFuncDesc(ctx sessionctx.Context, name string, args []expression.Expression, hasDistinct bool) *AggFuncDesc {
	a := &AggFuncDesc{
		Name:        strings.ToLower(name),
		Args:        args,
		HasDistinct: hasDistinct,
	}
	a.typeInfer(ctx)
	return a
}

// Equal checks whether two aggregation function signatures are equal.
func (a *AggFuncDesc) Equal(ctx sessionctx.Context, other *AggFuncDesc) bool {
	if a.Name != other.Name || a.HasDistinct != other.HasDistinct || len(a.Args) != len(other.Args) {
		return false
	}
	for i := range a.Args {
		if !a.Args[i].Equal(ctx, other.Args[i]) {
			return false
		}
	}
	return true
}

// Clone copies an aggregation function signature totally.
func (a *AggFuncDesc) Clone() *AggFuncDesc {
	clone := *a
	newTp := *a.RetTp
	clone.RetTp = &newTp
	for i := range a.Args {
		clone.Args[i] = a.Args[i].Clone()
	}
	return &clone
}

// Split splits `a` into two aggregate descriptors for partial phase and
// final phase individually.
// This function is only used when executing aggregate function parallelly.
// ordinal indicates the column ordinal of the intermediate result.
func (a *AggFuncDesc) Split(ordinal []int) (partialAggDesc, finalAggDesc *AggFuncDesc) {
	partialAggDesc = a.Clone()
	if a.Mode == CompleteMode {
		partialAggDesc.Mode = Partial1Mode
	} else if a.Mode == FinalMode {
		partialAggDesc.Mode = Partial2Mode
	} else {
		panic("Error happened during AggFuncDesc.Split, the AggFunctionMode is not CompleteMode or FinalMode.")
	}
	finalAggDesc = &AggFuncDesc{
		Name:        a.Name,
		Mode:        FinalMode, // We only support FinalMode now in final phase.
		HasDistinct: a.HasDistinct,
		RetTp:       a.RetTp,
	}
	switch a.Name {
	case ast.AggFuncAvg:
		args := make([]expression.Expression, 0, 2)
		args = append(args, &expression.Column{
			ColName: model.NewCIStr(fmt.Sprintf("avg_final_col_%d", ordinal[0])),
			Index:   ordinal[0],
			RetType: types.NewFieldType(mysql.TypeLonglong),
		})
		args = append(args, &expression.Column{
			ColName: model.NewCIStr(fmt.Sprintf("avg_final_col_%d", ordinal[1])),
			Index:   ordinal[1],
			RetType: a.RetTp,
		})
		finalAggDesc.Args = args
	default:
		args := make([]expression.Expression, 0, 1)
		args = append(args, &expression.Column{
			ColName: model.NewCIStr(fmt.Sprintf("%s_final_col_%d", a.Name, ordinal[0])),
			Index:   ordinal[0],
			RetType: a.RetTp,
		})
		finalAggDesc.Args = args
		if finalAggDesc.Name == ast.AggFuncGroupConcat {
			finalAggDesc.Args = append(finalAggDesc.Args, a.Args[len(a.Args)-1]) // separator
		}
	}
	return
}

// String implements the fmt.Stringer interface.
func (a *AggFuncDesc) String() string {
	buffer := bytes.NewBufferString(a.Name)
	buffer.WriteString("(")
	for i, arg := range a.Args {
		buffer.WriteString(arg.String())
		if i+1 != len(a.Args) {
			buffer.WriteString(", ")
		}
	}
	buffer.WriteString(")")
	return buffer.String()
}

// typeInfer infers the arguments and return types of an aggregation function.
func (a *AggFuncDesc) typeInfer(ctx sessionctx.Context) {
	switch a.Name {
	case ast.AggFuncCount:
		a.typeInfer4Count(ctx)
	case ast.AggFuncSum:
		a.typeInfer4Sum(ctx)
	case ast.AggFuncAvg:
		a.typeInfer4Avg(ctx)
	case ast.AggFuncGroupConcat:
		a.typeInfer4GroupConcat(ctx)
	case ast.AggFuncMax, ast.AggFuncMin, ast.AggFuncFirstRow:
		a.typeInfer4MaxMin(ctx)
	case ast.AggFuncBitAnd, ast.AggFuncBitOr, ast.AggFuncBitXor:
		a.typeInfer4BitFuncs(ctx)
	default:
		panic("unsupported agg function: " + a.Name)
	}
}

// EvalNullValueInOuterJoin gets the null value when the aggregation is upon an outer join,
// and the aggregation function's input is null.
// If there is no matching row for the inner table of an outer join,
// an aggregation function only involves constant and/or columns belongs to the inner table
// will be set to the null value.
// The input stands for the schema of Aggregation's child. If the function can't produce a null value, the second
// return value will be false.
// e.g.
// Table t with only one row:
// +-------+---------+---------+
// | Table | Field   | Type    |
// +-------+---------+---------+
// | t     | a       | int(11) |
// +-------+---------+---------+
// +------+
// | a    |
// +------+
// |    1 |
// +------+
//
// Table s which is empty:
// +-------+---------+---------+
// | Table | Field   | Type    |
// +-------+---------+---------+
// | s     | a       | int(11) |
// +-------+---------+---------+
//
// Query: `select t.a as `t.a`,  count(95), sum(95), avg(95), bit_or(95), bit_and(95), bit_or(95), max(95), min(95), s.a as `s.a`, avg(95) from t left join s on t.a = s.a;`
// +------+-----------+---------+---------+------------+-------------+------------+---------+---------+------+----------+
// | t.a  | count(95) | sum(95) | avg(95) | bit_or(95) | bit_and(95) | bit_or(95) | max(95) | min(95) | s.a  | avg(s.a) |
// +------+-----------+---------+---------+------------+-------------+------------+---------+---------+------+----------+
// |    1 |         1 |      95 | 95.0000 |         95 |          95 |         95 |      95 |      95 | NULL |     NULL |
// +------+-----------+---------+---------+------------+-------------+------------+---------+---------+------+----------+
func (a *AggFuncDesc) EvalNullValueInOuterJoin(ctx sessionctx.Context, schema *expression.Schema) (types.Datum, bool) {
	switch a.Name {
	case ast.AggFuncCount:
		return a.evalNullValueInOuterJoin4Count(ctx, schema)
	case ast.AggFuncSum, ast.AggFuncMax, ast.AggFuncMin,
		ast.AggFuncFirstRow:
		return a.evalNullValueInOuterJoin4Sum(ctx, schema)
	case ast.AggFuncAvg, ast.AggFuncGroupConcat:
		return types.Datum{}, false
	case ast.AggFuncBitAnd:
		return a.evalNullValueInOuterJoin4BitAnd(ctx, schema)
	case ast.AggFuncBitOr, ast.AggFuncBitXor:
		return a.evalNullValueInOuterJoin4BitOr(ctx, schema)
	default:
		panic("unsupported agg function")
	}
}

// GetDefaultValue gets the default value when the aggregation function's input is null.
// According to MySQL, default values of the aggregation function are listed as follows:
// e.g.
// Table t which is empty:
// +-------+---------+---------+
// | Table | Field   | Type    |
// +-------+---------+---------+
// | t     | a       | int(11) |
// +-------+---------+---------+
//
// Query: `select a, avg(a), sum(a), count(a), bit_xor(a), bit_or(a), bit_and(a), max(a), min(a), group_concat(a) from t;`
// +------+--------+--------+----------+------------+-----------+----------------------+--------+--------+-----------------+
// | a    | avg(a) | sum(a) | count(a) | bit_xor(a) | bit_or(a) | bit_and(a)           | max(a) | min(a) | group_concat(a) |
// +------+--------+--------+----------+------------+-----------+----------------------+--------+--------+-----------------+
// | NULL |   NULL |   NULL |        0 |          0 |         0 | 18446744073709551615 |   NULL |   NULL | NULL            |
// +------+--------+--------+----------+------------+-----------+----------------------+--------+--------+-----------------+
func (a *AggFuncDesc) GetDefaultValue() (v types.Datum) {
	switch a.Name {
	case ast.AggFuncCount, ast.AggFuncBitOr, ast.AggFuncBitXor:
		v = types.NewIntDatum(0)
	case ast.AggFuncFirstRow, ast.AggFuncAvg, ast.AggFuncSum, ast.AggFuncMax,
		ast.AggFuncMin, ast.AggFuncGroupConcat:
		v = types.Datum{}
	case ast.AggFuncBitAnd:
		v = types.NewUintDatum(uint64(math.MaxUint64))
	}
	return
}

// GetAggFunc gets an evaluator according to the aggregation function signature.
func (a *AggFuncDesc) GetAggFunc(ctx sessionctx.Context) Aggregation {
	aggFunc := aggFunction{AggFuncDesc: a}
	switch a.Name {
	case ast.AggFuncSum:
		return &sumFunction{aggFunction: aggFunc}
	case ast.AggFuncCount:
		return &countFunction{aggFunction: aggFunc}
	case ast.AggFuncAvg:
		return &avgFunction{aggFunction: aggFunc}
	case ast.AggFuncGroupConcat:
		var s string
		var err error
		var maxLen uint64
		s, err = variable.GetSessionSystemVar(ctx.GetSessionVars(), variable.GroupConcatMaxLen)
		if err != nil {
			panic(fmt.Sprintf("Error happened when GetAggFunc: no system variable named '%s'", variable.GroupConcatMaxLen))
		}
		maxLen, err = strconv.ParseUint(s, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("Error happened when GetAggFunc: illegal value for system variable named '%s'", variable.GroupConcatMaxLen))
		}
		return &concatFunction{aggFunction: aggFunc, maxLen: maxLen}
	case ast.AggFuncMax:
		return &maxMinFunction{aggFunction: aggFunc, isMax: true}
	case ast.AggFuncMin:
		return &maxMinFunction{aggFunction: aggFunc, isMax: false}
	case ast.AggFuncFirstRow:
		return &firstRowFunction{aggFunction: aggFunc}
	case ast.AggFuncBitOr:
		return &bitOrFunction{aggFunction: aggFunc}
	case ast.AggFuncBitXor:
		return &bitXorFunction{aggFunction: aggFunc}
	case ast.AggFuncBitAnd:
		return &bitAndFunction{aggFunction: aggFunc}
	default:
		panic("unsupported agg function")
	}
}

func (a *AggFuncDesc) typeInfer4Count(ctx sessionctx.Context) {
	a.RetTp = types.NewFieldType(mysql.TypeLonglong)
	a.RetTp.Flen = 21
	types.SetBinChsClnFlag(a.RetTp)
}

// typeInfer4Sum should returns a "decimal", otherwise it returns a "double".
// Because child returns integer or decimal type.
func (a *AggFuncDesc) typeInfer4Sum(ctx sessionctx.Context) {
	switch a.Args[0].GetType().Tp {
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong:
		a.RetTp = types.NewFieldType(mysql.TypeNewDecimal)
		a.RetTp.Flen, a.RetTp.Decimal = mysql.MaxDecimalWidth, 0
	case mysql.TypeNewDecimal:
		a.RetTp = types.NewFieldType(mysql.TypeNewDecimal)
		a.RetTp.Flen, a.RetTp.Decimal = mysql.MaxDecimalWidth, a.Args[0].GetType().Decimal
		if a.RetTp.Decimal < 0 || a.RetTp.Decimal > mysql.MaxDecimalScale {
			a.RetTp.Decimal = mysql.MaxDecimalScale
		}
	case mysql.TypeDouble, mysql.TypeFloat:
		a.RetTp = types.NewFieldType(mysql.TypeDouble)
		a.RetTp.Flen, a.RetTp.Decimal = mysql.MaxRealWidth, a.Args[0].GetType().Decimal
	default:
		a.RetTp = types.NewFieldType(mysql.TypeDouble)
		a.RetTp.Flen, a.RetTp.Decimal = mysql.MaxRealWidth, types.UnspecifiedLength
	}
	types.SetBinChsClnFlag(a.RetTp)
}

// typeInfer4Avg should returns a "decimal", otherwise it returns a "double".
// Because child returns integer or decimal type.
func (a *AggFuncDesc) typeInfer4Avg(ctx sessionctx.Context) {
	switch a.Args[0].GetType().Tp {
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong, mysql.TypeNewDecimal:
		a.RetTp = types.NewFieldType(mysql.TypeNewDecimal)
		if a.Args[0].GetType().Decimal < 0 {
			a.RetTp.Decimal = mysql.MaxDecimalScale
		} else {
			a.RetTp.Decimal = mathutil.Min(a.Args[0].GetType().Decimal+types.DivFracIncr, mysql.MaxDecimalScale)
		}
		a.RetTp.Flen = mysql.MaxDecimalWidth
	case mysql.TypeDouble, mysql.TypeFloat:
		a.RetTp = types.NewFieldType(mysql.TypeDouble)
		a.RetTp.Flen, a.RetTp.Decimal = mysql.MaxRealWidth, a.Args[0].GetType().Decimal
	default:
		a.RetTp = types.NewFieldType(mysql.TypeDouble)
		a.RetTp.Flen, a.RetTp.Decimal = mysql.MaxRealWidth, types.UnspecifiedLength
	}
	types.SetBinChsClnFlag(a.RetTp)
}

func (a *AggFuncDesc) typeInfer4GroupConcat(ctx sessionctx.Context) {
	a.RetTp = types.NewFieldType(mysql.TypeVarString)
	a.RetTp.Charset, a.RetTp.Collate = charset.GetDefaultCharsetAndCollate()

	a.RetTp.Flen, a.RetTp.Decimal = mysql.MaxBlobWidth, 0
	// TODO: a.Args[i] = expression.WrapWithCastAsString(ctx, a.Args[i])
}

func (a *AggFuncDesc) typeInfer4MaxMin(ctx sessionctx.Context) {
	_, argIsScalaFunc := a.Args[0].(*expression.ScalarFunction)
	if argIsScalaFunc && a.Args[0].GetType().Tp == mysql.TypeFloat {
		// For scalar function, the result of "float32" is set to the "float64"
		// field in the "Datum". If we do not wrap a cast-as-double function on a.Args[0],
		// error would happen when extracting the evaluation of a.Args[0] to a ProjectionExec.
		tp := types.NewFieldType(mysql.TypeDouble)
		tp.Flen, tp.Decimal = mysql.MaxRealWidth, types.UnspecifiedLength
		types.SetBinChsClnFlag(tp)
		a.Args[0] = expression.BuildCastFunction(ctx, a.Args[0], tp)
	}
	a.RetTp = a.Args[0].GetType()
	if a.RetTp.Tp == mysql.TypeEnum || a.RetTp.Tp == mysql.TypeSet {
		a.RetTp = &types.FieldType{Tp: mysql.TypeString, Flen: mysql.MaxFieldCharLength}
	}
}

func (a *AggFuncDesc) typeInfer4BitFuncs(ctx sessionctx.Context) {
	a.RetTp = types.NewFieldType(mysql.TypeLonglong)
	a.RetTp.Flen = 21
	types.SetBinChsClnFlag(a.RetTp)
	a.RetTp.Flag |= mysql.UnsignedFlag | mysql.NotNullFlag
	// TODO: a.Args[0] = expression.WrapWithCastAsInt(ctx, a.Args[0])
}

func (a *AggFuncDesc) evalNullValueInOuterJoin4Count(ctx sessionctx.Context, schema *expression.Schema) (types.Datum, bool) {
	for _, arg := range a.Args {
		result := expression.EvaluateExprWithNull(ctx, schema, arg)
		con, ok := result.(*expression.Constant)
		if !ok || con.Value.IsNull() {
			return types.Datum{}, ok
		}
	}
	return types.NewDatum(1), true
}

func (a *AggFuncDesc) evalNullValueInOuterJoin4Sum(ctx sessionctx.Context, schema *expression.Schema) (types.Datum, bool) {
	result := expression.EvaluateExprWithNull(ctx, schema, a.Args[0])
	con, ok := result.(*expression.Constant)
	if !ok || con.Value.IsNull() {
		return types.Datum{}, ok
	}
	return con.Value, true
}

func (a *AggFuncDesc) evalNullValueInOuterJoin4BitAnd(ctx sessionctx.Context, schema *expression.Schema) (types.Datum, bool) {
	result := expression.EvaluateExprWithNull(ctx, schema, a.Args[0])
	con, ok := result.(*expression.Constant)
	if !ok || con.Value.IsNull() {
		return types.NewDatum(uint64(math.MaxUint64)), true
	}
	return con.Value, true
}

func (a *AggFuncDesc) evalNullValueInOuterJoin4BitOr(ctx sessionctx.Context, schema *expression.Schema) (types.Datum, bool) {
	result := expression.EvaluateExprWithNull(ctx, schema, a.Args[0])
	con, ok := result.(*expression.Constant)
	if !ok || con.Value.IsNull() {
		return types.NewDatum(0), true
	}
	return con.Value, true
}
