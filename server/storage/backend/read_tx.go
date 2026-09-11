// Copyright 2017 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"math"
	"sync"

	"go.uber.org/zap"

	bolt "go.etcd.io/bbolt"
)

// IsSafeRangeBucket is a hack to avoid inadvertently reading duplicate keys;
// overwrites on a bucket should only fetch with limit=1, but IsSafeRangeBucket
// is known to never overwrite any key so range is safe.

type ReadTx interface {
	RLock()
	RUnlock()
	UnsafeReader
}

type UnsafeReader interface {
	UnsafeRange(bucket Bucket, key, endKey []byte, limit int64) (keys [][]byte, vals [][]byte)
	UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error
}

type sharedTx interface {
	lock()
	unlock()
	addWait()
	wait()
	finishWait()
	setTx(tx *bolt.Tx)
	bucket(bucketType Bucket) *bolt.Bucket
	reset(lg *zap.Logger)
	clone() sharedTx
}

type sharedBoltTx struct {
	// txMu protects accesses to buckets and tx on Range requests.
	txMu    *sync.RWMutex
	tx      *bolt.Tx
	buckets map[BucketID]*bolt.Bucket
	// txWg protects tx from being rolled back at the end of a batch interval until all reads using this tx are done.
	txWg *sync.WaitGroup
}

func newSharedTx() sharedTx {
	return &sharedBoltTx{
		txMu:    new(sync.RWMutex),
		buckets: make(map[BucketID]*bolt.Bucket),
		txWg:    new(sync.WaitGroup),
	}
}

func (st *sharedBoltTx) lock()             { st.txMu.Lock() }
func (st *sharedBoltTx) unlock()           { st.txMu.Unlock() }
func (st *sharedBoltTx) addWait()          { st.txWg.Add(1) }
func (st *sharedBoltTx) wait()             { st.txWg.Wait() }
func (st *sharedBoltTx) finishWait()       { st.txWg.Done() }
func (st *sharedBoltTx) setTx(tx *bolt.Tx) { st.tx = tx }

func (st *sharedBoltTx) bucket(bucketType Bucket) *bolt.Bucket {
	// find/cache bucket
	bn := bucketType.ID()
	st.txMu.RLock()
	bucket, ok := st.buckets[bn]
	st.txMu.RUnlock()
	if !ok {
		st.lock()
		bucket = st.tx.Bucket(bucketType.Name())
		st.buckets[bn] = bucket
		st.unlock()
	}

	return bucket
}

func (st *sharedBoltTx) reset(lg *zap.Logger) {
	if st.tx != nil {
		// wait all store read transactions using the current boltdb tx to finish,
		// then close the boltdb tx
		go func(tx *bolt.Tx, wg *sync.WaitGroup) {
			wg.Wait()
			if err := tx.Rollback(); err != nil {
				lg.Fatal("failed to rollback tx", zap.Error(err))
			}
		}(st.tx, st.txWg)
	}

	st.buckets = make(map[BucketID]*bolt.Bucket)
	st.tx = nil
	st.txWg = new(sync.WaitGroup)
}

func (st *sharedBoltTx) clone() sharedTx {
	return &sharedBoltTx{
		txMu:    st.txMu,
		tx:      st.tx,
		buckets: st.buckets,
		txWg:    st.txWg,
	}
}

// Base type for readTx and concurrentReadTx to eliminate duplicate functions between these
type baseReadTx struct {
	// mu protects accesses to the txReadBuffer
	mu  sync.RWMutex
	buf txReadBuffer

	// tx encapsulates the underlying bolt.Tx and its associated locks and buckets.
	tx sharedTx
}

func (baseReadTx *baseReadTx) UnsafeForEach(bucketType Bucket, visitor func(k, v []byte) error) error {
	dups := make(map[string]struct{})
	getDups := func(k, v []byte) error {
		dups[string(k)] = struct{}{}
		return nil
	}
	visitNoDup := func(k, v []byte) error {
		if _, ok := dups[string(k)]; ok {
			return nil
		}
		return visitor(k, v)
	}
	if err := baseReadTx.buf.ForEach(bucketType, getDups); err != nil {
		return err
	}
	bucket := baseReadTx.tx.bucket(bucketType)
	baseReadTx.tx.lock()
	err := unsafeForEach(bucket, visitNoDup)
	baseReadTx.tx.unlock()
	if err != nil {
		return err
	}
	return baseReadTx.buf.ForEach(bucketType, visitor)
}

func (baseReadTx *baseReadTx) UnsafeRange(bucketType Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	if endKey == nil {
		// forbid duplicates for single keys
		limit = 1
	}
	if limit <= 0 {
		limit = math.MaxInt64
	}
	if limit > 1 && !bucketType.IsSafeRangeBucket() {
		panic("do not use unsafeRange on non-keys bucket")
	}
	keys, vals := baseReadTx.buf.Range(bucketType, key, endKey, limit)
	if int64(len(keys)) == limit {
		return keys, vals
	}

	bucket := baseReadTx.tx.bucket(bucketType)

	// ignore missing bucket since may have been created in this batch
	if bucket == nil {
		return keys, vals
	}
	baseReadTx.tx.lock()
	c := bucket.Cursor()
	baseReadTx.tx.unlock()

	k2, v2 := unsafeRange(c, key, endKey, limit-int64(len(keys)))
	return append(k2, keys...), append(v2, vals...)
}

type readTx struct {
	baseReadTx
}

func (rt *readTx) Lock()    { rt.mu.Lock() }
func (rt *readTx) Unlock()  { rt.mu.Unlock() }
func (rt *readTx) RLock()   { rt.mu.RLock() }
func (rt *readTx) RUnlock() { rt.mu.RUnlock() }

func (rt *readTx) reset(lg *zap.Logger) {
	rt.buf.reset()
	rt.tx.reset(lg)
}

type concurrentReadTx struct {
	baseReadTx
}

func (rt *concurrentReadTx) Lock()   {}
func (rt *concurrentReadTx) Unlock() {}

// RLock is no-op. concurrentReadTx does not need to be locked after it is created.
func (rt *concurrentReadTx) RLock() {}

// RUnlock signals the end of concurrentReadTx.
func (rt *concurrentReadTx) RUnlock() { rt.tx.finishWait() }
