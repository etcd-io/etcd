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
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
)

// KeyInfo stores the columns of one unique key or primary key.
type KeyInfo []*Column

// Clone copies the entire UniqueKey.
func (ki KeyInfo) Clone() KeyInfo {
	result := make([]*Column, 0, len(ki))
	for _, col := range ki {
		result = append(result, col.Clone().(*Column))
	}
	return result
}

// Schema stands for the row schema and unique key information get from input.
type Schema struct {
	Columns []*Column
	Keys    []KeyInfo
	// TblID2Handle stores the tables' handle column information if we need handle in execution phase.
	TblID2Handle map[int64][]*Column
}

// String implements fmt.Stringer interface.
func (s *Schema) String() string {
	colStrs := make([]string, 0, len(s.Columns))
	for _, col := range s.Columns {
		colStrs = append(colStrs, col.String())
	}
	ukStrs := make([]string, 0, len(s.Keys))
	for _, key := range s.Keys {
		ukColStrs := make([]string, 0, len(key))
		for _, col := range key {
			ukColStrs = append(ukColStrs, col.String())
		}
		ukStrs = append(ukStrs, "["+strings.Join(ukColStrs, ",")+"]")
	}
	return "Column: [" + strings.Join(colStrs, ",") + "] Unique key: [" + strings.Join(ukStrs, ",") + "]"
}

// Clone copies the total schema.
func (s *Schema) Clone() *Schema {
	cols := make([]*Column, 0, s.Len())
	keys := make([]KeyInfo, 0, len(s.Keys))
	for _, col := range s.Columns {
		cols = append(cols, col.Clone().(*Column))
	}
	for _, key := range s.Keys {
		keys = append(keys, key.Clone())
	}
	schema := NewSchema(cols...)
	schema.SetUniqueKeys(keys)
	for id, cols := range s.TblID2Handle {
		schema.TblID2Handle[id] = make([]*Column, 0, len(cols))
		for _, col := range cols {
			var inColumns = false
			for i, colInColumns := range s.Columns {
				if col == colInColumns {
					schema.TblID2Handle[id] = append(schema.TblID2Handle[id], schema.Columns[i])
					inColumns = true
					break
				}
			}
			if !inColumns {
				schema.TblID2Handle[id] = append(schema.TblID2Handle[id], col.Clone().(*Column))
			}
		}
	}
	return schema
}

// ExprFromSchema checks if all columns of this expression are from the same schema.
func ExprFromSchema(expr Expression, schema *Schema) bool {
	cols := ExtractColumns(expr)
	return len(schema.ColumnsIndices(cols)) > 0
}

// FindColumn finds an Column from schema for a ast.ColumnName. It compares the db/table/column names.
// If there are more than one result, it will raise ambiguous error.
func (s *Schema) FindColumn(astCol *ast.ColumnName) (*Column, error) {
	col, _, err := s.FindColumnAndIndex(astCol)
	return col, errors.Trace(err)
}

// FindColumnAndIndex finds an Column and its index from schema for a ast.ColumnName.
// It compares the db/table/column names. If there are more than one result, raise ambiguous error.
func (s *Schema) FindColumnAndIndex(astCol *ast.ColumnName) (*Column, int, error) {
	dbName, tblName, colName := astCol.Schema, astCol.Table, astCol.Name
	idx := -1
	for i, col := range s.Columns {
		if (dbName.L == "" || dbName.L == col.DBName.L) &&
			(tblName.L == "" || tblName.L == col.TblName.L) &&
			(colName.L == col.ColName.L) {
			if idx == -1 {
				idx = i
			} else {
				// For query like:
				// create table t1(a int); create table t2(d int);
				// select 1 from t1, t2 where 1 = (select d from t2 where a > 1) and d = 1;
				// we will get an Apply operator whose schema is [test.t1.a, test.t2.d, test.t2.d],
				// we check whether the column of the schema comes from a subquery to avoid
				// causing the ambiguous error when resolve the column `d` in the Selection.
				if !col.IsAggOrSubq {
					return nil, -1, errors.Errorf("Column %s is ambiguous", col.String())
				}
			}
		}
	}
	if idx == -1 {
		return nil, idx, nil
	}
	return s.Columns[idx], idx, nil
}

// RetrieveColumn retrieves column in expression from the columns in schema.
func (s *Schema) RetrieveColumn(col *Column) *Column {
	index := s.ColumnIndex(col)
	if index != -1 {
		return s.Columns[index]
	}
	return nil
}

// IsUniqueKey checks if this column is a unique key.
func (s *Schema) IsUniqueKey(col *Column) bool {
	for _, key := range s.Keys {
		if len(key) == 1 && key[0].Equal(nil, col) {
			return true
		}
	}
	return false
}

// ColumnIndex finds the index for a column.
func (s *Schema) ColumnIndex(col *Column) int {
	for i, c := range s.Columns {
		if c.UniqueID == col.UniqueID {
			return i
		}
	}
	return -1
}

// FindColumnByName finds a column by its name.
func (s *Schema) FindColumnByName(name string) *Column {
	for _, col := range s.Columns {
		if col.ColName.L == name {
			return col
		}
	}
	return nil
}

// Contains checks if the schema contains the column.
func (s *Schema) Contains(col *Column) bool {
	return s.ColumnIndex(col) != -1
}

// Len returns the number of columns in schema.
func (s *Schema) Len() int {
	return len(s.Columns)
}

// Append append new column to the columns stored in schema.
func (s *Schema) Append(col ...*Column) {
	s.Columns = append(s.Columns, col...)
}

// SetUniqueKeys will set the value of Schema.Keys.
func (s *Schema) SetUniqueKeys(keys []KeyInfo) {
	s.Keys = keys
}

// ColumnsIndices will return a slice which contains the position of each column in schema.
// If there is one column that doesn't match, nil will be returned.
func (s *Schema) ColumnsIndices(cols []*Column) (ret []int) {
	ret = make([]int, 0, len(cols))
	for _, col := range cols {
		pos := s.ColumnIndex(col)
		if pos != -1 {
			ret = append(ret, pos)
		} else {
			return nil
		}
	}
	return
}

// ColumnsByIndices returns columns by multiple offsets.
// Callers should guarantee that all the offsets provided should be valid, which means offset should:
// 1. not smaller than 0, and
// 2. not exceed len(s.Columns)
func (s *Schema) ColumnsByIndices(offsets []int) []*Column {
	cols := make([]*Column, 0, len(offsets))
	for _, offset := range offsets {
		cols = append(cols, s.Columns[offset])
	}
	return cols
}

// MergeSchema will merge two schema into one schema. We shouldn't need to consider unique keys.
// That will be processed in build_key_info.go.
func MergeSchema(lSchema, rSchema *Schema) *Schema {
	if lSchema == nil && rSchema == nil {
		return nil
	}
	if lSchema == nil {
		return rSchema.Clone()
	}
	if rSchema == nil {
		return lSchema.Clone()
	}
	tmpL := lSchema.Clone()
	tmpR := rSchema.Clone()
	ret := NewSchema(append(tmpL.Columns, tmpR.Columns...)...)
	ret.TblID2Handle = tmpL.TblID2Handle
	for id, cols := range tmpR.TblID2Handle {
		if _, ok := ret.TblID2Handle[id]; ok {
			ret.TblID2Handle[id] = append(ret.TblID2Handle[id], cols...)
		} else {
			ret.TblID2Handle[id] = cols
		}
	}
	return ret
}

// NewSchema returns a schema made by its parameter.
func NewSchema(cols ...*Column) *Schema {
	return &Schema{Columns: cols, TblID2Handle: make(map[int64][]*Column)}
}
