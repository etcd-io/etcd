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
	"fmt"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/codec"
	log "github.com/sirupsen/logrus"
)

// CorrelatedColumn stands for a column in a correlated sub query.
type CorrelatedColumn struct {
	Column

	Data *types.Datum
}

// Clone implements Expression interface.
func (col *CorrelatedColumn) Clone() Expression {
	return col
}

// Eval implements Expression interface.
func (col *CorrelatedColumn) Eval(row chunk.Row) (types.Datum, error) {
	return *col.Data, nil
}

// EvalInt returns int representation of CorrelatedColumn.
func (col *CorrelatedColumn) EvalInt(ctx sessionctx.Context, row chunk.Row) (int64, bool, error) {
	if col.Data.IsNull() {
		return 0, true, nil
	}
	if col.GetType().Hybrid() {
		res, err := col.Data.ToInt64(ctx.GetSessionVars().StmtCtx)
		return res, err != nil, errors.Trace(err)
	}
	return col.Data.GetInt64(), false, nil
}

// EvalReal returns real representation of CorrelatedColumn.
func (col *CorrelatedColumn) EvalReal(ctx sessionctx.Context, row chunk.Row) (float64, bool, error) {
	if col.Data.IsNull() {
		return 0, true, nil
	}
	return col.Data.GetFloat64(), false, nil
}

// EvalString returns string representation of CorrelatedColumn.
func (col *CorrelatedColumn) EvalString(ctx sessionctx.Context, row chunk.Row) (string, bool, error) {
	if col.Data.IsNull() {
		return "", true, nil
	}
	res, err := col.Data.ToString()
	resLen := len([]rune(res))
	if resLen < col.RetType.Flen && ctx.GetSessionVars().StmtCtx.PadCharToFullLength {
		res = res + strings.Repeat(" ", col.RetType.Flen-resLen)
	}
	return res, err != nil, errors.Trace(err)
}

// EvalDecimal returns decimal representation of CorrelatedColumn.
func (col *CorrelatedColumn) EvalDecimal(ctx sessionctx.Context, row chunk.Row) (*types.MyDecimal, bool, error) {
	if col.Data.IsNull() {
		return nil, true, nil
	}
	return col.Data.GetMysqlDecimal(), false, nil
}

// EvalTime returns DATE/DATETIME/TIMESTAMP representation of CorrelatedColumn.
func (col *CorrelatedColumn) EvalTime(ctx sessionctx.Context, row chunk.Row) (types.Time, bool, error) {
	if col.Data.IsNull() {
		return types.Time{}, true, nil
	}
	return col.Data.GetMysqlTime(), false, nil
}

// EvalDuration returns Duration representation of CorrelatedColumn.
func (col *CorrelatedColumn) EvalDuration(ctx sessionctx.Context, row chunk.Row) (types.Duration, bool, error) {
	if col.Data.IsNull() {
		return types.Duration{}, true, nil
	}
	return col.Data.GetMysqlDuration(), false, nil
}

// EvalJSON returns JSON representation of CorrelatedColumn.
func (col *CorrelatedColumn) EvalJSON(ctx sessionctx.Context, row chunk.Row) (json.BinaryJSON, bool, error) {
	if col.Data.IsNull() {
		return json.BinaryJSON{}, true, nil
	}
	return col.Data.GetMysqlJSON(), false, nil
}

// Equal implements Expression interface.
func (col *CorrelatedColumn) Equal(ctx sessionctx.Context, expr Expression) bool {
	if cc, ok := expr.(*CorrelatedColumn); ok {
		return col.Column.Equal(ctx, &cc.Column)
	}
	return false
}

// IsCorrelated implements Expression interface.
func (col *CorrelatedColumn) IsCorrelated() bool {
	return true
}

// Decorrelate implements Expression interface.
func (col *CorrelatedColumn) Decorrelate(schema *Schema) Expression {
	if !schema.Contains(&col.Column) {
		return col
	}
	return &col.Column
}

// ResolveIndices implements Expression interface.
func (col *CorrelatedColumn) ResolveIndices(_ *Schema) Expression {
	return col
}

func (col *CorrelatedColumn) resolveIndices(_ *Schema) {
}

// Column represents a column.
type Column struct {
	OrigColName model.CIStr
	ColName     model.CIStr
	DBName      model.CIStr
	OrigTblName model.CIStr
	TblName     model.CIStr
	RetType     *types.FieldType
	// ID is used to specify whether this column is ExtraHandleColumn or to access histogram.
	// We'll try to remove it in the future.
	ID int64
	// UniqueID is the unique id of this column.
	UniqueID int64
	// IsAggOrSubq means if this column is referenced to a Aggregation column or a Subquery column.
	// If so, this column's name will be the plain sql text.
	IsAggOrSubq bool

	// Index is used for execution, to tell the column's position in the given row.
	Index int

	hashcode []byte
}

// Equal implements Expression interface.
func (col *Column) Equal(_ sessionctx.Context, expr Expression) bool {
	if newCol, ok := expr.(*Column); ok {
		return newCol.UniqueID == col.UniqueID
	}
	return false
}

// String implements Stringer interface.
func (col *Column) String() string {
	result := col.ColName.L
	if col.TblName.L != "" {
		result = col.TblName.L + "." + result
	}
	if col.DBName.L != "" {
		result = col.DBName.L + "." + result
	}
	return result
}

// MarshalJSON implements json.Marshaler interface.
func (col *Column) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", col)), nil
}

// GetType implements Expression interface.
func (col *Column) GetType() *types.FieldType {
	return col.RetType
}

// Eval implements Expression interface.
func (col *Column) Eval(row chunk.Row) (types.Datum, error) {
	return row.GetDatum(col.Index, col.RetType), nil
}

// EvalInt returns int representation of Column.
func (col *Column) EvalInt(ctx sessionctx.Context, row chunk.Row) (int64, bool, error) {
	if col.GetType().Hybrid() {
		val := row.GetDatum(col.Index, col.RetType)
		if val.IsNull() {
			return 0, true, nil
		}
		res, err := val.ToInt64(ctx.GetSessionVars().StmtCtx)
		return res, err != nil, errors.Trace(err)
	}
	if row.IsNull(col.Index) {
		return 0, true, nil
	}
	return row.GetInt64(col.Index), false, nil
}

// EvalReal returns real representation of Column.
func (col *Column) EvalReal(ctx sessionctx.Context, row chunk.Row) (float64, bool, error) {
	if row.IsNull(col.Index) {
		return 0, true, nil
	}
	if col.GetType().Tp == mysql.TypeFloat {
		return float64(row.GetFloat32(col.Index)), false, nil
	}
	return row.GetFloat64(col.Index), false, nil
}

// EvalString returns string representation of Column.
func (col *Column) EvalString(ctx sessionctx.Context, row chunk.Row) (string, bool, error) {
	if row.IsNull(col.Index) {
		return "", true, nil
	}
	if col.GetType().Hybrid() {
		val := row.GetDatum(col.Index, col.RetType)
		if val.IsNull() {
			return "", true, nil
		}
		res, err := val.ToString()
		resLen := len([]rune(res))
		if ctx.GetSessionVars().StmtCtx.PadCharToFullLength && col.GetType().Tp == mysql.TypeString && resLen < col.RetType.Flen {
			res = res + strings.Repeat(" ", col.RetType.Flen-resLen)
		}
		return res, err != nil, errors.Trace(err)
	}
	val := row.GetString(col.Index)
	if ctx.GetSessionVars().StmtCtx.PadCharToFullLength && col.GetType().Tp == mysql.TypeString {
		valLen := len([]rune(val))
		if valLen < col.RetType.Flen {
			val = val + strings.Repeat(" ", col.RetType.Flen-valLen)
		}
	}
	return val, false, nil
}

// EvalDecimal returns decimal representation of Column.
func (col *Column) EvalDecimal(ctx sessionctx.Context, row chunk.Row) (*types.MyDecimal, bool, error) {
	if row.IsNull(col.Index) {
		return nil, true, nil
	}
	return row.GetMyDecimal(col.Index), false, nil
}

// EvalTime returns DATE/DATETIME/TIMESTAMP representation of Column.
func (col *Column) EvalTime(ctx sessionctx.Context, row chunk.Row) (types.Time, bool, error) {
	if row.IsNull(col.Index) {
		return types.Time{}, true, nil
	}
	return row.GetTime(col.Index), false, nil
}

// EvalDuration returns Duration representation of Column.
func (col *Column) EvalDuration(ctx sessionctx.Context, row chunk.Row) (types.Duration, bool, error) {
	if row.IsNull(col.Index) {
		return types.Duration{}, true, nil
	}
	duration := row.GetDuration(col.Index, col.RetType.Decimal)
	return duration, false, nil
}

// EvalJSON returns JSON representation of Column.
func (col *Column) EvalJSON(ctx sessionctx.Context, row chunk.Row) (json.BinaryJSON, bool, error) {
	if row.IsNull(col.Index) {
		return json.BinaryJSON{}, true, nil
	}
	return row.GetJSON(col.Index), false, nil
}

// Clone implements Expression interface.
func (col *Column) Clone() Expression {
	newCol := *col
	return &newCol
}

// IsCorrelated implements Expression interface.
func (col *Column) IsCorrelated() bool {
	return false
}

// Decorrelate implements Expression interface.
func (col *Column) Decorrelate(_ *Schema) Expression {
	return col
}

// HashCode implements Expression interface.
func (col *Column) HashCode(_ *stmtctx.StatementContext) []byte {
	if len(col.hashcode) != 0 {
		return col.hashcode
	}
	col.hashcode = make([]byte, 0, 9)
	col.hashcode = append(col.hashcode, columnFlag)
	col.hashcode = codec.EncodeInt(col.hashcode, int64(col.UniqueID))
	return col.hashcode
}

// ResolveIndices implements Expression interface.
func (col *Column) ResolveIndices(schema *Schema) Expression {
	newCol := col.Clone()
	newCol.resolveIndices(schema)
	return newCol
}

func (col *Column) resolveIndices(schema *Schema) {
	col.Index = schema.ColumnIndex(col)
	if col.Index == -1 {
		log.Errorf("Can't find column %s in schema %s", col, schema)
	}
}

// Column2Exprs will transfer column slice to expression slice.
func Column2Exprs(cols []*Column) []Expression {
	result := make([]Expression, 0, len(cols))
	for _, col := range cols {
		result = append(result, col)
	}
	return result
}

// ColInfo2Col finds the corresponding column of the ColumnInfo in a column slice.
func ColInfo2Col(cols []*Column, col *model.ColumnInfo) *Column {
	for _, c := range cols {
		if c.ColName.L == col.Name.L {
			return c
		}
	}
	return nil
}

// indexCol2Col finds the corresponding column of the IndexColumn in a column slice.
func indexCol2Col(cols []*Column, col *model.IndexColumn) *Column {
	for _, c := range cols {
		if c.ColName.L == col.Name.L {
			return c
		}
	}
	return nil
}

// IndexInfo2Cols gets the corresponding []*Column of the indexInfo's []*IndexColumn,
// together with a []int containing their lengths.
// If this index has three IndexColumn that the 1st and 3rd IndexColumn has corresponding *Column,
// the return value will be only the 1st corresponding *Column and its length.
func IndexInfo2Cols(cols []*Column, index *model.IndexInfo) ([]*Column, []int) {
	retCols := make([]*Column, 0, len(index.Columns))
	lengths := make([]int, 0, len(index.Columns))
	for _, c := range index.Columns {
		col := indexCol2Col(cols, c)
		if col == nil {
			return retCols, lengths
		}
		retCols = append(retCols, col)
		if c.Length != types.UnspecifiedLength && c.Length == col.RetType.Flen {
			lengths = append(lengths, types.UnspecifiedLength)
		} else {
			lengths = append(lengths, c.Length)
		}
	}
	return retCols, lengths
}

// FindPrefixOfIndex will find columns in index by checking the unique id.
// So it will return at once no matching column is found.
func FindPrefixOfIndex(cols []*Column, idxColIDs []int64) []*Column {
	retCols := make([]*Column, 0, len(idxColIDs))
idLoop:
	for _, id := range idxColIDs {
		for _, col := range cols {
			if col.UniqueID == id {
				retCols = append(retCols, col)
				continue idLoop
			}
		}
		// If no matching column is found, just return.
		return retCols
	}
	return retCols
}
