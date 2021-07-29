// Copyright 2015 The etcd Authors
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
	"bytes"
	"math"
	"sync"
	"sync/atomic"
	"time"

	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

type BucketID int

type Bucket interface {
	// ID returns a unique identifier of a bucket.
	// The id must NOT be persisted and can be used as lightweight identificator
	// in the in-memory maps.
	ID() BucketID
	Name() []byte
	// String implements Stringer (human readable name).
	String() string

	// IsSafeRangeBucket is a hack to avoid inadvertently reading duplicate keys;
	// overwrites on a bucket should only fetch with limit=1, but safeRangeBucket
	// is known to never overwrite any key so range is safe.
	IsSafeRangeBucket() bool
}

var KeyBucket Bucket

func SetKeyBucket(bucket Bucket) {
	KeyBucket = bucket
}

type BatchTx interface {
	ReadTx
	UnsafeCreateBucket(bucket Bucket)
	UnsafeDeleteBucket(bucket Bucket)
	UnsafePut(bucket Bucket, key []byte, value []byte)
	UnsafeSeqPut(bucket Bucket, key []byte, value []byte)
	UnsafeDelete(bucket Bucket, key []byte)
	// Commit commits a previous tx and begins a new writable one.
	Commit()
	// CommitAndStop commits the previous tx and does not create a new one.
	CommitAndStop()
}

type BatchTxAsync interface {
	BatchTx
	UnsafePutAsync(bucket Bucket, key []byte, value []byte)
	UnsafeSeqPutAsync(bucket Bucket, key []byte, value []byte)

	Flash2ReadTx(readTx ReadTx)

	LockAsync()
	UnlockAsync()
}

type batchTx struct {
	sync.Mutex
	tx      *bolt.Tx
	backend *backend

	pending int
}

func (t *batchTx) Lock() {
	t.Mutex.Lock()
}

func (t *batchTx) Unlock() {
	if t.pending >= t.backend.batchLimit {
		t.commit(false)
	}
	t.Mutex.Unlock()
}

// BatchTx interface embeds ReadTx interface. But RLock() and RUnlock() do not
// have appropriate semantics in BatchTx interface. Therefore should not be called.
// TODO: might want to decouple ReadTx and BatchTx

func (t *batchTx) RLock() {
	panic("unexpected RLock")
}

func (t *batchTx) RUnlock() {
	panic("unexpected RUnlock")
}

func (t *batchTx) GetBuffer() interface{} { panic("unexpected batchTx GetBuffer") }

func (t *batchTx) UnsafeCreateBucket(bucket Bucket) {
	_, err := t.tx.CreateBucket(bucket.Name())
	if err != nil && err != bolt.ErrBucketExists {
		t.backend.lg.Fatal(
			"failed to create a bucket",
			zap.Stringer("bucket-name", bucket),
			zap.Error(err),
		)
	}
	t.pending++
}

func (t *batchTx) UnsafeDeleteBucket(bucket Bucket) {
	err := t.tx.DeleteBucket(bucket.Name())
	if err != nil && err != bolt.ErrBucketNotFound {
		t.backend.lg.Fatal(
			"failed to delete a bucket",
			zap.Stringer("bucket-name", bucket),
			zap.Error(err),
		)
	}
	t.pending++
}

// UnsafePut must be called holding the lock on the tx.
func (t *batchTx) UnsafePut(bucket Bucket, key []byte, value []byte) {
	t.unsafePut(bucket, key, value, false)
}

// UnsafeSeqPut must be called holding the lock on the tx.
func (t *batchTx) UnsafeSeqPut(bucket Bucket, key []byte, value []byte) {
	t.unsafePut(bucket, key, value, true)
}

func (t *batchTx) unsafePut(bucket Bucket, key []byte, value []byte, seq bool) {
	t.unsafePutWithoutAddPending(bucket, key, value, seq)
	t.pending++
}

func (t *batchTx) unsafePutWithoutAddPending(bucketType Bucket, key []byte, value []byte, seq bool) {
	bucket := t.tx.Bucket(bucketType.Name())
	if bucket == nil {
		t.backend.lg.Fatal(
			"failed to find a bucket",
			zap.Stringer("bucket-name", bucketType),
			zap.Stack("stack"),
		)
	}
	if seq {
		// it is useful to increase fill percent when the workloads are mostly append-only.
		// this can delay the page split and reduce space usage.
		bucket.FillPercent = 0.9
	}
	if err := bucket.Put(key, value); err != nil {
		t.backend.lg.Fatal(
			"failed to write to a bucket",
			zap.Stringer("bucket-name", bucketType),
			zap.Error(err),
		)
	}
}

// UnsafeRange must be called holding the lock on the tx.
func (t *batchTx) UnsafeRange(bucketType Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	bucket := t.tx.Bucket(bucketType.Name())
	if bucket == nil {
		t.backend.lg.Fatal(
			"failed to find a bucket",
			zap.Stringer("bucket-name", bucketType),
			zap.Stack("stack"),
		)
	}
	return unsafeRange(bucket.Cursor(), key, endKey, limit)
}

func unsafeRange(c *bolt.Cursor, key, endKey []byte, limit int64) (keys [][]byte, vs [][]byte) {
	if limit <= 0 {
		limit = math.MaxInt64
	}
	var isMatch func(b []byte) bool
	if len(endKey) > 0 {
		isMatch = func(b []byte) bool { return bytes.Compare(b, endKey) < 0 }
	} else {
		isMatch = func(b []byte) bool { return bytes.Equal(b, key) }
		limit = 1
	}

	for ck, cv := c.Seek(key); ck != nil && isMatch(ck); ck, cv = c.Next() {
		vs = append(vs, cv)
		keys = append(keys, ck)
		if limit == int64(len(keys)) {
			break
		}
	}
	return keys, vs
}

// UnsafeDelete must be called holding the lock on the tx.
func (t *batchTx) UnsafeDelete(bucketType Bucket, key []byte) {
	bucket := t.tx.Bucket(bucketType.Name())
	if bucket == nil {
		t.backend.lg.Fatal(
			"failed to find a bucket",
			zap.Stringer("bucket-name", bucketType),
			zap.Stack("stack"),
		)
	}
	err := bucket.Delete(key)
	if err != nil {
		t.backend.lg.Fatal(
			"failed to delete a key",
			zap.Stringer("bucket-name", bucketType),
			zap.Error(err),
		)
	}
	t.pending++
}

// UnsafeForEach must be called holding the lock on the tx.
func (t *batchTx) UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error {
	return unsafeForEach(t.tx, bucket, visitor)
}

func unsafeForEach(tx *bolt.Tx, bucket Bucket, visitor func(k, v []byte) error) error {
	if b := tx.Bucket(bucket.Name()); b != nil {
		return b.ForEach(visitor)
	}
	return nil
}

// Commit commits a previous tx and begins a new writable one.
func (t *batchTx) Commit() {
	t.Lock()
	t.commit(false)
	t.Unlock()
}

// CommitAndStop commits the previous tx and does not create a new one.
func (t *batchTx) CommitAndStop() {
	t.Lock()
	t.commit(true)
	t.Unlock()
}

func (t *batchTx) safePending() int {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	return t.pending
}

func (t *batchTx) commit(stop bool) {
	// commit the last tx
	if t.tx != nil {
		if t.pending == 0 && !stop {
			return
		}

		start := time.Now()

		// gofail: var beforeCommit struct{}
		err := t.tx.Commit()
		// gofail: var afterCommit struct{}

		rebalanceSec.Observe(t.tx.Stats().RebalanceTime.Seconds())
		spillSec.Observe(t.tx.Stats().SpillTime.Seconds())
		writeSec.Observe(t.tx.Stats().WriteTime.Seconds())
		commitSec.Observe(time.Since(start).Seconds())
		atomic.AddInt64(&t.backend.commits, 1)

		t.pending = 0
		if err != nil {
			t.backend.lg.Fatal("failed to commit tx", zap.Error(err))
		}
	}
	if !stop {
		t.tx = t.backend.begin(true)
	}
}

type batchTxBuffered struct {
	batchTx
	buf txWriteBuffer
}

func newBatchTxBuffered(backend *backend) *batchTxBuffered {
	tx := &batchTxBuffered{
		batchTx: batchTx{backend: backend},
		buf: txWriteBuffer{
			txBuffer:   txBuffer{make(map[BucketID]*bucketBuffer)},
			bucket2seq: make(map[BucketID]bool),
		},
	}
	tx.Commit()
	return tx
}

func (t *batchTxBuffered) Unlock() {
	if t.pending != 0 {
		t.backend.readTx.Lock() // blocks txReadBuffer for writing.
		t.buf.writeback(t.backend.readTx.buf)
		t.backend.readTx.Unlock()
		if t.pending >= t.backend.batchLimit {
			t.commit(false)
		}
	}
	t.batchTx.Unlock()
}

func (t *batchTxBuffered) Commit() {
	t.Lock()
	t.commit(false)
	t.Unlock()
}

func (t *batchTxBuffered) CommitAndStop() {
	t.Lock()
	t.commit(true)
	t.Unlock()
}

func (t *batchTxBuffered) commit(stop bool) {
	if t.backend.hooks != nil {
		t.backend.hooks.OnPreCommitUnsafe(t)
	}

	// all read txs must be closed to acquire boltdb commit rwlock
	t.backend.readTx.Lock()
	t.unsafeCommit(stop)
	t.backend.readTx.Unlock()
}

func (t *batchTxBuffered) unsafeCommit(stop bool) {
	for bucketID := range t.backend.readTx.buf.buckets {
		if bucketID == KeyBucket.ID() {
			put := func(k, v []byte, needPut bool) error {
				if needPut {
					if seq, ok := t.buf.bucket2seq[bucketID]; ok {
						t.batchTx.unsafePutWithoutAddPending(KeyBucket, k, v, seq)
					} else {
						t.batchTx.unsafePutWithoutAddPending(KeyBucket, k, v, true)
					}
				}
				return nil
			}

			err := t.backend.readTx.buf.ForEachWithNeedPutAsync(KeyBucket, put)
			if err != nil {
				if t.backend.lg != nil {
					t.backend.lg.Fatal("failed to ForEach unsafePut", zap.Error(err))
				}

				return
			}
		}
	}
	if t.backend.readTx.tx != nil {
		// wait all store read transactions using the current boltdb tx to finish,
		// then close the boltdb tx
		go func(tx *bolt.Tx, wg *sync.WaitGroup) {
			wg.Wait()
			if err := tx.Rollback(); err != nil {
				t.backend.lg.Fatal("failed to rollback tx", zap.Error(err))
			}
		}(t.backend.readTx.tx, t.backend.readTx.txWg)
		t.backend.readTx.reset()
		t.backend.readTx.buf.reset()
	}

	t.batchTx.commit(stop)

	if !stop {
		t.backend.readTx.tx = t.backend.begin(false)
	}
}

func (t *batchTxBuffered) UnsafePut(bucket Bucket, key []byte, value []byte) {
	t.batchTx.UnsafePut(bucket, key, value)
	t.buf.put(bucket, key, value)
}

func (t *batchTxBuffered) UnsafeSeqPut(bucket Bucket, key []byte, value []byte) {
	t.batchTx.UnsafeSeqPut(bucket, key, value)
	t.buf.putSeq(bucket, key, value)
}

type batchTxBufferedAsync struct {
	*batchTxBuffered
	asyncBuf        txWriteBuffer
	asyncBufLock    sync.Mutex
	asyncBufPending int
}

func newBatchTxBufferedAsync(backend *backend) *batchTxBufferedAsync {
	twb := txWriteBuffer{
		txBuffer:   txBuffer{make(map[BucketID]*bucketBuffer)},
		bucket2seq: make(map[BucketID]bool),
	}
	tx := &batchTxBufferedAsync{}
	tx.batchTxBuffered = newBatchTxBuffered(backend)
	tx.asyncBuf = twb

	return tx
}

func (t *batchTxBufferedAsync) safePending() int {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	t.asyncBufLock.Lock()
	defer t.asyncBufLock.Unlock()

	return t.pending + t.asyncBufPending
}

func (t *batchTxBufferedAsync) LockAsync() {
	t.asyncBufLock.Lock()
}

func (t *batchTxBufferedAsync) UnlockAsync() {
	if t.asyncBufPending != 0 {
		t.backend.readTx.Lock() // blocks txReadBuffer for writing.
		t.asyncBuf.writeback(t.backend.readTx.buf)
		t.backend.readTx.Unlock()
	}
	t.asyncBufLock.Unlock()
}

func (t *batchTxBufferedAsync) Commit() {
	t.Lock()
	t.commit(false)
	t.Unlock()
}

func (t *batchTxBufferedAsync) CommitAndStop() {
	t.Lock()
	t.commit(true)
	t.Unlock()
}

func (t *batchTxBufferedAsync) commit(stop bool) {
	t.commitAndPut(stop)
}

func (t *batchTxBufferedAsync) commitAndPut(stop bool) {
	// all read txs must be closed to acquire boltdb commit rwlock
	t.asyncBufLock.Lock()
	t.backend.readTx.Lock()

	var index uint64
	var term uint64
	if t.backend.hooks != nil {
		index, term = t.backend.hooks.OnPreCommitIndexAndTermUnsafe()
	}

	t.pending += t.asyncBufPending
	t.asyncBufPending = 0
	t.backend.readTx.committingBuf.reset()
	bufVersion := t.backend.readTx.buf.bufVersion
	t.backend.readTx.committingBuf, t.backend.readTx.buf = t.backend.readTx.buf, t.backend.readTx.committingBuf
	t.backend.readTx.buf.bufVersion = bufVersion

	t.backend.readTx.Unlock()
	t.asyncBufLock.Unlock()

	if t.backend.hooks != nil {
		t.backend.hooks.OnPreCommitWithIndexAndTermUnsafe(t, index, term)
	}

	commitDone := make(chan interface{}, 1)

	go t.unsafeCommitAndPut(stop, commitDone)

	timeOut := time.NewTimer(500 * time.Millisecond)
	defer timeOut.Stop()
	select {
	case <-commitDone:
		var rtx *bolt.Tx
		if !stop {
			rtx = t.backend.begin(false)
		}

		t.backend.readTx.Lock()
		t.unsafeRollbackTx()
		if !stop {
			t.backend.readTx.tx = rtx
		}
		t.backend.readTx.Unlock()
	case <-timeOut.C:
		t.backend.lg.Warn("commit too long, lock range and put")
		t.backend.readTx.Lock()
		t.unsafeRollbackTx()
		<-commitDone

		if !stop {
			t.backend.readTx.tx = t.backend.begin(false)
		}
		t.backend.readTx.Unlock()
	}
}

func (t *batchTxBufferedAsync) unsafeRollbackTx() {
	if t.backend.readTx.tx != nil {
		// wait all store read transactions using the current boltdb tx to finish,
		// then close the boltdb tx
		go func(tx *bolt.Tx, wg *sync.WaitGroup) {
			wg.Wait()
			if err := tx.Rollback(); err != nil {
				if t.backend.lg != nil {
					t.backend.lg.Fatal("failed to rollback tx", zap.Error(err))
				}
			}
		}(t.backend.readTx.tx, t.backend.readTx.txWg)
		t.backend.readTx.reset()
		t.backend.readTx.committingBuf.reset()
	}
}

func (t *batchTxBufferedAsync) unsafeCommitAndPut(stop bool, commitDone chan interface{}) {
	if t.pending != 0 {
		for bucketID := range t.backend.readTx.committingBuf.buckets {
			if bucketID == KeyBucket.ID() {
				put := func(k, v []byte, needPut bool) error {
					if needPut {
						if seq, ok := t.buf.bucket2seq[bucketID]; ok {
							t.batchTx.unsafePutWithoutAddPending(KeyBucket, k, v, seq)
						} else {
							t.batchTx.unsafePutWithoutAddPending(KeyBucket, k, v, true)
						}

					}
					return nil
				}

				err := t.backend.readTx.committingBuf.ForEachWithNeedPutAsync(KeyBucket, put)
				if err != nil {
					if t.backend.lg != nil {
						t.backend.lg.Fatal("failed to ForEach unsafePut", zap.Error(err))
					}

					return
				}
			}
		}
	}

	t.batchTx.commit(stop)

	close(commitDone)
}

func (t *batchTxBufferedAsync) UnsafePutAsync(bucket Bucket, key []byte, value []byte) {
	t.asyncBuf.putAsync(bucket, key, value)
	t.asyncBufPending++
}

func (t *batchTxBufferedAsync) UnsafeSeqPutAsync(bucket Bucket, key []byte, value []byte) {
	t.asyncBuf.putSeqAsync(bucket, key, value)
	t.asyncBufPending++
}

func (t *batchTxBufferedAsync) Flash2ReadTx(readTx ReadTx) {
	buf := readTx.GetBuffer().(*txReadBuffer)
	t.asyncBuf.writebackNoReset(buf)
}
