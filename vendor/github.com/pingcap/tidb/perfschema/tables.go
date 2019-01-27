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

package perfschema

import (
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/types"
)

// perfSchemaTable stands for the fake table all its data is in the memory.
type perfSchemaTable struct {
	meta *model.TableInfo
	cols []*table.Column
}

// createPerfSchemaTable creates all perfSchemaTables
func createPerfSchemaTable(meta *model.TableInfo) *perfSchemaTable {
	columns := make([]*table.Column, 0, len(meta.Columns))
	for _, colInfo := range meta.Columns {
		col := table.ToColumn(colInfo)
		columns = append(columns, col)
	}
	t := &perfSchemaTable{
		meta: meta,
		cols: columns,
	}
	return t
}

// IterRecords implements table.Table Type interface.
func (vt *perfSchemaTable) IterRecords(ctx sessionctx.Context, startKey kv.Key, cols []*table.Column,
	fn table.RecordIterFunc) error {
	if len(startKey) != 0 {
		return table.ErrUnsupportedOp
	}
	return nil
}

// RowWithCols implements table.Table Type interface.
func (vt *perfSchemaTable) RowWithCols(ctx sessionctx.Context, h int64, cols []*table.Column) ([]types.Datum, error) {
	return nil, table.ErrUnsupportedOp
}

// Row implements table.Table Type interface.
func (vt *perfSchemaTable) Row(ctx sessionctx.Context, h int64) ([]types.Datum, error) {
	return nil, table.ErrUnsupportedOp
}

// Cols implements table.Table Type interface.
func (vt *perfSchemaTable) Cols() []*table.Column {
	return vt.cols
}

// WritableCols implements table.Table Type interface.
func (vt *perfSchemaTable) WritableCols() []*table.Column {
	return vt.cols
}

// Indices implements table.Table Type interface.
func (vt *perfSchemaTable) Indices() []table.Index {
	return nil
}

// WritableIndices implements table.Table Type interface.
func (vt *perfSchemaTable) WritableIndices() []table.Index {
	return nil
}

// DeletableIndices implements table.Table Type interface.
func (vt *perfSchemaTable) DeletableIndices() []table.Index {
	return nil
}

// RecordPrefix implements table.Table Type interface.
func (vt *perfSchemaTable) RecordPrefix() kv.Key {
	return nil
}

// IndexPrefix implements table.Table Type interface.
func (vt *perfSchemaTable) IndexPrefix() kv.Key {
	return nil
}

// FirstKey implements table.Table Type interface.
func (vt *perfSchemaTable) FirstKey() kv.Key {
	return nil
}

// RecordKey implements table.Table Type interface.
func (vt *perfSchemaTable) RecordKey(h int64) kv.Key {
	return nil
}

// AddRecord implements table.Table Type interface.
func (vt *perfSchemaTable) AddRecord(ctx sessionctx.Context, r []types.Datum, skipHandleCheck bool) (recordID int64, err error) {
	return 0, table.ErrUnsupportedOp
}

// RemoveRecord implements table.Table Type interface.
func (vt *perfSchemaTable) RemoveRecord(ctx sessionctx.Context, h int64, r []types.Datum) error {
	return table.ErrUnsupportedOp
}

// UpdateRecord implements table.Table Type interface.
func (vt *perfSchemaTable) UpdateRecord(ctx sessionctx.Context, h int64, oldData, newData []types.Datum, touched []bool) error {
	return table.ErrUnsupportedOp
}

// AllocAutoID implements table.Table Type interface.
func (vt *perfSchemaTable) AllocAutoID(ctx sessionctx.Context) (int64, error) {
	return 0, table.ErrUnsupportedOp
}

// Allocator implements table.Table Type interface.
func (vt *perfSchemaTable) Allocator(ctx sessionctx.Context) autoid.Allocator {
	return nil
}

// RebaseAutoID implements table.Table Type interface.
func (vt *perfSchemaTable) RebaseAutoID(ctx sessionctx.Context, newBase int64, isSetStep bool) error {
	return table.ErrUnsupportedOp
}

// Meta implements table.Table Type interface.
func (vt *perfSchemaTable) Meta() *model.TableInfo {
	return vt.meta
}

// GetID implements table.Table GetID interface.
func (vt *perfSchemaTable) GetPhysicalID() int64 {
	return vt.meta.ID
}

// Seek implements table.Table Type interface.
func (vt *perfSchemaTable) Seek(ctx sessionctx.Context, h int64) (int64, bool, error) {
	return 0, false, table.ErrUnsupportedOp
}

// Type implements table.Table Type interface.
func (vt *perfSchemaTable) Type() table.Type {
	return table.VirtualTable
}
