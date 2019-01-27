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

package tables

import (
	"bytes"
	"encoding/binary"
	"io"
	"unicode/utf8"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
)

// EncodeHandle encodes handle in data.
func EncodeHandle(h int64) []byte {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.BigEndian, h)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// DecodeHandle decodes handle in data.
func DecodeHandle(data []byte) (int64, error) {
	var h int64
	buf := bytes.NewBuffer(data)
	err := binary.Read(buf, binary.BigEndian, &h)
	return h, errors.Trace(err)
}

// indexIter is for KV store index iterator.
type indexIter struct {
	it     kv.Iterator
	idx    *index
	prefix kv.Key
}

// Close does the clean up works when KV store index iterator is closed.
func (c *indexIter) Close() {
	if c.it != nil {
		c.it.Close()
		c.it = nil
	}
}

// Next returns current key and moves iterator to the next step.
func (c *indexIter) Next() (val []types.Datum, h int64, err error) {
	if !c.it.Valid() {
		return nil, 0, errors.Trace(io.EOF)
	}
	if !c.it.Key().HasPrefix(c.prefix) {
		return nil, 0, errors.Trace(io.EOF)
	}
	// get indexedValues
	buf := c.it.Key()[len(c.prefix):]
	vv, err := codec.Decode(buf, len(c.idx.idxInfo.Columns))
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	if len(vv) > len(c.idx.idxInfo.Columns) {
		h = vv[len(vv)-1].GetInt64()
		val = vv[0 : len(vv)-1]
	} else {
		// If the index is unique and the value isn't nil, the handle is in value.
		h, err = DecodeHandle(c.it.Value())
		if err != nil {
			return nil, 0, errors.Trace(err)
		}
		val = vv
	}
	// update new iter to next
	err = c.it.Next()
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	return
}

// index is the data structure for index data in the KV store.
type index struct {
	idxInfo *model.IndexInfo
	tblInfo *model.TableInfo
	prefix  kv.Key
}

// NewIndex builds a new Index object.
func NewIndex(physicalID int64, tblInfo *model.TableInfo, indexInfo *model.IndexInfo) table.Index {
	index := &index{
		idxInfo: indexInfo,
		tblInfo: tblInfo,
		// The prefix can't encode from tblInfo.ID, because table partition may change the id to partition id.
		prefix: tablecodec.EncodeTableIndexPrefix(physicalID, indexInfo.ID),
	}
	return index
}

// Meta returns index info.
func (c *index) Meta() *model.IndexInfo {
	return c.idxInfo
}

func (c *index) getIndexKeyBuf(buf []byte, defaultCap int) []byte {
	if buf != nil {
		return buf[:0]
	}

	return make([]byte, 0, defaultCap)
}

// TruncateIndexValuesIfNeeded truncates the index values created using only the leading part of column values.
func TruncateIndexValuesIfNeeded(tblInfo *model.TableInfo, idxInfo *model.IndexInfo, indexedValues []types.Datum) []types.Datum {
	for i := 0; i < len(indexedValues); i++ {
		v := &indexedValues[i]
		if v.Kind() == types.KindString || v.Kind() == types.KindBytes {
			ic := idxInfo.Columns[i]
			colCharset := tblInfo.Columns[ic.Offset].Charset
			colValue := v.GetBytes()
			isUTF8Charset := colCharset == charset.CharsetUTF8 || colCharset == charset.CharsetUTF8MB4
			origKind := v.Kind()
			if isUTF8Charset {
				if ic.Length != types.UnspecifiedLength && utf8.RuneCount(colValue) > ic.Length {
					rs := bytes.Runes(colValue)
					truncateStr := string(rs[:ic.Length])
					// truncate value and limit its length
					v.SetString(truncateStr)
					if origKind == types.KindBytes {
						v.SetBytes(v.GetBytes())
					}
				}
			} else if ic.Length != types.UnspecifiedLength && len(colValue) > ic.Length {
				// truncate value and limit its length
				v.SetBytes(colValue[:ic.Length])
				if origKind == types.KindString {
					v.SetString(v.GetString())
				}
			}
		}
	}

	return indexedValues
}

// GenIndexKey generates storage key for index values. Returned distinct indicates whether the
// indexed values should be distinct in storage (i.e. whether handle is encoded in the key).
func (c *index) GenIndexKey(sc *stmtctx.StatementContext, indexedValues []types.Datum, h int64, buf []byte) (key []byte, distinct bool, err error) {
	if c.idxInfo.Unique {
		// See https://dev.mysql.com/doc/refman/5.7/en/create-index.html
		// A UNIQUE index creates a constraint such that all values in the index must be distinct.
		// An error occurs if you try to add a new row with a key value that matches an existing row.
		// For all engines, a UNIQUE index permits multiple NULL values for columns that can contain NULL.
		distinct = true
		for _, cv := range indexedValues {
			if cv.IsNull() {
				distinct = false
				break
			}
		}
	}

	// For string columns, indexes can be created using only the leading part of column values,
	// using col_name(length) syntax to specify an index prefix length.
	indexedValues = TruncateIndexValuesIfNeeded(c.tblInfo, c.idxInfo, indexedValues)
	key = c.getIndexKeyBuf(buf, len(c.prefix)+len(indexedValues)*9+9)
	key = append(key, []byte(c.prefix)...)
	key, err = codec.EncodeKey(sc, key, indexedValues...)
	if !distinct && err == nil {
		key, err = codec.EncodeKey(sc, key, types.NewDatum(h))
	}
	if err != nil {
		return nil, false, errors.Trace(err)
	}
	return
}

// Create creates a new entry in the kvIndex data.
// If the index is unique and there is an existing entry with the same key,
// Create will return the existing entry's handle as the first return value, ErrKeyExists as the second return value.
func (c *index) Create(ctx sessionctx.Context, rm kv.RetrieverMutator, indexedValues []types.Datum, h int64) (int64, error) {
	writeBufs := ctx.GetSessionVars().GetWriteStmtBufs()
	skipCheck := ctx.GetSessionVars().LightningMode || ctx.GetSessionVars().StmtCtx.BatchCheck
	key, distinct, err := c.GenIndexKey(ctx.GetSessionVars().StmtCtx, indexedValues, h, writeBufs.IndexKeyBuf)
	if err != nil {
		return 0, errors.Trace(err)
	}
	// save the key buffer to reuse.
	writeBufs.IndexKeyBuf = key
	if !distinct {
		// non-unique index doesn't need store value, write a '0' to reduce space
		err = rm.Set(key, []byte{'0'})
		return 0, errors.Trace(err)
	}

	var value []byte
	if !skipCheck {
		value, err = rm.Get(key)
	}

	if skipCheck || kv.IsErrNotFound(err) {
		err = rm.Set(key, EncodeHandle(h))
		return 0, errors.Trace(err)
	}

	handle, err := DecodeHandle(value)
	if err != nil {
		return 0, errors.Trace(err)
	}
	return handle, kv.ErrKeyExists
}

// Delete removes the entry for handle h and indexdValues from KV index.
func (c *index) Delete(sc *stmtctx.StatementContext, m kv.Mutator, indexedValues []types.Datum, h int64) error {
	key, _, err := c.GenIndexKey(sc, indexedValues, h, nil)
	if err != nil {
		return errors.Trace(err)
	}
	err = m.Delete(key)
	return errors.Trace(err)
}

// Drop removes the KV index from store.
func (c *index) Drop(rm kv.RetrieverMutator) error {
	it, err := rm.Iter(c.prefix, c.prefix.PrefixNext())
	if err != nil {
		return errors.Trace(err)
	}
	defer it.Close()

	// remove all indices
	for it.Valid() {
		if !it.Key().HasPrefix(c.prefix) {
			break
		}
		err := rm.Delete(it.Key())
		if err != nil {
			return errors.Trace(err)
		}
		err = it.Next()
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Seek searches KV index for the entry with indexedValues.
func (c *index) Seek(sc *stmtctx.StatementContext, r kv.Retriever, indexedValues []types.Datum) (iter table.IndexIterator, hit bool, err error) {
	key, _, err := c.GenIndexKey(sc, indexedValues, 0, nil)
	if err != nil {
		return nil, false, errors.Trace(err)
	}

	upperBound := c.prefix.PrefixNext()
	it, err := r.Iter(key, upperBound)
	if err != nil {
		return nil, false, errors.Trace(err)
	}
	// check if hit
	hit = false
	if it.Valid() && it.Key().Cmp(key) == 0 {
		hit = true
	}
	return &indexIter{it: it, idx: c, prefix: c.prefix}, hit, nil
}

// SeekFirst returns an iterator which points to the first entry of the KV index.
func (c *index) SeekFirst(r kv.Retriever) (iter table.IndexIterator, err error) {
	upperBound := c.prefix.PrefixNext()
	it, err := r.Iter(c.prefix, upperBound)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &indexIter{it: it, idx: c, prefix: c.prefix}, nil
}

func (c *index) Exist(sc *stmtctx.StatementContext, rm kv.RetrieverMutator, indexedValues []types.Datum, h int64) (bool, int64, error) {
	key, distinct, err := c.GenIndexKey(sc, indexedValues, h, nil)
	if err != nil {
		return false, 0, errors.Trace(err)
	}

	value, err := rm.Get(key)
	if kv.IsErrNotFound(err) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, errors.Trace(err)
	}

	// For distinct index, the value of key is handle.
	if distinct {
		handle, err := DecodeHandle(value)
		if err != nil {
			return false, 0, errors.Trace(err)
		}

		if handle != h {
			return true, handle, errors.Trace(kv.ErrKeyExists)
		}

		return true, handle, nil
	}

	return true, h, nil
}

func (c *index) FetchValues(r []types.Datum, vals []types.Datum) ([]types.Datum, error) {
	needLength := len(c.idxInfo.Columns)
	if vals == nil || cap(vals) < needLength {
		vals = make([]types.Datum, needLength)
	}
	vals = vals[:needLength]
	for i, ic := range c.idxInfo.Columns {
		if ic.Offset < 0 || ic.Offset >= len(r) {
			return nil, table.ErrIndexOutBound.GenWithStack("Index column %s offset out of bound, offset: %d, row: %v",
				ic.Name, ic.Offset, r)
		}
		vals[i] = r[ic.Offset]
	}
	return vals, nil
}
