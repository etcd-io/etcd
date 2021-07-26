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
	"encoding/binary"
	"math"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// IsSafeRangeBucket is a hack to avoid inadvertently reading duplicate keys;
// overwrites on a bucket should only fetch with limit=1, but IsSafeRangeBucket
// is known to never overwrite any key so range is safe.

type ReadTx interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()

	UnsafeRange(bucket Bucket, key, endKey []byte, limit int64) (keys [][]byte, vals [][]byte)
	UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error
}

// Base type for readTx and concurrentReadTx to eliminate duplicate functions between these
type baseReadTx struct {
	// mu protects accesses to the txReadBuffer
	mu           *sync.RWMutex
	buf          *txReadBuffer
	bufferNoCopy bool

	// TODO: group and encapsulate {txMu, tx, buckets, txWg}, as they share the same lifecycle.
	// txMu protects accesses to buckets and tx on Range requests.
	txMu    *sync.RWMutex
	tx      *bolt.Tx
	buckets map[BucketID]*bolt.Bucket
	// txWg protects tx from being rolled back at the end of a batch interval until all reads using this tx are done.
	txWg *sync.WaitGroup
}

func (baseReadTx *baseReadTx) UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error {
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
	if err := baseReadTx.buf.ForEach(bucket, getDups); err != nil {
		return err
	}
	baseReadTx.txMu.Lock()
	err := unsafeForEach(baseReadTx.tx, bucket, visitNoDup)
	baseReadTx.txMu.Unlock()
	if err != nil {
		return err
	}
	return baseReadTx.buf.ForEach(bucket, visitor)
}

func bytesToRev(bytes []byte) int64 {
	if len(bytes) <= 8 {
		return math.MaxInt64
	}
	return int64(binary.BigEndian.Uint64(bytes[0:8]))
}

func (baseReadTx *baseReadTx) UnsafeRange(bucketType Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	rangeTotalCounter.Inc()
	if endKey == nil {
		// forbid duplicates for single keys
		limit = 1
	}
	if limit <= 0 {
		limit = math.MaxInt64
	}
	if limit > 1 && !bucketType.IsSafeRangeBucket() {
		panic("do not use UnsafeRange on non-keys bucket")
	}

	// ConcurrentReadTxNoCopyMode only for Key bucket
	if baseReadTx.bufferNoCopy && endKey == nil {
		// Single key search. Key exists either bolt db or read buffer
		k1, v1 := baseReadTx.unsafeRangeDB(bucketType, key, endKey, limit)
		if len(k1) != 0 {
			return k1, v1
		}
		k2, v2 := baseReadTx.unsafeRangeBuff(bucketType, key, endKey, limit-int64(len(k1)))
		if len(k2) != 0 {
			// If rate of buffRangeHitCounter/rangeTotalCounter is high, we should not use ConcurrentReadTxNoCopyMode
			buffRangeHitCounter.Inc()
		}
		return k2, v2
	}

	// Range keys search. Need searching buff first, to make assure result is from newest to old with limit.
	k1, v1 := baseReadTx.unsafeRangeBuff(bucketType, key, endKey, limit)
	if int64(len(k1)) == limit {
		return k1, v1
	}
	k2, v2 := baseReadTx.unsafeRangeDB(bucketType, key, endKey, limit-int64(len(k1)))
	return append(k1, k2...), append(v1, v2...)
}

func (baseReadTx *baseReadTx) unsafeRangeBuff(bucketType Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	keyRev := int64(0)
	endKeyRev := int64(0)
	if bucketType.IsSafeRangeBucket() {
		keyRev = bytesToRev(key)
		if endKey != nil {
			endKeyRev = bytesToRev(endKey)
		}
	}

	var keys [][]byte
	var vals [][]byte
	if baseReadTx.bufferNoCopy {
		baseReadTx.mu.RLock()
		defer baseReadTx.mu.RUnlock()
	}

	// Meta data search, or key search in buffer scope
	if !bucketType.IsSafeRangeBucket() || keyRev >= baseReadTx.buf.bufMinRev || endKeyRev >= baseReadTx.buf.bufMinRev {
		keys, vals = baseReadTx.buf.Range(bucketType, key, endKey, limit)
		if int64(len(keys)) == limit {
			return keys, vals
		}
	}

	return keys, vals
}

func (baseReadTx *baseReadTx) unsafeRangeDB(bucketType Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	// find/cache bucket
	bn := bucketType.ID()
	baseReadTx.txMu.RLock()
	bucket, ok := baseReadTx.buckets[bn]
	baseReadTx.txMu.RUnlock()
	lockHeld := false
	if !ok {
		baseReadTx.txMu.Lock()
		lockHeld = true
		bucket = baseReadTx.tx.Bucket(bucketType.Name())
		baseReadTx.buckets[bn] = bucket
	}

	// ignore missing bucket since may have been created in this batch
	if bucket == nil {
		if lockHeld {
			baseReadTx.txMu.Unlock()
		}
		return nil, nil
	}
	if !lockHeld {
		baseReadTx.txMu.Lock()
	}
	c := bucket.Cursor()
	baseReadTx.txMu.Unlock()

	keys, vals := unsafeRange(c, key, endKey, limit)
	return keys, vals
}

type readTx struct {
	baseReadTx
}

func (rt *readTx) Lock()    { rt.mu.Lock() }
func (rt *readTx) Unlock()  { rt.mu.Unlock() }
func (rt *readTx) RLock()   { rt.mu.RLock() }
func (rt *readTx) RUnlock() { rt.mu.RUnlock() }

func (rt *readTx) reset() {
	rt.buckets = make(map[BucketID]*bolt.Bucket)
	rt.tx = nil
	rt.txWg = new(sync.WaitGroup)
}

type concurrentReadTx struct {
	baseReadTx
}

func (rt *concurrentReadTx) Lock()   {}
func (rt *concurrentReadTx) Unlock() {}

// RLock is no-op. concurrentReadTx does not need to be locked after it is created.
func (rt *concurrentReadTx) RLock() {}

// RUnlock signals the end of concurrentReadTx.
func (rt *concurrentReadTx) RUnlock() { rt.txWg.Done() }
