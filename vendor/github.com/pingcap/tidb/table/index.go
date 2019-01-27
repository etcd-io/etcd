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

package table

import (
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
)

// IndexIterator is the interface for iterator of index data on KV store.
type IndexIterator interface {
	Next() (k []types.Datum, h int64, err error)
	Close()
}

// Index is the interface for index data on KV store.
type Index interface {
	// Meta returns IndexInfo.
	Meta() *model.IndexInfo
	// Create supports insert into statement.
	Create(ctx sessionctx.Context, rm kv.RetrieverMutator, indexedValues []types.Datum, h int64) (int64, error)
	// Delete supports delete from statement.
	Delete(sc *stmtctx.StatementContext, m kv.Mutator, indexedValues []types.Datum, h int64) error
	// Drop supports drop table, drop index statements.
	Drop(rm kv.RetrieverMutator) error
	// Exist supports check index exists or not.
	Exist(sc *stmtctx.StatementContext, rm kv.RetrieverMutator, indexedValues []types.Datum, h int64) (bool, int64, error)
	// GenIndexKey generates an index key.
	GenIndexKey(sc *stmtctx.StatementContext, indexedValues []types.Datum, h int64, buf []byte) (key []byte, distinct bool, err error)
	// Seek supports where clause.
	Seek(sc *stmtctx.StatementContext, r kv.Retriever, indexedValues []types.Datum) (iter IndexIterator, hit bool, err error)
	// SeekFirst supports aggregate min and ascend order by.
	SeekFirst(r kv.Retriever) (iter IndexIterator, err error)
	// FetchValues fetched index column values in a row.
	// Param columns is a reused buffer, if it is not nil, FetchValues will fill the index values in it,
	// and return the buffer, if it is nil, FetchValues will allocate the buffer instead.
	FetchValues(row []types.Datum, columns []types.Datum) ([]types.Datum, error)
}
