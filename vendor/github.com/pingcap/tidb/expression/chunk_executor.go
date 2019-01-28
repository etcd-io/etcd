// Copyright 2017 PingCAP, Inc.
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
	"strconv"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
)

// Vectorizable checks whether a list of expressions can employ vectorized execution.
func Vectorizable(exprs []Expression) bool {
	for _, expr := range exprs {
		if HasGetSetVarFunc(expr) {
			return false
		}
	}
	return true
}

// HasGetSetVarFunc checks whether an expression contains SetVar/GetVar function.
func HasGetSetVarFunc(expr Expression) bool {
	scalaFunc, ok := expr.(*ScalarFunction)
	if !ok {
		return false
	}
	if scalaFunc.FuncName.L == ast.SetVar {
		return true
	}
	if scalaFunc.FuncName.L == ast.GetVar {
		return true
	}
	for _, arg := range scalaFunc.GetArgs() {
		if HasGetSetVarFunc(arg) {
			return true
		}
	}
	return false
}

// VectorizedExecute evaluates a list of expressions column by column and append their results to "output" Chunk.
func VectorizedExecute(ctx sessionctx.Context, exprs []Expression, iterator *chunk.Iterator4Chunk, output *chunk.Chunk) error {
	for colID, expr := range exprs {
		err := evalOneColumn(ctx, expr, iterator, output, colID)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func evalOneColumn(ctx sessionctx.Context, expr Expression, iterator *chunk.Iterator4Chunk, output *chunk.Chunk, colID int) (err error) {
	switch fieldType, evalType := expr.GetType(), expr.GetType().EvalType(); evalType {
	case types.ETInt:
		for row := iterator.Begin(); err == nil && row != iterator.End(); row = iterator.Next() {
			err = executeToInt(ctx, expr, fieldType, row, output, colID)
		}
	case types.ETReal:
		for row := iterator.Begin(); err == nil && row != iterator.End(); row = iterator.Next() {
			err = executeToReal(ctx, expr, fieldType, row, output, colID)
		}
	case types.ETDecimal:
		for row := iterator.Begin(); err == nil && row != iterator.End(); row = iterator.Next() {
			err = executeToDecimal(ctx, expr, fieldType, row, output, colID)
		}
	case types.ETDatetime, types.ETTimestamp:
		for row := iterator.Begin(); err == nil && row != iterator.End(); row = iterator.Next() {
			err = executeToDatetime(ctx, expr, fieldType, row, output, colID)
		}
	case types.ETDuration:
		for row := iterator.Begin(); err == nil && row != iterator.End(); row = iterator.Next() {
			err = executeToDuration(ctx, expr, fieldType, row, output, colID)
		}
	case types.ETJson:
		for row := iterator.Begin(); err == nil && row != iterator.End(); row = iterator.Next() {
			err = executeToJSON(ctx, expr, fieldType, row, output, colID)
		}
	case types.ETString:
		for row := iterator.Begin(); err == nil && row != iterator.End(); row = iterator.Next() {
			err = executeToString(ctx, expr, fieldType, row, output, colID)
		}
	}
	return errors.Trace(err)
}

func evalOneCell(ctx sessionctx.Context, expr Expression, row chunk.Row, output *chunk.Chunk, colID int) (err error) {
	switch fieldType, evalType := expr.GetType(), expr.GetType().EvalType(); evalType {
	case types.ETInt:
		err = executeToInt(ctx, expr, fieldType, row, output, colID)
	case types.ETReal:
		err = executeToReal(ctx, expr, fieldType, row, output, colID)
	case types.ETDecimal:
		err = executeToDecimal(ctx, expr, fieldType, row, output, colID)
	case types.ETDatetime, types.ETTimestamp:
		err = executeToDatetime(ctx, expr, fieldType, row, output, colID)
	case types.ETDuration:
		err = executeToDuration(ctx, expr, fieldType, row, output, colID)
	case types.ETJson:
		err = executeToJSON(ctx, expr, fieldType, row, output, colID)
	case types.ETString:
		err = executeToString(ctx, expr, fieldType, row, output, colID)
	}
	return errors.Trace(err)
}

func executeToInt(ctx sessionctx.Context, expr Expression, fieldType *types.FieldType, row chunk.Row, output *chunk.Chunk, colID int) error {
	res, isNull, err := expr.EvalInt(ctx, row)
	if err != nil {
		return errors.Trace(err)
	}
	if isNull {
		output.AppendNull(colID)
		return nil
	}
	if fieldType.Tp == mysql.TypeBit {
		output.AppendBytes(colID, strconv.AppendUint(make([]byte, 0, 8), uint64(res), 10))
		return nil
	}
	if mysql.HasUnsignedFlag(fieldType.Flag) {
		output.AppendUint64(colID, uint64(res))
		return nil
	}
	output.AppendInt64(colID, res)
	return nil
}

func executeToReal(ctx sessionctx.Context, expr Expression, fieldType *types.FieldType, row chunk.Row, output *chunk.Chunk, colID int) error {
	res, isNull, err := expr.EvalReal(ctx, row)
	if err != nil {
		return errors.Trace(err)
	}
	if isNull {
		output.AppendNull(colID)
		return nil
	}
	if fieldType.Tp == mysql.TypeFloat {
		output.AppendFloat32(colID, float32(res))
		return nil
	}
	output.AppendFloat64(colID, res)
	return nil
}

func executeToDecimal(ctx sessionctx.Context, expr Expression, fieldType *types.FieldType, row chunk.Row, output *chunk.Chunk, colID int) error {
	res, isNull, err := expr.EvalDecimal(ctx, row)
	if err != nil {
		return errors.Trace(err)
	}
	if isNull {
		output.AppendNull(colID)
		return nil
	}
	output.AppendMyDecimal(colID, res)
	return nil
}

func executeToDatetime(ctx sessionctx.Context, expr Expression, fieldType *types.FieldType, row chunk.Row, output *chunk.Chunk, colID int) error {
	res, isNull, err := expr.EvalTime(ctx, row)
	if err != nil {
		return errors.Trace(err)
	}
	if isNull {
		output.AppendNull(colID)
	} else {
		output.AppendTime(colID, res)
	}
	return nil
}

func executeToDuration(ctx sessionctx.Context, expr Expression, fieldType *types.FieldType, row chunk.Row, output *chunk.Chunk, colID int) error {
	res, isNull, err := expr.EvalDuration(ctx, row)
	if err != nil {
		return errors.Trace(err)
	}
	if isNull {
		output.AppendNull(colID)
	} else {
		output.AppendDuration(colID, res)
	}
	return nil
}

func executeToJSON(ctx sessionctx.Context, expr Expression, fieldType *types.FieldType, row chunk.Row, output *chunk.Chunk, colID int) error {
	res, isNull, err := expr.EvalJSON(ctx, row)
	if err != nil {
		return errors.Trace(err)
	}
	if isNull {
		output.AppendNull(colID)
	} else {
		output.AppendJSON(colID, res)
	}
	return nil
}

func executeToString(ctx sessionctx.Context, expr Expression, fieldType *types.FieldType, row chunk.Row, output *chunk.Chunk, colID int) error {
	res, isNull, err := expr.EvalString(ctx, row)
	if err != nil {
		return errors.Trace(err)
	}
	if isNull {
		output.AppendNull(colID)
	} else if fieldType.Tp == mysql.TypeEnum {
		val := types.Enum{Value: uint64(0), Name: res}
		output.AppendEnum(colID, val)
	} else if fieldType.Tp == mysql.TypeSet {
		val := types.Set{Value: uint64(0), Name: res}
		output.AppendSet(colID, val)
	} else {
		output.AppendString(colID, res)
	}
	return nil
}

// VectorizedFilter applies a list of filters to a Chunk and
// returns a bool slice, which indicates whether a row is passed the filters.
// Filters is executed vectorized.
func VectorizedFilter(ctx sessionctx.Context, filters []Expression, iterator *chunk.Iterator4Chunk, selected []bool) ([]bool, error) {
	selected = selected[:0]
	for i, numRows := 0, iterator.Len(); i < numRows; i++ {
		selected = append(selected, true)
	}
	for _, filter := range filters {
		isIntType := true
		if filter.GetType().EvalType() != types.ETInt {
			isIntType = false
		}
		for row := iterator.Begin(); row != iterator.End(); row = iterator.Next() {
			if !selected[row.Idx()] {
				continue
			}
			if isIntType {
				filterResult, isNull, err := filter.EvalInt(ctx, row)
				if err != nil {
					return nil, errors.Trace(err)
				}
				selected[row.Idx()] = selected[row.Idx()] && !isNull && (filterResult != 0)
			} else {
				// TODO: should rewrite the filter to `cast(expr as SIGNED) != 0` and always use `EvalInt`.
				bVal, err := EvalBool(ctx, []Expression{filter}, row)
				if err != nil {
					return nil, errors.Trace(err)
				}
				selected[row.Idx()] = selected[row.Idx()] && bVal
			}
		}
	}
	return selected, nil
}
