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

package table

import (
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
)

// Type , the type of table, store data in different ways.
type Type int16

const (
	// NormalTable , store data in tikv, mocktikv and so on.
	NormalTable Type = iota
	// VirtualTable , store no data, just extract data from the memory struct.
	VirtualTable
	// MemoryTable , store data only in local memory.
	MemoryTable
)

const (
	// DirtyTableAddRow is the constant for dirty table operation type.
	DirtyTableAddRow = iota
	// DirtyTableDeleteRow is the constant for dirty table operation type.
	DirtyTableDeleteRow
	// DirtyTableTruncate is the constant for dirty table operation type.
	DirtyTableTruncate
)

var (
	// ErrColumnCantNull is used for inserting null to a not null column.
	ErrColumnCantNull  = terror.ClassTable.New(codeColumnCantNull, mysql.MySQLErrName[mysql.ErrBadNull])
	errUnknownColumn   = terror.ClassTable.New(codeUnknownColumn, "unknown column")
	errDuplicateColumn = terror.ClassTable.New(codeDuplicateColumn, "duplicate column")

	errGetDefaultFailed = terror.ClassTable.New(codeGetDefaultFailed, "get default value fail")

	// ErrNoDefaultValue is used when insert a row, the column value is not given, and the column has not null flag
	// and it doesn't have a default value.
	ErrNoDefaultValue = terror.ClassTable.New(codeNoDefaultValue, "field doesn't have a default value")
	// ErrIndexOutBound returns for index column offset out of bound.
	ErrIndexOutBound = terror.ClassTable.New(codeIndexOutBound, "index column offset out of bound")
	// ErrUnsupportedOp returns for unsupported operation.
	ErrUnsupportedOp = terror.ClassTable.New(codeUnsupportedOp, "operation not supported")
	// ErrRowNotFound returns for row not found.
	ErrRowNotFound = terror.ClassTable.New(codeRowNotFound, "can not find the row")
	// ErrTableStateCantNone returns for table none state.
	ErrTableStateCantNone = terror.ClassTable.New(codeTableStateCantNone, "table can not be in none state")
	// ErrColumnStateCantNone returns for column none state.
	ErrColumnStateCantNone = terror.ClassTable.New(codeColumnStateCantNone, "column can not be in none state")
	// ErrColumnStateNonPublic returns for column non-public state.
	ErrColumnStateNonPublic = terror.ClassTable.New(codeColumnStateNonPublic, "can not use non-public column")
	// ErrIndexStateCantNone returns for index none state.
	ErrIndexStateCantNone = terror.ClassTable.New(codeIndexStateCantNone, "index can not be in none state")
	// ErrInvalidRecordKey returns for invalid record key.
	ErrInvalidRecordKey = terror.ClassTable.New(codeInvalidRecordKey, "invalid record key")
	// ErrTruncateWrongValue returns for truncate wrong value for field.
	ErrTruncateWrongValue = terror.ClassTable.New(codeTruncateWrongValue, "incorrect value")
	// ErrTruncatedWrongValueForField returns for truncate wrong value for field.
	ErrTruncatedWrongValueForField = terror.ClassTable.New(codeTruncateWrongValue, mysql.MySQLErrName[mysql.ErrTruncatedWrongValueForField])
	// ErrTrgInvalidCreationCtx happens when inserting a value outside the table partitions.
	ErrTrgInvalidCreationCtx = terror.ClassTable.New(codeTrgInvalidCreationCtx, "locate partition failed")
)

// RecordIterFunc is used for low-level record iteration.
type RecordIterFunc func(h int64, rec []types.Datum, cols []*Column) (more bool, err error)

// Table is used to retrieve and modify rows in table.
type Table interface {
	// IterRecords iterates records in the table and calls fn.
	IterRecords(ctx sessionctx.Context, startKey kv.Key, cols []*Column, fn RecordIterFunc) error

	// RowWithCols returns a row that contains the given cols.
	RowWithCols(ctx sessionctx.Context, h int64, cols []*Column) ([]types.Datum, error)

	// Row returns a row for all columns.
	Row(ctx sessionctx.Context, h int64) ([]types.Datum, error)

	// Cols returns the columns of the table which is used in select.
	Cols() []*Column

	// WritableCols returns columns of the table in writable states.
	// Writable states includes Public, WriteOnly, WriteOnlyReorganization.
	WritableCols() []*Column

	// Indices returns the indices of the table.
	Indices() []Index

	// WritableIndices returns write-only and public indices of the table.
	WritableIndices() []Index

	// DeletableIndices returns delete-only, write-only and public indices of the table.
	DeletableIndices() []Index

	// RecordPrefix returns the record key prefix.
	RecordPrefix() kv.Key

	// IndexPrefix returns the index key prefix.
	IndexPrefix() kv.Key

	// FirstKey returns the first key.
	FirstKey() kv.Key

	// RecordKey returns the key in KV storage for the row.
	RecordKey(h int64) kv.Key

	// AddRecord inserts a row which should contain only public columns
	// skipHandleCheck indicates that recordID in r has been checked as not duplicate already.
	AddRecord(ctx sessionctx.Context, r []types.Datum, skipHandleCheck bool) (recordID int64, err error)

	// UpdateRecord updates a row which should contain only writable columns.
	UpdateRecord(ctx sessionctx.Context, h int64, currData, newData []types.Datum, touched []bool) error

	// RemoveRecord removes a row in the table.
	RemoveRecord(ctx sessionctx.Context, h int64, r []types.Datum) error

	// AllocAutoID allocates an auto_increment ID for a new row.
	AllocAutoID(ctx sessionctx.Context) (int64, error)

	// Allocator returns Allocator.
	Allocator(ctx sessionctx.Context) autoid.Allocator

	// RebaseAutoID rebases the auto_increment ID base.
	// If allocIDs is true, it will allocate some IDs and save to the cache.
	// If allocIDs is false, it will not allocate IDs.
	RebaseAutoID(ctx sessionctx.Context, newBase int64, allocIDs bool) error

	// Meta returns TableInfo.
	Meta() *model.TableInfo

	// Seek returns the handle greater or equal to h.
	Seek(ctx sessionctx.Context, h int64) (handle int64, found bool, err error)

	// Type returns the type of table
	Type() Type
}

// PhysicalTable is an abstraction for two kinds of table representation: partition or non-partitioned table.
// PhysicalID is a ID that can be used to construct a key ranges, all the data in the key range belongs to the corresponding PhysicalTable.
// For a non-partitioned table, its PhysicalID equals to its TableID; For a partition of a partitioned table, its PhysicalID is the partition's ID.
type PhysicalTable interface {
	Table
	GetPhysicalID() int64
}

// PartitionedTable is a Table, and it has a GetPartition() method.
// GetPartition() gets the partition from a partition table by a physical table ID,
type PartitionedTable interface {
	Table
	GetPartition(physicalID int64) PhysicalTable
	GetPartitionByRow(sessionctx.Context, []types.Datum) (Table, error)
}

// TableFromMeta builds a table.Table from *model.TableInfo.
// Currently, it is assigned to tables.TableFromMeta in tidb package's init function.
var TableFromMeta func(alloc autoid.Allocator, tblInfo *model.TableInfo) (Table, error)

// MockTableFromMeta only serves for test.
var MockTableFromMeta func(tableInfo *model.TableInfo) Table

// Table error codes.
const (
	codeGetDefaultFailed     = 1
	codeIndexOutBound        = 2
	codeUnsupportedOp        = 3
	codeRowNotFound          = 4
	codeTableStateCantNone   = 5
	codeColumnStateCantNone  = 6
	codeColumnStateNonPublic = 7
	codeIndexStateCantNone   = 8
	codeInvalidRecordKey     = 9

	codeColumnCantNull     = mysql.ErrBadNull
	codeUnknownColumn      = 1054
	codeDuplicateColumn    = 1110
	codeNoDefaultValue     = 1364
	codeTruncateWrongValue = 1366
	// MySQL error code, "Trigger creation context of table `%-.64s`.`%-.64s` is invalid".
	// It may happen when inserting some data outside of all table partitions.
	codeTrgInvalidCreationCtx = 1604
)

// Slice is used for table sorting.
type Slice []Table

func (s Slice) Len() int { return len(s) }

func (s Slice) Less(i, j int) bool {
	return s[i].Meta().Name.O < s[j].Meta().Name.O
}

func (s Slice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func init() {
	tableMySQLErrCodes := map[terror.ErrCode]uint16{
		codeColumnCantNull:        mysql.ErrBadNull,
		codeUnknownColumn:         mysql.ErrBadField,
		codeDuplicateColumn:       mysql.ErrFieldSpecifiedTwice,
		codeNoDefaultValue:        mysql.ErrNoDefaultForField,
		codeTruncateWrongValue:    mysql.ErrTruncatedWrongValueForField,
		codeTrgInvalidCreationCtx: mysql.ErrTrgInvalidCreationCtx,
	}
	terror.ErrClassToMySQLCodes[terror.ClassTable] = tableMySQLErrCodes
}
