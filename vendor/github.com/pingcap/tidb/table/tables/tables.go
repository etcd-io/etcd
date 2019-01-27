// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

// Copyright 2015 PingCAP, Inc.
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

package tables

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/owner"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/kvcache"
	binlog "github.com/pingcap/tipb/go-binlog"
	log "github.com/sirupsen/logrus"
	"github.com/spaolacci/murmur3"
	"golang.org/x/net/context"
)

// tableCommon is shared by both Table and partition.
type tableCommon struct {
	tableID int64
	// physicalTableID is a unique int64 to identify a physical table.
	physicalTableID int64
	Columns         []*table.Column
	publicColumns   []*table.Column
	writableColumns []*table.Column
	writableIndices []table.Index
	indices         []table.Index
	meta            *model.TableInfo
	alloc           autoid.Allocator

	// recordPrefix and indexPrefix are generated using physicalTableID.
	recordPrefix kv.Key
	indexPrefix  kv.Key
}

// Table implements table.Table interface.
type Table struct {
	tableCommon
}

var _ table.Table = &Table{}

// MockTableFromMeta only serves for test.
func MockTableFromMeta(tblInfo *model.TableInfo) table.Table {
	columns := make([]*table.Column, 0, len(tblInfo.Columns))
	for _, colInfo := range tblInfo.Columns {
		col := table.ToColumn(colInfo)
		columns = append(columns, col)
	}

	var t Table
	initTableCommon(&t.tableCommon, tblInfo, tblInfo.ID, columns, nil)
	if tblInfo.GetPartitionInfo() == nil {
		if err := initTableIndices(&t.tableCommon); err != nil {
			return nil
		}
		return &t
	}

	ret, err := newPartitionedTable(&t, tblInfo)
	if err != nil {
		return nil
	}
	return ret
}

// TableFromMeta creates a Table instance from model.TableInfo.
func TableFromMeta(alloc autoid.Allocator, tblInfo *model.TableInfo) (table.Table, error) {
	if tblInfo.State == model.StateNone {
		return nil, table.ErrTableStateCantNone.GenWithStack("table %s can't be in none state", tblInfo.Name)
	}

	colsLen := len(tblInfo.Columns)
	columns := make([]*table.Column, 0, colsLen)
	for i, colInfo := range tblInfo.Columns {
		if colInfo.State == model.StateNone {
			return nil, table.ErrColumnStateCantNone.GenWithStack("column %s can't be in none state", colInfo.Name)
		}

		// Print some information when the column's offset isn't equal to i.
		if colInfo.Offset != i {
			log.Errorf("[tables] table %#v schema is wrong, no.%d col %#v, cols len %v", tblInfo, i, tblInfo.Columns[i], colsLen)
		}

		col := table.ToColumn(colInfo)
		if col.IsGenerated() {
			expr, err := parseExpression(colInfo.GeneratedExprString)
			if err != nil {
				return nil, errors.Trace(err)
			}
			expr, err = simpleResolveName(expr, tblInfo)
			if err != nil {
				return nil, errors.Trace(err)
			}
			col.GeneratedExpr = expr
		}
		columns = append(columns, col)
	}

	var t Table
	initTableCommon(&t.tableCommon, tblInfo, tblInfo.ID, columns, alloc)
	if tblInfo.GetPartitionInfo() == nil {
		if err := initTableIndices(&t.tableCommon); err != nil {
			return nil, errors.Trace(err)
		}
		return &t, nil
	}

	return newPartitionedTable(&t, tblInfo)
}

// initTableCommon initializes a tableCommon struct.
func initTableCommon(t *tableCommon, tblInfo *model.TableInfo, physicalTableID int64, cols []*table.Column, alloc autoid.Allocator) {
	t.tableID = tblInfo.ID
	t.physicalTableID = physicalTableID
	t.alloc = alloc
	t.meta = tblInfo
	t.Columns = cols
	t.publicColumns = t.Cols()
	t.writableColumns = t.WritableCols()
	t.writableIndices = t.WritableIndices()
	t.recordPrefix = tablecodec.GenTableRecordPrefix(physicalTableID)
	t.indexPrefix = tablecodec.GenTableIndexPrefix(physicalTableID)
}

// initTableIndices initializes the indices of the tableCommon.
func initTableIndices(t *tableCommon) error {
	tblInfo := t.meta
	for _, idxInfo := range tblInfo.Indices {
		if idxInfo.State == model.StateNone {
			return table.ErrIndexStateCantNone.GenWithStack("index %s can't be in none state", idxInfo.Name)
		}

		// Use partition ID for index, because tableCommon may be table or partition.
		idx := NewIndex(t.physicalTableID, tblInfo, idxInfo)
		t.indices = append(t.indices, idx)
	}
	return nil
}

func initTableCommonWithIndices(t *tableCommon, tblInfo *model.TableInfo, physicalTableID int64, cols []*table.Column, alloc autoid.Allocator) error {
	initTableCommon(t, tblInfo, physicalTableID, cols, alloc)
	return errors.Trace(initTableIndices(t))
}

// Indices implements table.Table Indices interface.
func (t *tableCommon) Indices() []table.Index {
	return t.indices
}

// WritableIndices implements table.Table WritableIndices interface.
func (t *tableCommon) WritableIndices() []table.Index {
	if len(t.writableIndices) > 0 {
		return t.writableIndices
	}
	writable := make([]table.Index, 0, len(t.indices))
	for _, index := range t.indices {
		s := index.Meta().State
		if s != model.StateDeleteOnly && s != model.StateDeleteReorganization {
			writable = append(writable, index)
		}
	}
	return writable
}

// DeletableIndices implements table.Table DeletableIndices interface.
func (t *tableCommon) DeletableIndices() []table.Index {
	// All indices are deletable because we don't need to check StateNone.
	return t.indices
}

// Meta implements table.Table Meta interface.
func (t *tableCommon) Meta() *model.TableInfo {
	return t.meta
}

// GetPhysicalID implements table.Table GetPhysicalID interface.
func (t *Table) GetPhysicalID() int64 {
	return t.physicalTableID
}

// Cols implements table.Table Cols interface.
func (t *tableCommon) Cols() []*table.Column {
	if len(t.publicColumns) > 0 {
		return t.publicColumns
	}
	publicColumns := make([]*table.Column, len(t.Columns))
	maxOffset := -1
	for _, col := range t.Columns {
		if col.State != model.StatePublic {
			continue
		}
		publicColumns[col.Offset] = col
		if maxOffset < col.Offset {
			maxOffset = col.Offset
		}
	}
	return publicColumns[0 : maxOffset+1]
}

// WritableCols implements table WritableCols interface.
func (t *tableCommon) WritableCols() []*table.Column {
	if len(t.writableColumns) > 0 {
		return t.writableColumns
	}
	writableColumns := make([]*table.Column, len(t.Columns))
	maxOffset := -1
	for _, col := range t.Columns {
		if col.State == model.StateDeleteOnly || col.State == model.StateDeleteReorganization {
			continue
		}
		writableColumns[col.Offset] = col
		if maxOffset < col.Offset {
			maxOffset = col.Offset
		}
	}
	return writableColumns[0 : maxOffset+1]
}

// RecordPrefix implements table.Table interface.
func (t *tableCommon) RecordPrefix() kv.Key {
	return t.recordPrefix
}

// IndexPrefix implements table.Table interface.
func (t *tableCommon) IndexPrefix() kv.Key {
	return t.indexPrefix
}

// RecordKey implements table.Table interface.
func (t *tableCommon) RecordKey(h int64) kv.Key {
	return tablecodec.EncodeRecordKey(t.recordPrefix, h)
}

// FirstKey implements table.Table interface.
func (t *tableCommon) FirstKey() kv.Key {
	return t.RecordKey(math.MinInt64)
}

// UpdateRecord implements table.Table UpdateRecord interface.
// `touched` means which columns are really modified, used for secondary indices.
// Length of `oldData` and `newData` equals to length of `t.WritableCols()`.
func (t *tableCommon) UpdateRecord(ctx sessionctx.Context, h int64, oldData, newData []types.Datum, touched []bool) error {
	txn, err := ctx.Txn(true)
	if err != nil {
		return errors.Trace(err)
	}

	// TODO: reuse bs, like AddRecord does.
	bs := kv.NewBufferStore(txn, kv.DefaultTxnMembufCap)

	// rebuild index
	err = t.rebuildIndices(ctx, bs, h, touched, oldData, newData)
	if err != nil {
		return errors.Trace(err)
	}
	numColsCap := len(newData) + 1 // +1 for the extra handle column that we may need to append.

	var colIDs, binlogColIDs []int64
	var row, binlogOldRow, binlogNewRow []types.Datum
	colIDs = make([]int64, 0, numColsCap)
	row = make([]types.Datum, 0, numColsCap)
	if shouldWriteBinlog(ctx) {
		binlogColIDs = make([]int64, 0, numColsCap)
		binlogOldRow = make([]types.Datum, 0, numColsCap)
		binlogNewRow = make([]types.Datum, 0, numColsCap)
	}

	for _, col := range t.WritableCols() {
		var value types.Datum
		if col.State != model.StatePublic {
			// If col is in write only or write reorganization state we should keep the oldData.
			// Because the oldData must be the orignal data(it's changed by other TiDBs.) or the orignal default value.
			// TODO: Use newData directly.
			value = oldData[col.Offset]
		} else {
			value = newData[col.Offset]
		}
		if !t.canSkip(col, value) {
			colIDs = append(colIDs, col.ID)
			row = append(row, value)
		}
		if shouldWriteBinlog(ctx) && !t.canSkipUpdateBinlog(col, value) {
			binlogColIDs = append(binlogColIDs, col.ID)
			binlogOldRow = append(binlogOldRow, oldData[col.Offset])
			binlogNewRow = append(binlogNewRow, value)
		}
	}

	key := t.RecordKey(h)
	value, err := tablecodec.EncodeRow(ctx.GetSessionVars().StmtCtx, row, colIDs, nil, nil)
	if err != nil {
		return errors.Trace(err)
	}
	if err = bs.Set(key, value); err != nil {
		return errors.Trace(err)
	}
	if err = bs.SaveTo(txn); err != nil {
		return errors.Trace(err)
	}
	ctx.StmtAddDirtyTableOP(table.DirtyTableDeleteRow, t.physicalTableID, h, nil)
	ctx.StmtAddDirtyTableOP(table.DirtyTableAddRow, t.physicalTableID, h, newData)
	if shouldWriteBinlog(ctx) {
		if !t.meta.PKIsHandle {
			binlogColIDs = append(binlogColIDs, model.ExtraHandleID)
			binlogOldRow = append(binlogOldRow, types.NewIntDatum(h))
			binlogNewRow = append(binlogNewRow, types.NewIntDatum(h))
		}
		err = t.addUpdateBinlog(ctx, binlogOldRow, binlogNewRow, binlogColIDs)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (t *tableCommon) rebuildIndices(ctx sessionctx.Context, rm kv.RetrieverMutator, h int64, touched []bool, oldData []types.Datum, newData []types.Datum) error {
	for _, idx := range t.DeletableIndices() {
		for _, ic := range idx.Meta().Columns {
			if !touched[ic.Offset] {
				continue
			}
			oldVs, err := idx.FetchValues(oldData, nil)
			if err != nil {
				return errors.Trace(err)
			}
			if err = t.removeRowIndex(ctx.GetSessionVars().StmtCtx, rm, h, oldVs, idx); err != nil {
				return errors.Trace(err)
			}
			break
		}
	}
	for _, idx := range t.WritableIndices() {
		for _, ic := range idx.Meta().Columns {
			if !touched[ic.Offset] {
				continue
			}
			newVs, err := idx.FetchValues(newData, nil)
			if err != nil {
				return errors.Trace(err)
			}
			if err := t.buildIndexForRow(ctx, rm, h, newVs, idx); err != nil {
				return errors.Trace(err)
			}
			break
		}
	}
	return nil
}

// adjustRowValuesBuf adjust writeBufs.AddRowValues length, AddRowValues stores the inserting values that is used
// by tablecodec.EncodeRow, the encoded row format is `id1, colval, id2, colval`, so the correct length is rowLen * 2. If
// the inserting row has null value, AddRecord will skip it, so the rowLen will be different, so we need to adjust it.
func adjustRowValuesBuf(writeBufs *variable.WriteStmtBufs, rowLen int) {
	adjustLen := rowLen * 2
	if writeBufs.AddRowValues == nil || cap(writeBufs.AddRowValues) < adjustLen {
		writeBufs.AddRowValues = make([]types.Datum, adjustLen)
	}
	writeBufs.AddRowValues = writeBufs.AddRowValues[:adjustLen]
}

// getRollbackableMemStore get a rollbackable BufferStore, when we are importing data,
// Just add the kv to transaction's membuf directly.
func (t *tableCommon) getRollbackableMemStore(ctx sessionctx.Context) (kv.RetrieverMutator, error) {
	if ctx.GetSessionVars().LightningMode {
		return ctx.Txn(true)
	}

	bs := ctx.GetSessionVars().GetWriteStmtBufs().BufStore
	if bs == nil {
		txn, err := ctx.Txn(true)
		if err != nil {
			return nil, errors.Trace(err)
		}
		bs = kv.NewBufferStore(txn, kv.DefaultTxnMembufCap)
	} else {
		bs.Reset()
	}
	return bs, nil
}

// AddRecord implements table.Table AddRecord interface.
func (t *tableCommon) AddRecord(ctx sessionctx.Context, r []types.Datum, skipHandleCheck bool) (recordID int64, err error) {
	var hasRecordID bool
	cols := t.Cols()
	if len(r) > len(cols) {
		// The last value is _tidb_rowid.
		recordID = r[len(r)-1].GetInt64()
		hasRecordID = true
	} else {
		for _, col := range cols {
			if col.IsPKHandleColumn(t.meta) {
				recordID = r[col.Offset].GetInt64()
				hasRecordID = true
				break
			}
		}
	}
	if !hasRecordID {
		recordID, err = t.AllocAutoID(ctx)
		if err != nil {
			return 0, errors.Trace(err)
		}
	}

	txn, err := ctx.Txn(true)
	if err != nil {
		return 0, errors.Trace(err)
	}

	sessVars := ctx.GetSessionVars()
	// when LightningMode or BatchCheck is true,
	// no needs to check the key constrains, so we names the variable skipCheck.
	skipCheck := sessVars.LightningMode || ctx.GetSessionVars().StmtCtx.BatchCheck
	if skipCheck {
		txn.SetOption(kv.SkipCheckForWrite, true)
	}

	rm, err := t.getRollbackableMemStore(ctx)
	// Insert new entries into indices.
	h, err := t.addIndices(ctx, recordID, r, rm, skipHandleCheck)
	if err != nil {
		return h, errors.Trace(err)
	}

	var colIDs, binlogColIDs []int64
	var row, binlogRow []types.Datum
	colIDs = make([]int64, 0, len(r))
	row = make([]types.Datum, 0, len(r))

	for _, col := range t.WritableCols() {
		var value types.Datum
		if col.State != model.StatePublic {
			// If col is in write only or write reorganization state, we must add it with its default value.
			value, err = table.GetColOriginDefaultValue(ctx, col.ToInfo())
			if err != nil {
				return 0, errors.Trace(err)
			}
		} else {
			value = r[col.Offset]
		}
		if !t.canSkip(col, value) {
			colIDs = append(colIDs, col.ID)
			row = append(row, value)
		}
	}
	writeBufs := sessVars.GetWriteStmtBufs()
	adjustRowValuesBuf(writeBufs, len(row))
	key := t.RecordKey(recordID)
	writeBufs.RowValBuf, err = tablecodec.EncodeRow(ctx.GetSessionVars().StmtCtx, row, colIDs, writeBufs.RowValBuf, writeBufs.AddRowValues)
	if err != nil {
		return 0, errors.Trace(err)
	}
	value := writeBufs.RowValBuf
	if err = txn.Set(key, value); err != nil {
		return 0, errors.Trace(err)
	}
	if !sessVars.LightningMode {
		if err = rm.(*kv.BufferStore).SaveTo(txn); err != nil {
			return 0, errors.Trace(err)
		}
	}

	if !ctx.GetSessionVars().LightningMode {
		ctx.StmtAddDirtyTableOP(table.DirtyTableAddRow, t.physicalTableID, recordID, r)
	}
	if shouldWriteBinlog(ctx) {
		// For insert, TiDB and Binlog can use same row and schema.
		binlogRow = row
		binlogColIDs = colIDs
		err = t.addInsertBinlog(ctx, recordID, binlogRow, binlogColIDs)
		if err != nil {
			return 0, errors.Trace(err)
		}
	}
	sessVars.StmtCtx.AddAffectedRows(1)
	colSize := make(map[int64]int64)
	for id, col := range t.Cols() {
		val := int64(len(r[id].GetBytes()))
		if val != 0 {
			colSize[col.ID] = val
		}
	}
	sessVars.TxnCtx.UpdateDeltaForTable(t.tableID, 1, 1, colSize)
	return recordID, nil
}

// genIndexKeyStr generates index content string representation.
func (t *tableCommon) genIndexKeyStr(colVals []types.Datum) (string, error) {
	// Pass pre-composed error to txn.
	strVals := make([]string, 0, len(colVals))
	for _, cv := range colVals {
		cvs := "NULL"
		var err error
		if !cv.IsNull() {
			cvs, err = types.ToString(cv.GetValue())
			if err != nil {
				return "", errors.Trace(err)
			}
		}
		strVals = append(strVals, cvs)
	}
	return strings.Join(strVals, "-"), nil
}

// addIndices adds data into indices. If any key is duplicated, returns the original handle.
func (t *tableCommon) addIndices(ctx sessionctx.Context, recordID int64, r []types.Datum, rm kv.RetrieverMutator, skipHandleCheck bool) (int64, error) {
	txn, err := ctx.Txn(true)
	if err != nil {
		return 0, errors.Trace(err)
	}
	// Clean up lazy check error environment
	defer txn.DelOption(kv.PresumeKeyNotExistsError)
	skipCheck := ctx.GetSessionVars().LightningMode || ctx.GetSessionVars().StmtCtx.BatchCheck
	if t.meta.PKIsHandle && !skipCheck && !skipHandleCheck {
		if err := CheckHandleExists(ctx, t, recordID, nil); err != nil {
			return recordID, errors.Trace(err)
		}
	}

	writeBufs := ctx.GetSessionVars().GetWriteStmtBufs()
	indexVals := writeBufs.IndexValsBuf
	for _, v := range t.WritableIndices() {
		var err2 error
		indexVals, err2 = v.FetchValues(r, indexVals)
		if err2 != nil {
			return 0, errors.Trace(err2)
		}
		var dupKeyErr error
		if !skipCheck && (v.Meta().Unique || v.Meta().Primary) {
			entryKey, err1 := t.genIndexKeyStr(indexVals)
			if err1 != nil {
				return 0, errors.Trace(err1)
			}
			dupKeyErr = kv.ErrKeyExists.FastGen("Duplicate entry '%s' for key '%s'", entryKey, v.Meta().Name)
			txn.SetOption(kv.PresumeKeyNotExistsError, dupKeyErr)
		}
		if dupHandle, err := v.Create(ctx, rm, indexVals, recordID); err != nil {
			if kv.ErrKeyExists.Equal(err) {
				return dupHandle, errors.Trace(dupKeyErr)
			}
			return 0, errors.Trace(err)
		}
		txn.DelOption(kv.PresumeKeyNotExistsError)
	}
	// save the buffer, multi rows insert can use it.
	writeBufs.IndexValsBuf = indexVals
	return 0, nil
}

// RowWithCols implements table.Table RowWithCols interface.
func (t *tableCommon) RowWithCols(ctx sessionctx.Context, h int64, cols []*table.Column) ([]types.Datum, error) {
	// Get raw row data from kv.
	key := t.RecordKey(h)
	txn, err := ctx.Txn(true)
	if err != nil {
		return nil, errors.Trace(err)
	}
	value, err := txn.Get(key)
	if err != nil {
		return nil, errors.Trace(err)
	}
	v, _, err := DecodeRawRowData(ctx, t.Meta(), h, cols, value)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return v, nil
}

// DecodeRawRowData decodes raw row data into a datum slice and a (columnID:columnValue) map.
func DecodeRawRowData(ctx sessionctx.Context, meta *model.TableInfo, h int64, cols []*table.Column,
	value []byte) ([]types.Datum, map[int64]types.Datum, error) {
	v := make([]types.Datum, len(cols))
	colTps := make(map[int64]*types.FieldType, len(cols))
	for i, col := range cols {
		if col == nil {
			continue
		}
		if col.IsPKHandleColumn(meta) {
			if mysql.HasUnsignedFlag(col.Flag) {
				v[i].SetUint64(uint64(h))
			} else {
				v[i].SetInt64(h)
			}
			continue
		}
		colTps[col.ID] = &col.FieldType
	}
	rowMap, err := tablecodec.DecodeRow(value, colTps, ctx.GetSessionVars().Location())
	if err != nil {
		return nil, rowMap, errors.Trace(err)
	}
	defaultVals := make([]types.Datum, len(cols))
	for i, col := range cols {
		if col == nil {
			continue
		}
		if col.IsPKHandleColumn(meta) {
			continue
		}
		ri, ok := rowMap[col.ID]
		if ok {
			v[i] = ri
			continue
		}
		v[i], err = GetColDefaultValue(ctx, col, defaultVals)
		if err != nil {
			return nil, rowMap, errors.Trace(err)
		}
	}
	return v, rowMap, nil
}

// Row implements table.Table Row interface.
func (t *tableCommon) Row(ctx sessionctx.Context, h int64) ([]types.Datum, error) {
	r, err := t.RowWithCols(ctx, h, t.Cols())
	if err != nil {
		return nil, errors.Trace(err)
	}
	return r, nil
}

// RemoveRecord implements table.Table RemoveRecord interface.
func (t *tableCommon) RemoveRecord(ctx sessionctx.Context, h int64, r []types.Datum) error {
	err := t.removeRowData(ctx, h)
	if err != nil {
		return errors.Trace(err)
	}
	err = t.removeRowIndices(ctx, h, r)
	if err != nil {
		return errors.Trace(err)
	}

	ctx.StmtAddDirtyTableOP(table.DirtyTableDeleteRow, t.physicalTableID, h, nil)
	if shouldWriteBinlog(ctx) {
		cols := t.Cols()
		colIDs := make([]int64, 0, len(cols)+1)
		for _, col := range cols {
			colIDs = append(colIDs, col.ID)
		}
		var binlogRow []types.Datum
		if !t.meta.PKIsHandle {
			colIDs = append(colIDs, model.ExtraHandleID)
			binlogRow = make([]types.Datum, 0, len(r)+1)
			binlogRow = append(binlogRow, r...)
			binlogRow = append(binlogRow, types.NewIntDatum(h))
		} else {
			binlogRow = r
		}
		err = t.addDeleteBinlog(ctx, binlogRow, colIDs)
	}
	return errors.Trace(err)
}

func (t *tableCommon) addInsertBinlog(ctx sessionctx.Context, h int64, row []types.Datum, colIDs []int64) error {
	mutation := t.getMutation(ctx)
	pk, err := codec.EncodeValue(ctx.GetSessionVars().StmtCtx, nil, types.NewIntDatum(h))
	if err != nil {
		return errors.Trace(err)
	}
	value, err := tablecodec.EncodeRow(ctx.GetSessionVars().StmtCtx, row, colIDs, nil, nil)
	if err != nil {
		return errors.Trace(err)
	}
	bin := append(pk, value...)
	mutation.InsertedRows = append(mutation.InsertedRows, bin)
	mutation.Sequence = append(mutation.Sequence, binlog.MutationType_Insert)
	return nil
}

func (t *tableCommon) addUpdateBinlog(ctx sessionctx.Context, oldRow, newRow []types.Datum, colIDs []int64) error {
	old, err := tablecodec.EncodeRow(ctx.GetSessionVars().StmtCtx, oldRow, colIDs, nil, nil)
	if err != nil {
		return errors.Trace(err)
	}
	newVal, err := tablecodec.EncodeRow(ctx.GetSessionVars().StmtCtx, newRow, colIDs, nil, nil)
	if err != nil {
		return errors.Trace(err)
	}
	bin := append(old, newVal...)
	mutation := t.getMutation(ctx)
	mutation.UpdatedRows = append(mutation.UpdatedRows, bin)
	mutation.Sequence = append(mutation.Sequence, binlog.MutationType_Update)
	return nil
}

func (t *tableCommon) addDeleteBinlog(ctx sessionctx.Context, r []types.Datum, colIDs []int64) error {
	data, err := tablecodec.EncodeRow(ctx.GetSessionVars().StmtCtx, r, colIDs, nil, nil)
	if err != nil {
		return errors.Trace(err)
	}
	mutation := t.getMutation(ctx)
	mutation.DeletedRows = append(mutation.DeletedRows, data)
	mutation.Sequence = append(mutation.Sequence, binlog.MutationType_DeleteRow)
	return nil
}

func (t *tableCommon) removeRowData(ctx sessionctx.Context, h int64) error {
	// Remove row data.
	txn, err := ctx.Txn(true)
	if err != nil {
		return errors.Trace(err)
	}
	err = txn.Delete([]byte(t.RecordKey(h)))
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// removeRowIndices removes all the indices of a row.
func (t *tableCommon) removeRowIndices(ctx sessionctx.Context, h int64, rec []types.Datum) error {
	txn, err := ctx.Txn(true)
	if err != nil {
		return errors.Trace(err)
	}

	for _, v := range t.DeletableIndices() {
		vals, err := v.FetchValues(rec, nil)
		if err != nil {
			log.Infof("remove row index %v failed %v, txn %d, handle %d, data %v", v.Meta(), err, txn.StartTS(), h, rec)
			return errors.Trace(err)
		}
		if err = v.Delete(ctx.GetSessionVars().StmtCtx, txn, vals, h); err != nil {
			if v.Meta().State != model.StatePublic && kv.ErrNotExist.Equal(err) {
				// If the index is not in public state, we may have not created the index,
				// or already deleted the index, so skip ErrNotExist error.
				log.Debugf("remove row index %v doesn't exist, txn %d, handle %d", v.Meta(), txn.StartTS(), h)
				continue
			}
			return errors.Trace(err)
		}
	}
	return nil
}

// removeRowIndex implements table.Table RemoveRowIndex interface.
func (t *tableCommon) removeRowIndex(sc *stmtctx.StatementContext, rm kv.RetrieverMutator, h int64, vals []types.Datum, idx table.Index) error {
	if err := idx.Delete(sc, rm, vals, h); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// buildIndexForRow implements table.Table BuildIndexForRow interface.
func (t *tableCommon) buildIndexForRow(ctx sessionctx.Context, rm kv.RetrieverMutator, h int64, vals []types.Datum, idx table.Index) error {
	if _, err := idx.Create(ctx, rm, vals, h); err != nil {
		if kv.ErrKeyExists.Equal(err) {
			// Make error message consistent with MySQL.
			entryKey, err1 := t.genIndexKeyStr(vals)
			if err1 != nil {
				// if genIndexKeyStr failed, return the original error.
				return errors.Trace(err)
			}

			return kv.ErrKeyExists.FastGen("Duplicate entry '%s' for key '%s'", entryKey, idx.Meta().Name)
		}
		return errors.Trace(err)
	}
	return nil
}

// IterRecords implements table.Table IterRecords interface.
func (t *tableCommon) IterRecords(ctx sessionctx.Context, startKey kv.Key, cols []*table.Column,
	fn table.RecordIterFunc) error {
	prefix := t.RecordPrefix()
	txn, err := ctx.Txn(true)
	if err != nil {
		return errors.Trace(err)
	}

	it, err := txn.Iter(startKey, prefix.PrefixNext())
	if err != nil {
		return errors.Trace(err)
	}
	defer it.Close()

	if !it.Valid() {
		return nil
	}

	log.Debugf("startKey:%q, key:%q, value:%q", startKey, it.Key(), it.Value())

	colMap := make(map[int64]*types.FieldType)
	for _, col := range cols {
		colMap[col.ID] = &col.FieldType
	}
	defaultVals := make([]types.Datum, len(cols))
	for it.Valid() && it.Key().HasPrefix(prefix) {
		// first kv pair is row lock information.
		// TODO: check valid lock
		// get row handle
		handle, err := tablecodec.DecodeRowKey(it.Key())
		if err != nil {
			return errors.Trace(err)
		}
		rowMap, err := tablecodec.DecodeRow(it.Value(), colMap, ctx.GetSessionVars().Location())
		if err != nil {
			return errors.Trace(err)
		}
		data := make([]types.Datum, len(cols))
		for _, col := range cols {
			if col.IsPKHandleColumn(t.meta) {
				if mysql.HasUnsignedFlag(col.Flag) {
					data[col.Offset].SetUint64(uint64(handle))
				} else {
					data[col.Offset].SetInt64(handle)
				}
				continue
			}
			if _, ok := rowMap[col.ID]; ok {
				data[col.Offset] = rowMap[col.ID]
				continue
			}
			data[col.Offset], err = GetColDefaultValue(ctx, col, defaultVals)
			if err != nil {
				return errors.Trace(err)
			}
		}
		more, err := fn(handle, data, cols)
		if !more || err != nil {
			return errors.Trace(err)
		}

		rk := t.RecordKey(handle)
		err = kv.NextUntil(it, util.RowKeyPrefixFilter(rk))
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

// GetColDefaultValue gets a column default value.
// The defaultVals is used to avoid calculating the default value multiple times.
func GetColDefaultValue(ctx sessionctx.Context, col *table.Column, defaultVals []types.Datum) (
	colVal types.Datum, err error) {
	if col.OriginDefaultValue == nil && mysql.HasNotNullFlag(col.Flag) {
		return colVal, errors.New("Miss column")
	}
	if col.State != model.StatePublic {
		return colVal, nil
	}
	if defaultVals[col.Offset].IsNull() {
		colVal, err = table.GetColOriginDefaultValue(ctx, col.ToInfo())
		if err != nil {
			return colVal, errors.Trace(err)
		}
		defaultVals[col.Offset] = colVal
	} else {
		colVal = defaultVals[col.Offset]
	}

	return colVal, nil
}

// AllocAutoID implements table.Table AllocAutoID interface.
func (t *tableCommon) AllocAutoID(ctx sessionctx.Context) (int64, error) {
	rowID, err := t.Allocator(ctx).Alloc(t.tableID)
	if err != nil {
		return 0, errors.Trace(err)
	}
	if t.meta.ShardRowIDBits > 0 {
		txnCtx := ctx.GetSessionVars().TxnCtx
		if txnCtx.Shard == nil {
			shard := t.calcShard(txnCtx.StartTS)
			txnCtx.Shard = &shard
		}
		rowID |= *txnCtx.Shard
	}
	return rowID, nil
}

func (t *tableCommon) calcShard(startTS uint64) int64 {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], startTS)
	hashVal := int64(murmur3.Sum32(buf[:]))
	return (hashVal & (1<<t.meta.ShardRowIDBits - 1)) << (64 - t.meta.ShardRowIDBits - 1)
}

// Allocator implements table.Table Allocator interface.
func (t *tableCommon) Allocator(ctx sessionctx.Context) autoid.Allocator {
	if ctx != nil {
		sessAlloc := ctx.GetSessionVars().IDAllocator
		if sessAlloc != nil {
			return sessAlloc
		}
	}
	return t.alloc
}

// RebaseAutoID implements table.Table RebaseAutoID interface.
func (t *tableCommon) RebaseAutoID(ctx sessionctx.Context, newBase int64, isSetStep bool) error {
	return t.Allocator(ctx).Rebase(t.tableID, newBase, isSetStep)
}

// Seek implements table.Table Seek interface.
func (t *tableCommon) Seek(ctx sessionctx.Context, h int64) (int64, bool, error) {
	txn, err := ctx.Txn(true)
	if err != nil {
		return 0, false, errors.Trace(err)
	}
	seekKey := tablecodec.EncodeRowKeyWithHandle(t.physicalTableID, h)
	iter, err := txn.Iter(seekKey, t.RecordPrefix().PrefixNext())
	if !iter.Valid() || !iter.Key().HasPrefix(t.RecordPrefix()) {
		// No more records in the table, skip to the end.
		return 0, false, nil
	}
	handle, err := tablecodec.DecodeRowKey(iter.Key())
	if err != nil {
		return 0, false, errors.Trace(err)
	}
	return handle, true, nil
}

// Type implements table.Table Type interface.
func (t *tableCommon) Type() table.Type {
	return table.NormalTable
}

func shouldWriteBinlog(ctx sessionctx.Context) bool {
	if ctx.GetSessionVars().BinlogClient == nil {
		return false
	}
	return !ctx.GetSessionVars().InRestrictedSQL
}

func (t *tableCommon) getMutation(ctx sessionctx.Context) *binlog.TableMutation {
	return ctx.StmtGetMutation(t.tableID)
}

func (t *tableCommon) canSkip(col *table.Column, value types.Datum) bool {
	return CanSkip(t.Meta(), col, value)
}

// CanSkip is for these cases, we can skip the columns in encoded row:
// 1. the column is included in primary key;
// 2. the column's default value is null, and the value equals to that;
// 3. the column is virtual generated.
func CanSkip(info *model.TableInfo, col *table.Column, value types.Datum) bool {
	if col.IsPKHandleColumn(info) {
		return true
	}
	if col.GetDefaultValue() == nil && value.IsNull() {
		return true
	}
	if col.IsGenerated() && !col.GeneratedStored {
		return true
	}
	return false
}

// canSkipUpdateBinlog checks whether the column can be skiped or not.
func (t *tableCommon) canSkipUpdateBinlog(col *table.Column, value types.Datum) bool {
	if col.IsGenerated() && !col.GeneratedStored {
		return true
	}
	return false
}

var (
	recordPrefixSep = []byte("_r")
)

// FindIndexByColName implements table.Table FindIndexByColName interface.
func FindIndexByColName(t table.Table, name string) table.Index {
	for _, idx := range t.Indices() {
		// only public index can be read.
		if idx.Meta().State != model.StatePublic {
			continue
		}

		if len(idx.Meta().Columns) == 1 && strings.EqualFold(idx.Meta().Columns[0].Name.L, name) {
			return idx
		}
	}
	return nil
}

// CheckHandleExists check whether recordID key exists. if not exists, return nil,
// otherwise return kv.ErrKeyExists error.
func CheckHandleExists(ctx sessionctx.Context, t table.Table, recordID int64, data []types.Datum) error {
	if pt, ok := t.(*partitionedTable); ok {
		info := t.Meta().GetPartitionInfo()
		pid, err := pt.locatePartition(ctx, info, data)
		if err != nil {
			return errors.Trace(err)
		}
		t = pt.GetPartition(pid)
	}
	txn, err := ctx.Txn(true)
	if err != nil {
		return errors.Trace(err)
	}
	// Check key exists.
	recordKey := t.RecordKey(recordID)
	e := kv.ErrKeyExists.FastGen("Duplicate entry '%d' for key 'PRIMARY'", recordID)
	txn.SetOption(kv.PresumeKeyNotExistsError, e)
	defer txn.DelOption(kv.PresumeKeyNotExistsError)
	_, err = txn.Get(recordKey)
	if err == nil {
		return errors.Trace(e)
	} else if !kv.ErrNotExist.Equal(err) {
		return errors.Trace(err)
	}
	return nil
}

func init() {
	table.TableFromMeta = TableFromMeta
	table.MockTableFromMeta = MockTableFromMeta
}

// ctxForPartitionExpr implement sessionctx.Context interfact.
type ctxForPartitionExpr struct {
	sessionVars *variable.SessionVars
}

// newCtxForPartitionExpr creates a new sessionctx.Context.
func newCtxForPartitionExpr() sessionctx.Context {
	sctx := &ctxForPartitionExpr{
		sessionVars: variable.NewSessionVars(),
	}
	sctx.sessionVars.MaxChunkSize = 2
	sctx.sessionVars.StmtCtx.TimeZone = time.UTC
	return sctx
}

// NewTxn creates a new transaction for further execution.
// If old transaction is valid, it is committed first.
// It's used in BEGIN statement and DDL statements to commit old transaction.
func (ctx *ctxForPartitionExpr) NewTxn() error {
	panic("not support")
}

// Txn returns the current transaction which is created before executing a statement.
func (ctx *ctxForPartitionExpr) Txn(bool) (kv.Transaction, error) {
	panic("not support")
}

// GetClient gets a kv.Client.
func (ctx *ctxForPartitionExpr) GetClient() kv.Client {
	panic("not support")
}

// SetValue saves a value associated with this context for key.
func (ctx *ctxForPartitionExpr) SetValue(key fmt.Stringer, value interface{}) {
	panic("not support")
}

// Value returns the value associated with this context for key.
func (ctx *ctxForPartitionExpr) Value(key fmt.Stringer) interface{} {
	panic("not support")
}

// ClearValue clears the value associated with this context for key.
func (ctx *ctxForPartitionExpr) ClearValue(key fmt.Stringer) {
	panic("not support")
}

func (ctx *ctxForPartitionExpr) GetSessionVars() *variable.SessionVars {
	return ctx.sessionVars
}

// GetSessionManager implements the sessionctx.Context interface.
func (ctx *ctxForPartitionExpr) GetSessionManager() util.SessionManager {
	panic("not support")
}

// RefreshTxnCtx commits old transaction without retry,
// and creates a new transaction.
// now just for load data and batch insert.
func (ctx *ctxForPartitionExpr) RefreshTxnCtx(context.Context) error {
	panic("not support")
}

// InitTxnWithStartTS initializes a transaction with startTS.
// It should be called right before we builds an executor.
func (ctx *ctxForPartitionExpr) InitTxnWithStartTS(startTS uint64) error {
	panic("not support")
}

// GetStore returns the store of session.
func (ctx *ctxForPartitionExpr) GetStore() kv.Storage {
	panic("not support")
}

// PreparedPlanCache returns the cache of the physical plan
func (ctx *ctxForPartitionExpr) PreparedPlanCache() *kvcache.SimpleLRUCache {
	panic("not support")
}

// StoreQueryFeedback stores the query feedback.
func (ctx *ctxForPartitionExpr) StoreQueryFeedback(feedback interface{}) {
	panic("not support")
}

// StmtCommit flush all changes by the statement to the underlying transaction.
func (ctx *ctxForPartitionExpr) StmtCommit() error {
	panic("not support")
}

// StmtRollback provides statement level rollback.
func (ctx *ctxForPartitionExpr) StmtRollback() {
	panic("not support")
}

// StmtGetMutation gets the binlog mutation for current statement.
func (ctx *ctxForPartitionExpr) StmtGetMutation(int64) *binlog.TableMutation {
	panic("not support")
}

// StmtAddDirtyTableOP adds the dirty table operation for current statement.
func (ctx *ctxForPartitionExpr) StmtAddDirtyTableOP(op int, tid int64, handle int64, row []types.Datum) {
	panic("not support")
}

// DDLOwnerChecker returns owner.DDLOwnerChecker.
func (ctx *ctxForPartitionExpr) DDLOwnerChecker() owner.DDLOwnerChecker {
	panic("not support")
}
