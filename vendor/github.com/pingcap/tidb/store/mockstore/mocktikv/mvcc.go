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

package mocktikv

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"sort"

	"github.com/google/btree"
	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/tidb/util/codec"
)

type mvccValueType int

const (
	typePut mvccValueType = iota
	typeDelete
	typeRollback
)

type mvccValue struct {
	valueType mvccValueType
	startTS   uint64
	commitTS  uint64
	value     []byte
}

type mvccLock struct {
	startTS uint64
	primary []byte
	value   []byte
	op      kvrpcpb.Op
	ttl     uint64
}

type mvccEntry struct {
	key    MvccKey
	values []mvccValue
	lock   *mvccLock
}

// MarshalBinary implements encoding.BinaryMarshaler interface.
func (l *mvccLock) MarshalBinary() ([]byte, error) {
	var (
		mh  marshalHelper
		buf bytes.Buffer
	)
	mh.WriteNumber(&buf, l.startTS)
	mh.WriteSlice(&buf, l.primary)
	mh.WriteSlice(&buf, l.value)
	mh.WriteNumber(&buf, l.op)
	mh.WriteNumber(&buf, l.ttl)
	return buf.Bytes(), errors.Trace(mh.err)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface.
func (l *mvccLock) UnmarshalBinary(data []byte) error {
	var mh marshalHelper
	buf := bytes.NewBuffer(data)
	mh.ReadNumber(buf, &l.startTS)
	mh.ReadSlice(buf, &l.primary)
	mh.ReadSlice(buf, &l.value)
	mh.ReadNumber(buf, &l.op)
	mh.ReadNumber(buf, &l.ttl)
	return errors.Trace(mh.err)
}

// MarshalBinary implements encoding.BinaryMarshaler interface.
func (v mvccValue) MarshalBinary() ([]byte, error) {
	var (
		mh  marshalHelper
		buf bytes.Buffer
	)
	mh.WriteNumber(&buf, int64(v.valueType))
	mh.WriteNumber(&buf, v.startTS)
	mh.WriteNumber(&buf, v.commitTS)
	mh.WriteSlice(&buf, v.value)
	return buf.Bytes(), errors.Trace(mh.err)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface.
func (v *mvccValue) UnmarshalBinary(data []byte) error {
	var mh marshalHelper
	buf := bytes.NewBuffer(data)
	var vt int64
	mh.ReadNumber(buf, &vt)
	v.valueType = mvccValueType(vt)
	mh.ReadNumber(buf, &v.startTS)
	mh.ReadNumber(buf, &v.commitTS)
	mh.ReadSlice(buf, &v.value)
	return errors.Trace(mh.err)
}

type marshalHelper struct {
	err error
}

func (mh *marshalHelper) WriteSlice(buf io.Writer, slice []byte) {
	if mh.err != nil {
		return
	}
	var tmp [binary.MaxVarintLen64]byte
	off := binary.PutUvarint(tmp[:], uint64(len(slice)))
	if err := writeFull(buf, tmp[:off]); err != nil {
		mh.err = errors.Trace(err)
		return
	}
	if err := writeFull(buf, slice); err != nil {
		mh.err = errors.Trace(err)
	}
}

func (mh *marshalHelper) WriteNumber(buf io.Writer, n interface{}) {
	if mh.err != nil {
		return
	}
	err := binary.Write(buf, binary.LittleEndian, n)
	if err != nil {
		mh.err = errors.Trace(err)
	}
}

func writeFull(w io.Writer, slice []byte) error {
	written := 0
	for written < len(slice) {
		n, err := w.Write(slice[written:])
		if err != nil {
			return errors.Trace(err)
		}
		written += n
	}
	return nil
}

func (mh *marshalHelper) ReadNumber(r io.Reader, n interface{}) {
	if mh.err != nil {
		return
	}
	err := binary.Read(r, binary.LittleEndian, n)
	if err != nil {
		mh.err = errors.Trace(err)
	}
}

func (mh *marshalHelper) ReadSlice(r *bytes.Buffer, slice *[]byte) {
	if mh.err != nil {
		return
	}
	sz, err := binary.ReadUvarint(r)
	if err != nil {
		mh.err = errors.Trace(err)
		return
	}
	const c10M = 10 * 1024 * 1024
	if sz > c10M {
		mh.err = errors.New("too large slice, maybe something wrong")
		return
	}
	data := make([]byte, sz)
	if _, err := io.ReadFull(r, data); err != nil {
		mh.err = errors.Trace(err)
		return
	}
	*slice = data
}

func newEntry(key MvccKey) *mvccEntry {
	return &mvccEntry{
		key: key,
	}
}

// lockErr returns ErrLocked.
// Note that parameter key is raw key, while key in ErrLocked is mvcc key.
func (l *mvccLock) lockErr(key []byte) error {
	return &ErrLocked{
		Key:     mvccEncode(key, lockVer),
		Primary: l.primary,
		StartTS: l.startTS,
		TTL:     l.ttl,
	}
}

func (l *mvccLock) check(ts uint64, key []byte) (uint64, error) {
	// ignore when ts is older than lock or lock's type is Lock.
	if l.startTS > ts || l.op == kvrpcpb.Op_Lock {
		return ts, nil
	}
	// for point get latest version.
	if ts == math.MaxUint64 && bytes.Equal(l.primary, key) {
		return l.startTS - 1, nil
	}
	return 0, l.lockErr(key)
}

func (e *mvccEntry) Clone() *mvccEntry {
	var entry mvccEntry
	entry.key = append([]byte(nil), e.key...)
	for _, v := range e.values {
		entry.values = append(entry.values, mvccValue{
			valueType: v.valueType,
			startTS:   v.startTS,
			commitTS:  v.commitTS,
			value:     append([]byte(nil), v.value...),
		})
	}
	if e.lock != nil {
		entry.lock = &mvccLock{
			startTS: e.lock.startTS,
			primary: append([]byte(nil), e.lock.primary...),
			value:   append([]byte(nil), e.lock.value...),
			op:      e.lock.op,
			ttl:     e.lock.ttl,
		}
	}
	return &entry
}

func (e *mvccEntry) Less(than btree.Item) bool {
	return bytes.Compare(e.key, than.(*mvccEntry).key) < 0
}

func (e *mvccEntry) Get(ts uint64, isoLevel kvrpcpb.IsolationLevel) ([]byte, error) {
	if isoLevel == kvrpcpb.IsolationLevel_SI && e.lock != nil {
		var err error
		ts, err = e.lock.check(ts, e.key.Raw())
		if err != nil {
			return nil, err
		}
	}
	for _, v := range e.values {
		if v.commitTS <= ts && v.valueType != typeRollback {
			return v.value, nil
		}
	}
	return nil, nil
}

func (e *mvccEntry) Prewrite(mutation *kvrpcpb.Mutation, startTS uint64, primary []byte, ttl uint64) error {
	if len(e.values) > 0 {
		if e.values[0].commitTS >= startTS {
			return ErrRetryable("write conflict")
		}
	}
	if e.lock != nil {
		if e.lock.startTS != startTS {
			return e.lock.lockErr(e.key.Raw())
		}
		return nil
	}
	e.lock = &mvccLock{
		startTS: startTS,
		primary: primary,
		value:   mutation.Value,
		op:      mutation.GetOp(),
		ttl:     ttl,
	}
	return nil
}

func (e *mvccEntry) getTxnCommitInfo(startTS uint64) *mvccValue {
	for _, v := range e.values {
		if v.startTS == startTS {
			return &v
		}
	}
	return nil
}

func (e *mvccEntry) Commit(startTS, commitTS uint64) error {
	if e.lock == nil || e.lock.startTS != startTS {
		if c := e.getTxnCommitInfo(startTS); c != nil && c.valueType != typeRollback {
			return nil
		}
		return ErrRetryable("txn not found")
	}
	if e.lock.op != kvrpcpb.Op_Lock {
		var valueType mvccValueType
		if e.lock.op == kvrpcpb.Op_Put {
			valueType = typePut
		} else {
			valueType = typeDelete
		}
		e.addValue(mvccValue{
			valueType: valueType,
			startTS:   startTS,
			commitTS:  commitTS,
			value:     e.lock.value,
		})
	}
	e.lock = nil
	return nil
}

func (e *mvccEntry) Rollback(startTS uint64) error {
	// If current transaction's lock exist.
	if e.lock != nil && e.lock.startTS == startTS {
		e.lock = nil
		e.addValue(mvccValue{
			valueType: typeRollback,
			startTS:   startTS,
			commitTS:  startTS,
		})
		return nil
	}

	// If current transaction's lock not exist.
	// If commit info of current transaction exist.
	if c := e.getTxnCommitInfo(startTS); c != nil {
		// If current transaction is already committed.
		if c.valueType != typeRollback {
			return ErrAlreadyCommitted(c.commitTS)
		}
		// If current transaction is already rollback.
		return nil
	}
	// If current transaction is not prewritted before.
	e.addValue(mvccValue{
		valueType: typeRollback,
		startTS:   startTS,
		commitTS:  startTS,
	})
	return nil
}

func (e *mvccEntry) addValue(v mvccValue) {
	i := sort.Search(len(e.values), func(i int) bool { return e.values[i].commitTS <= v.commitTS })
	if i >= len(e.values) {
		e.values = append(e.values, v)
	} else {
		e.values = append(e.values[:i+1], e.values[i:]...)
		e.values[i] = v
	}
}

func (e *mvccEntry) containsStartTS(startTS uint64) bool {
	if e.lock != nil && e.lock.startTS == startTS {
		return true
	}
	for _, item := range e.values {
		if item.startTS == startTS {
			return true
		}
		if item.commitTS < startTS {
			return false
		}
	}
	return false
}

func (e *mvccEntry) dumpMvccInfo() *kvrpcpb.MvccInfo {
	info := &kvrpcpb.MvccInfo{}
	if e.lock != nil {
		info.Lock = &kvrpcpb.MvccLock{
			Type:       e.lock.op,
			StartTs:    e.lock.startTS,
			Primary:    e.lock.primary,
			ShortValue: e.lock.value,
		}
	}

	info.Writes = make([]*kvrpcpb.MvccWrite, len(e.values))
	info.Values = make([]*kvrpcpb.MvccValue, len(e.values))

	for id, item := range e.values {
		var tp kvrpcpb.Op
		switch item.valueType {
		case typePut:
			tp = kvrpcpb.Op_Put
		case typeDelete:
			tp = kvrpcpb.Op_Del
		case typeRollback:
			tp = kvrpcpb.Op_Rollback
		}
		info.Writes[id] = &kvrpcpb.MvccWrite{
			Type:     tp,
			StartTs:  item.startTS,
			CommitTs: item.commitTS,
		}

		info.Values[id] = &kvrpcpb.MvccValue{
			Value:   item.value,
			StartTs: item.startTS,
		}
	}
	return info
}

type rawEntry struct {
	key   []byte
	value []byte
}

func newRawEntry(key []byte) *rawEntry {
	return &rawEntry{
		key: key,
	}
}

func (e *rawEntry) Less(than btree.Item) bool {
	return bytes.Compare(e.key, than.(*rawEntry).key) < 0
}

// MVCCStore is a mvcc key-value storage.
type MVCCStore interface {
	Get(key []byte, startTS uint64, isoLevel kvrpcpb.IsolationLevel) ([]byte, error)
	Scan(startKey, endKey []byte, limit int, startTS uint64, isoLevel kvrpcpb.IsolationLevel) []Pair
	ReverseScan(startKey, endKey []byte, limit int, startTS uint64, isoLevel kvrpcpb.IsolationLevel) []Pair
	BatchGet(ks [][]byte, startTS uint64, isoLevel kvrpcpb.IsolationLevel) []Pair
	Prewrite(mutations []*kvrpcpb.Mutation, primary []byte, startTS uint64, ttl uint64) []error
	Commit(keys [][]byte, startTS, commitTS uint64) error
	Rollback(keys [][]byte, startTS uint64) error
	Cleanup(key []byte, startTS uint64) error
	ScanLock(startKey, endKey []byte, maxTS uint64) ([]*kvrpcpb.LockInfo, error)
	ResolveLock(startKey, endKey []byte, startTS, commitTS uint64) error
	BatchResolveLock(startKey, endKey []byte, txnInfos map[uint64]uint64) error
	DeleteRange(startKey, endKey []byte) error
	Close() error
}

// RawKV is a key-value storage. MVCCStore can be implemented upon it with timestamp encoded into key.
type RawKV interface {
	RawGet(key []byte) []byte
	RawBatchGet(keys [][]byte) [][]byte
	RawScan(startKey, endKey []byte, limit int) []Pair
	RawPut(key, value []byte)
	RawBatchPut(keys, values [][]byte)
	RawDelete(key []byte)
	RawBatchDelete(keys [][]byte)
	RawDeleteRange(startKey, endKey []byte)
}

// MVCCDebugger is for debugging.
type MVCCDebugger interface {
	MvccGetByStartTS(startKey, endKey []byte, starTS uint64) (*kvrpcpb.MvccInfo, []byte)
	MvccGetByKey(key []byte) *kvrpcpb.MvccInfo
}

// Pair is a KV pair read from MvccStore or an error if any occurs.
type Pair struct {
	Key   []byte
	Value []byte
	Err   error
}

func regionContains(startKey []byte, endKey []byte, key []byte) bool {
	return bytes.Compare(startKey, key) <= 0 &&
		(bytes.Compare(key, endKey) < 0 || len(endKey) == 0)
}

// MvccKey is the encoded key type.
// On TiKV, keys are encoded before they are saved into storage engine.
type MvccKey []byte

// NewMvccKey encodes a key into MvccKey.
func NewMvccKey(key []byte) MvccKey {
	if len(key) == 0 {
		return nil
	}
	return codec.EncodeBytes(nil, key)
}

// Raw decodes a MvccKey to original key.
func (key MvccKey) Raw() []byte {
	if len(key) == 0 {
		return nil
	}
	_, k, err := codec.DecodeBytes(key, nil)
	if err != nil {
		panic(err)
	}
	return k
}
