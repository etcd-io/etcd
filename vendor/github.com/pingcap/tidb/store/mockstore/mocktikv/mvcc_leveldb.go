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

package mocktikv

import (
	"bytes"
	"math"
	"sync"

	"github.com/pingcap/errors"
	"github.com/pingcap/goleveldb/leveldb"
	"github.com/pingcap/goleveldb/leveldb/iterator"
	"github.com/pingcap/goleveldb/leveldb/opt"
	"github.com/pingcap/goleveldb/leveldb/storage"
	"github.com/pingcap/goleveldb/leveldb/util"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/util/codec"
	log "github.com/sirupsen/logrus"
)

// MVCCLevelDB implements the MVCCStore interface.
type MVCCLevelDB struct {
	// Key layout:
	// ...
	// Key_lock        -- (0)
	// Key_verMax      -- (1)
	// ...
	// Key_ver+1       -- (2)
	// Key_ver         -- (3)
	// Key_ver-1       -- (4)
	// ...
	// Key_0           -- (5)
	// NextKey_lock    -- (6)
	// NextKey_verMax  -- (7)
	// ...
	// NextKey_ver+1   -- (8)
	// NextKey_ver     -- (9)
	// NextKey_ver-1   -- (10)
	// ...
	// NextKey_0       -- (11)
	// ...
	// EOF
	db *leveldb.DB
	// leveldb can not guarantee multiple operations to be atomic, for example, read
	// then write, another write may happen during it, so this lock is necessory.
	mu sync.RWMutex
}

const lockVer uint64 = math.MaxUint64

// ErrInvalidEncodedKey describes parsing an invalid format of EncodedKey.
var ErrInvalidEncodedKey = errors.New("invalid encoded key")

// mvccEncode returns the encoded key.
func mvccEncode(key []byte, ver uint64) []byte {
	b := codec.EncodeBytes(nil, key)
	ret := codec.EncodeUintDesc(b, ver)
	return ret
}

// mvccDecode parses the origin key and version of an encoded key, if the encoded key is a meta key,
// just returns the origin key.
func mvccDecode(encodedKey []byte) ([]byte, uint64, error) {
	// Skip DataPrefix
	remainBytes, key, err := codec.DecodeBytes(encodedKey, nil)
	if err != nil {
		// should never happen
		return nil, 0, errors.Trace(err)
	}
	// if it's meta key
	if len(remainBytes) == 0 {
		return key, 0, nil
	}
	var ver uint64
	remainBytes, ver, err = codec.DecodeUintDesc(remainBytes)
	if err != nil {
		// should never happen
		return nil, 0, errors.Trace(err)
	}
	if len(remainBytes) != 0 {
		return nil, 0, ErrInvalidEncodedKey
	}
	return key, ver, nil
}

// MustNewMVCCStore is used for testing, use NewMVCCLevelDB instead.
func MustNewMVCCStore() MVCCStore {
	mvccStore, err := NewMVCCLevelDB("")
	if err != nil {
		panic(err)
	}
	return mvccStore
}

// NewMVCCLevelDB returns a new MVCCLevelDB object.
func NewMVCCLevelDB(path string) (*MVCCLevelDB, error) {
	var (
		d   *leveldb.DB
		err error
	)
	if path == "" {
		d, err = leveldb.Open(storage.NewMemStorage(), nil)
	} else {
		d, err = leveldb.OpenFile(path, &opt.Options{BlockCacheCapacity: 600 * 1024 * 1024})
	}

	return &MVCCLevelDB{db: d}, errors.Trace(err)
}

// Iterator wraps iterator.Iterator to provide Valid() method.
type Iterator struct {
	iterator.Iterator
	valid bool
}

// Next moves the iterator to the next key/value pair.
func (iter *Iterator) Next() {
	iter.valid = iter.Iterator.Next()
}

// Valid returns whether the iterator is exhausted.
func (iter *Iterator) Valid() bool {
	return iter.valid
}

func newIterator(db *leveldb.DB, slice *util.Range) *Iterator {
	iter := &Iterator{db.NewIterator(slice, nil), true}
	iter.Next()
	return iter
}

func newScanIterator(db *leveldb.DB, startKey, endKey []byte) (*Iterator, []byte, error) {
	var start, end []byte
	if len(startKey) > 0 {
		start = mvccEncode(startKey, lockVer)
	}
	if len(endKey) > 0 {
		end = mvccEncode(endKey, lockVer)
	}
	iter := newIterator(db, &util.Range{
		Start: start,
		Limit: end,
	})
	// newScanIterator must handle startKey is nil, in this case, the real startKey
	// should be change the frist key of the store.
	if len(startKey) == 0 && iter.Valid() {
		key, _, err := mvccDecode(iter.Key())
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		startKey = key
	}
	return iter, startKey, nil
}

// iterDecoder tries to decode an Iterator value.
// If current iterator value can be decoded by this decoder, store the value and call iter.Next(),
// Otherwise current iterator is not touched and returns false.
type iterDecoder interface {
	Decode(iter *Iterator) (bool, error)
}

type lockDecoder struct {
	lock      mvccLock
	expectKey []byte
}

// lockDecoder decodes the lock value if current iterator is at expectKey::lock.
func (dec *lockDecoder) Decode(iter *Iterator) (bool, error) {
	if iter.Error() != nil || !iter.Valid() {
		return false, iter.Error()
	}

	iterKey := iter.Key()
	key, ver, err := mvccDecode(iterKey)
	if err != nil {
		return false, errors.Trace(err)
	}
	if !bytes.Equal(key, dec.expectKey) {
		return false, nil
	}
	if ver != lockVer {
		return false, nil
	}

	var lock mvccLock
	err = lock.UnmarshalBinary(iter.Value())
	if err != nil {
		return false, errors.Trace(err)
	}
	dec.lock = lock
	iter.Next()
	return true, nil
}

type valueDecoder struct {
	value     mvccValue
	expectKey []byte
}

// valueDecoder decodes a mvcc value if iter key is expectKey.
func (dec *valueDecoder) Decode(iter *Iterator) (bool, error) {
	if iter.Error() != nil || !iter.Valid() {
		return false, iter.Error()
	}

	key, ver, err := mvccDecode(iter.Key())
	if err != nil {
		return false, errors.Trace(err)
	}
	if !bytes.Equal(key, dec.expectKey) {
		return false, nil
	}
	if ver == lockVer {
		return false, nil
	}

	var value mvccValue
	err = value.UnmarshalBinary(iter.Value())
	if err != nil {
		return false, errors.Trace(err)
	}
	dec.value = value
	iter.Next()
	return true, nil
}

type skipDecoder struct {
	currKey []byte
}

// skipDecoder skips the iterator as long as its key is currKey, the new key would be stored.
func (dec *skipDecoder) Decode(iter *Iterator) (bool, error) {
	if iter.Error() != nil {
		return false, iter.Error()
	}
	for iter.Valid() {
		key, _, err := mvccDecode(iter.Key())
		if err != nil {
			return false, errors.Trace(err)
		}
		if !bytes.Equal(key, dec.currKey) {
			dec.currKey = key
			return true, nil
		}
		iter.Next()
	}
	return false, nil
}

type mvccEntryDecoder struct {
	expectKey []byte
	// Just values and lock is valid.
	mvccEntry
}

// mvccEntryDecoder decodes a mvcc entry.
func (dec *mvccEntryDecoder) Decode(iter *Iterator) (bool, error) {
	ldec := lockDecoder{expectKey: dec.expectKey}
	ok, err := ldec.Decode(iter)
	if err != nil {
		return ok, errors.Trace(err)
	}
	if ok {
		dec.mvccEntry.lock = &ldec.lock
	}
	for iter.Valid() {
		vdec := valueDecoder{expectKey: dec.expectKey}
		ok, err = vdec.Decode(iter)
		if err != nil {
			return ok, errors.Trace(err)
		}
		if !ok {
			break
		}
		dec.mvccEntry.values = append(dec.mvccEntry.values, vdec.value)
	}
	succ := dec.mvccEntry.lock != nil || len(dec.mvccEntry.values) > 0
	return succ, nil
}

// Get implements the MVCCStore interface.
// key cannot be nil or []byte{}
func (mvcc *MVCCLevelDB) Get(key []byte, startTS uint64, isoLevel kvrpcpb.IsolationLevel) ([]byte, error) {
	mvcc.mu.RLock()
	defer mvcc.mu.RUnlock()

	return mvcc.getValue(key, startTS, isoLevel)
}

func (mvcc *MVCCLevelDB) getValue(key []byte, startTS uint64, isoLevel kvrpcpb.IsolationLevel) ([]byte, error) {
	startKey := mvccEncode(key, lockVer)
	iter := newIterator(mvcc.db, &util.Range{
		Start: startKey,
	})
	defer iter.Release()

	return getValue(iter, key, startTS, isoLevel)
}

func getValue(iter *Iterator, key []byte, startTS uint64, isoLevel kvrpcpb.IsolationLevel) ([]byte, error) {
	dec1 := lockDecoder{expectKey: key}
	ok, err := dec1.Decode(iter)
	if ok && isoLevel == kvrpcpb.IsolationLevel_SI {
		startTS, err = dec1.lock.check(startTS, key)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	dec2 := valueDecoder{expectKey: key}
	for iter.Valid() {
		ok, err := dec2.Decode(iter)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !ok {
			break
		}

		value := &dec2.value
		if value.valueType == typeRollback {
			continue
		}
		// Read the first committed value that can be seen at startTS.
		if value.commitTS <= startTS {
			if value.valueType == typeDelete {
				return nil, nil
			}
			return value.value, nil
		}
	}
	return nil, nil
}

// BatchGet implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) BatchGet(ks [][]byte, startTS uint64, isoLevel kvrpcpb.IsolationLevel) []Pair {
	mvcc.mu.RLock()
	defer mvcc.mu.RUnlock()

	var pairs []Pair
	for _, k := range ks {
		v, err := mvcc.getValue(k, startTS, isoLevel)
		if v == nil && err == nil {
			continue
		}
		pairs = append(pairs, Pair{
			Key:   k,
			Value: v,
			Err:   errors.Trace(err),
		})
	}
	return pairs
}

// Scan implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) Scan(startKey, endKey []byte, limit int, startTS uint64, isoLevel kvrpcpb.IsolationLevel) []Pair {
	mvcc.mu.RLock()
	defer mvcc.mu.RUnlock()

	iter, currKey, err := newScanIterator(mvcc.db, startKey, endKey)
	defer iter.Release()
	if err != nil {
		log.Error("scan new iterator fail:", errors.ErrorStack(err))
		return nil
	}

	ok := true
	var pairs []Pair
	for len(pairs) < limit && ok {
		value, err := getValue(iter, currKey, startTS, isoLevel)
		if err != nil {
			pairs = append(pairs, Pair{
				Key: currKey,
				Err: errors.Trace(err),
			})
		}
		if value != nil {
			pairs = append(pairs, Pair{
				Key:   currKey,
				Value: value,
			})
		}

		skip := skipDecoder{currKey}
		ok, err = skip.Decode(iter)
		if err != nil {
			log.Error("seek to next key error:", errors.ErrorStack(err))
			break
		}
		currKey = skip.currKey
	}
	return pairs
}

// ReverseScan implements the MVCCStore interface. The search range is [startKey, endKey).
func (mvcc *MVCCLevelDB) ReverseScan(startKey, endKey []byte, limit int, startTS uint64, isoLevel kvrpcpb.IsolationLevel) []Pair {
	mvcc.mu.RLock()
	defer mvcc.mu.RUnlock()

	var mvccEnd []byte
	if len(endKey) != 0 {
		mvccEnd = mvccEncode(endKey, lockVer)
	}
	iter := mvcc.db.NewIterator(&util.Range{
		Limit: mvccEnd,
	}, nil)
	defer iter.Release()

	succ := iter.Last()
	currKey, _, err := mvccDecode(iter.Key())
	// TODO: return error.
	terror.Log(errors.Trace(err))
	helper := reverseScanHelper{
		startTS:  startTS,
		isoLevel: isoLevel,
		currKey:  currKey,
	}

	for succ && len(helper.pairs) < limit {
		key, ver, err := mvccDecode(iter.Key())
		if err != nil {
			break
		}
		if bytes.Compare(key, startKey) < 0 {
			break
		}

		if !bytes.Equal(key, helper.currKey) {
			helper.finishEntry()
			helper.currKey = key
		}
		if ver == lockVer {
			var lock mvccLock
			err = lock.UnmarshalBinary(iter.Value())
			helper.entry.lock = &lock
		} else {
			var value mvccValue
			err = value.UnmarshalBinary(iter.Value())
			helper.entry.values = append(helper.entry.values, value)
		}
		if err != nil {
			log.Error("Unmarshal fail:", errors.Trace(err))
			break
		}
		succ = iter.Prev()
	}
	if len(helper.pairs) < limit {
		helper.finishEntry()
	}
	return helper.pairs
}

type reverseScanHelper struct {
	startTS  uint64
	isoLevel kvrpcpb.IsolationLevel
	currKey  []byte
	entry    mvccEntry
	pairs    []Pair
}

func (helper *reverseScanHelper) finishEntry() {
	reverse(helper.entry.values)
	helper.entry.key = NewMvccKey(helper.currKey)
	val, err := helper.entry.Get(helper.startTS, helper.isoLevel)
	if len(val) != 0 || err != nil {
		helper.pairs = append(helper.pairs, Pair{
			Key:   helper.currKey,
			Value: val,
			Err:   err,
		})
	}
	helper.entry = mvccEntry{}
}

func reverse(values []mvccValue) {
	i, j := 0, len(values)-1
	for i < j {
		values[i], values[j] = values[j], values[i]
		i++
		j--
	}
}

// Prewrite implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) Prewrite(mutations []*kvrpcpb.Mutation, primary []byte, startTS uint64, ttl uint64) []error {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	anyError := false
	batch := &leveldb.Batch{}
	errs := make([]error, 0, len(mutations))
	for _, m := range mutations {
		err := prewriteMutation(mvcc.db, batch, m, startTS, primary, ttl)
		errs = append(errs, err)
		if err != nil {
			anyError = true
		}
	}
	if anyError {
		return errs
	}
	if err := mvcc.db.Write(batch, nil); err != nil {
		return nil
	}

	return errs
}

func prewriteMutation(db *leveldb.DB, batch *leveldb.Batch, mutation *kvrpcpb.Mutation, startTS uint64, primary []byte, ttl uint64) error {
	startKey := mvccEncode(mutation.Key, lockVer)
	iter := newIterator(db, &util.Range{
		Start: startKey,
	})
	defer iter.Release()

	dec := lockDecoder{
		expectKey: mutation.Key,
	}
	ok, err := dec.Decode(iter)
	if err != nil {
		return errors.Trace(err)
	}
	if ok {
		if dec.lock.startTS != startTS {
			return dec.lock.lockErr(mutation.Key)
		}
		return nil
	}

	dec1 := valueDecoder{
		expectKey: mutation.Key,
	}
	ok, err = dec1.Decode(iter)
	if err != nil {
		return errors.Trace(err)
	}
	// Note that it's a write conflict here, even if the value is a rollback one.
	if ok && dec1.value.commitTS >= startTS {
		return ErrRetryable("write conflict")
	}

	lock := mvccLock{
		startTS: startTS,
		primary: primary,
		value:   mutation.Value,
		op:      mutation.GetOp(),
		ttl:     ttl,
	}
	writeKey := mvccEncode(mutation.Key, lockVer)
	writeValue, err := lock.MarshalBinary()
	if err != nil {
		return errors.Trace(err)
	}
	batch.Put(writeKey, writeValue)
	return nil
}

// Commit implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) Commit(keys [][]byte, startTS, commitTS uint64) error {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	batch := &leveldb.Batch{}
	for _, k := range keys {
		err := commitKey(mvcc.db, batch, k, startTS, commitTS)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return mvcc.db.Write(batch, nil)
}

func commitKey(db *leveldb.DB, batch *leveldb.Batch, key []byte, startTS, commitTS uint64) error {
	startKey := mvccEncode(key, lockVer)
	iter := newIterator(db, &util.Range{
		Start: startKey,
	})
	defer iter.Release()

	dec := lockDecoder{
		expectKey: key,
	}
	ok, err := dec.Decode(iter)
	if err != nil {
		return errors.Trace(err)
	}
	if !ok || dec.lock.startTS != startTS {
		// If the lock of this transaction is not found, or the lock is replaced by
		// another transaction, check commit information of this transaction.
		c, ok, err1 := getTxnCommitInfo(iter, key, startTS)
		if err1 != nil {
			return errors.Trace(err1)
		}
		if ok && c.valueType != typeRollback {
			// c.valueType != typeRollback means the transaction is already committed, do nothing.
			return nil
		}
		return ErrRetryable("txn not found")
	}

	if err = commitLock(batch, dec.lock, key, startTS, commitTS); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func commitLock(batch *leveldb.Batch, lock mvccLock, key []byte, startTS, commitTS uint64) error {
	if lock.op != kvrpcpb.Op_Lock {
		var valueType mvccValueType
		if lock.op == kvrpcpb.Op_Put {
			valueType = typePut
		} else {
			valueType = typeDelete
		}
		value := mvccValue{
			valueType: valueType,
			startTS:   startTS,
			commitTS:  commitTS,
			value:     lock.value,
		}
		writeKey := mvccEncode(key, commitTS)
		writeValue, err := value.MarshalBinary()
		if err != nil {
			return errors.Trace(err)
		}
		batch.Put(writeKey, writeValue)
	}
	batch.Delete(mvccEncode(key, lockVer))
	return nil
}

// Rollback implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) Rollback(keys [][]byte, startTS uint64) error {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	batch := &leveldb.Batch{}
	for _, k := range keys {
		err := rollbackKey(mvcc.db, batch, k, startTS)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return mvcc.db.Write(batch, nil)
}

func rollbackKey(db *leveldb.DB, batch *leveldb.Batch, key []byte, startTS uint64) error {
	startKey := mvccEncode(key, lockVer)
	iter := newIterator(db, &util.Range{
		Start: startKey,
	})
	defer iter.Release()

	if iter.Valid() {
		dec := lockDecoder{
			expectKey: key,
		}
		ok, err := dec.Decode(iter)
		if err != nil {
			return errors.Trace(err)
		}
		// If current transaction's lock exist.
		if ok && dec.lock.startTS == startTS {
			if err = rollbackLock(batch, dec.lock, key, startTS); err != nil {
				return errors.Trace(err)
			}
			return nil
		}

		// If current transaction's lock not exist.
		// If commit info of current transaction exist.
		c, ok, err := getTxnCommitInfo(iter, key, startTS)
		if err != nil {
			return errors.Trace(err)
		}
		if ok {
			// If current transaction is already committed.
			if c.valueType != typeRollback {
				return ErrAlreadyCommitted(c.commitTS)
			}
			// If current transaction is already rollback.
			return nil
		}
	}

	// If current transaction is not prewritted before.
	value := mvccValue{
		valueType: typeRollback,
		startTS:   startTS,
		commitTS:  startTS,
	}
	writeKey := mvccEncode(key, startTS)
	writeValue, err := value.MarshalBinary()
	if err != nil {
		return errors.Trace(err)
	}
	batch.Put(writeKey, writeValue)
	return nil
}

func rollbackLock(batch *leveldb.Batch, lock mvccLock, key []byte, startTS uint64) error {
	tomb := mvccValue{
		valueType: typeRollback,
		startTS:   startTS,
		commitTS:  startTS,
	}
	writeKey := mvccEncode(key, startTS)
	writeValue, err := tomb.MarshalBinary()
	if err != nil {
		return errors.Trace(err)
	}
	batch.Put(writeKey, writeValue)
	batch.Delete(mvccEncode(key, lockVer))
	return nil
}

func getTxnCommitInfo(iter *Iterator, expectKey []byte, startTS uint64) (mvccValue, bool, error) {
	for iter.Valid() {
		dec := valueDecoder{
			expectKey: expectKey,
		}
		ok, err := dec.Decode(iter)
		if err != nil || !ok {
			return mvccValue{}, ok, errors.Trace(err)
		}

		if dec.value.startTS == startTS {
			return dec.value, true, nil
		}
	}
	return mvccValue{}, false, nil
}

// Cleanup implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) Cleanup(key []byte, startTS uint64) error {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	batch := &leveldb.Batch{}
	err := rollbackKey(mvcc.db, batch, key, startTS)
	if err != nil {
		return errors.Trace(err)
	}
	return mvcc.db.Write(batch, nil)
}

// ScanLock implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) ScanLock(startKey, endKey []byte, maxTS uint64) ([]*kvrpcpb.LockInfo, error) {
	mvcc.mu.RLock()
	defer mvcc.mu.RUnlock()

	iter, currKey, err := newScanIterator(mvcc.db, startKey, endKey)
	defer iter.Release()
	if err != nil {
		return nil, errors.Trace(err)
	}

	var locks []*kvrpcpb.LockInfo
	for iter.Valid() {
		dec := lockDecoder{expectKey: currKey}
		ok, err := dec.Decode(iter)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if ok && dec.lock.startTS <= maxTS {
			locks = append(locks, &kvrpcpb.LockInfo{
				PrimaryLock: dec.lock.primary,
				LockVersion: dec.lock.startTS,
				Key:         currKey,
			})
		}

		skip := skipDecoder{currKey: currKey}
		_, err = skip.Decode(iter)
		if err != nil {
			return nil, errors.Trace(err)
		}
		currKey = skip.currKey
	}
	return locks, nil
}

// ResolveLock implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) ResolveLock(startKey, endKey []byte, startTS, commitTS uint64) error {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	iter, currKey, err := newScanIterator(mvcc.db, startKey, endKey)
	defer iter.Release()
	if err != nil {
		return errors.Trace(err)
	}

	batch := &leveldb.Batch{}
	for iter.Valid() {
		dec := lockDecoder{expectKey: currKey}
		ok, err := dec.Decode(iter)
		if err != nil {
			return errors.Trace(err)
		}
		if ok && dec.lock.startTS == startTS {
			if commitTS > 0 {
				err = commitLock(batch, dec.lock, currKey, startTS, commitTS)
			} else {
				err = rollbackLock(batch, dec.lock, currKey, startTS)
			}
			if err != nil {
				return errors.Trace(err)
			}
		}

		skip := skipDecoder{currKey: currKey}
		_, err = skip.Decode(iter)
		if err != nil {
			return errors.Trace(err)
		}
		currKey = skip.currKey
	}
	return mvcc.db.Write(batch, nil)
}

// BatchResolveLock implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) BatchResolveLock(startKey, endKey []byte, txnInfos map[uint64]uint64) error {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	iter, currKey, err := newScanIterator(mvcc.db, startKey, endKey)
	defer iter.Release()
	if err != nil {
		return errors.Trace(err)
	}

	batch := &leveldb.Batch{}
	for iter.Valid() {
		dec := lockDecoder{expectKey: currKey}
		ok, err := dec.Decode(iter)
		if err != nil {
			return errors.Trace(err)
		}
		if ok {
			if commitTS, ok := txnInfos[dec.lock.startTS]; ok {
				if commitTS > 0 {
					err = commitLock(batch, dec.lock, currKey, dec.lock.startTS, commitTS)
				} else {
					err = rollbackLock(batch, dec.lock, currKey, dec.lock.startTS)
				}
				if err != nil {
					return errors.Trace(err)
				}
			}
		}

		skip := skipDecoder{currKey: currKey}
		_, err = skip.Decode(iter)
		if err != nil {
			return errors.Trace(err)
		}
		currKey = skip.currKey
	}
	return mvcc.db.Write(batch, nil)
}

// DeleteRange implements the MVCCStore interface.
func (mvcc *MVCCLevelDB) DeleteRange(startKey, endKey []byte) error {
	return mvcc.doRawDeleteRange(codec.EncodeBytes(nil, startKey), codec.EncodeBytes(nil, endKey))
}

// Close calls leveldb's Close to free resources.
func (mvcc *MVCCLevelDB) Close() error {
	return mvcc.db.Close()
}

// RawPut implements the RawKV interface.
func (mvcc *MVCCLevelDB) RawPut(key, value []byte) {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	if value == nil {
		value = []byte{}
	}
	terror.Log(mvcc.db.Put(key, value, nil))
}

// RawBatchPut implements the RawKV interface
func (mvcc *MVCCLevelDB) RawBatchPut(keys, values [][]byte) {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	batch := &leveldb.Batch{}
	for i, key := range keys {
		value := values[i]
		if value == nil {
			value = []byte{}
		}
		batch.Put(key, value)
	}
	terror.Log(mvcc.db.Write(batch, nil))
}

// RawGet implements the RawKV interface.
func (mvcc *MVCCLevelDB) RawGet(key []byte) []byte {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	ret, err := mvcc.db.Get(key, nil)
	terror.Log(err)
	return ret
}

// RawBatchGet implements the RawKV interface.
func (mvcc *MVCCLevelDB) RawBatchGet(keys [][]byte) [][]byte {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	var values [][]byte
	for _, key := range keys {
		value, err := mvcc.db.Get(key, nil)
		terror.Log(err)
		values = append(values, value)
	}
	return values
}

// RawDelete implements the RawKV interface.
func (mvcc *MVCCLevelDB) RawDelete(key []byte) {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	terror.Log(mvcc.db.Delete(key, nil))
}

// RawBatchDelete implements the RawKV interface.
func (mvcc *MVCCLevelDB) RawBatchDelete(keys [][]byte) {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	batch := &leveldb.Batch{}
	for _, key := range keys {
		batch.Delete(key)
	}
	terror.Log(mvcc.db.Write(batch, nil))
}

// RawScan implements the RawKV interface.
func (mvcc *MVCCLevelDB) RawScan(startKey, endKey []byte, limit int) []Pair {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	iter := mvcc.db.NewIterator(&util.Range{
		Start: startKey,
	}, nil)

	var pairs []Pair
	for iter.Next() && len(pairs) < limit {
		key := iter.Key()
		value := iter.Value()
		err := iter.Error()
		if len(endKey) > 0 && bytes.Compare(key, endKey) >= 0 {
			break
		}
		pairs = append(pairs, Pair{
			Key:   append([]byte{}, key...),
			Value: append([]byte{}, value...),
			Err:   err,
		})
	}
	return pairs
}

// RawDeleteRange implements the RawKV interface.
func (mvcc *MVCCLevelDB) RawDeleteRange(startKey, endKey []byte) {
	terror.Log(mvcc.doRawDeleteRange(startKey, endKey))
}

// doRawDeleteRange deletes all keys in a range and return the error if any.
func (mvcc *MVCCLevelDB) doRawDeleteRange(startKey, endKey []byte) error {
	mvcc.mu.Lock()
	defer mvcc.mu.Unlock()

	batch := &leveldb.Batch{}

	iter := mvcc.db.NewIterator(&util.Range{
		Start: startKey,
		Limit: endKey,
	}, nil)
	for iter.Next() {
		batch.Delete(iter.Key())
	}

	return mvcc.db.Write(batch, nil)
}
