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
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	bolt "go.etcd.io/bbolt"

	"go.etcd.io/etcd/server/v3/bucket"
	"go.etcd.io/etcd/server/v3/interfaces"
)

type BatchTx interface {
	Lock()
	Unlock()
	// Commit commits a previous tx and begins a new writable one.
	Commit()
	// CommitAndStop commits the previous tx and does not create a new one.
	CommitAndStop()
	LockInsideApply()
	LockOutsideApply()
	UnsafeReadWriter
}

type UnsafeReadWriter interface {
	UnsafeReader
	UnsafeWriter
}

type UnsafeWriter interface {
	UnsafeCreateBucket(bucket bucket.Bucket)
	UnsafeDeleteBucket(bucket bucket.Bucket)
	UnsafePut(bucket bucket.Bucket, key []byte, value []byte)
	UnsafeSeqPut(bucket bucket.Bucket, key []byte, value []byte)
	UnsafeDelete(bucket bucket.Bucket, key []byte)
}

type batchTx struct {
	sync.Mutex
	tx      interfaces.Tx
	backend *backend

	pending int
}

// Lock is supposed to be called only by the unit test.
func (t *batchTx) Lock() {
	ValidateCalledInsideUnittest(t.backend.lg)
	t.lock()
}

func (t *batchTx) lock() {
	t.Mutex.Lock()
}

func (t *batchTx) LockInsideApply() {
	t.lock()
	if t.backend.txPostLockInsideApplyHook != nil {
		// The callers of some methods (i.e., (*RaftCluster).AddMember)
		// can be coming from both InsideApply and OutsideApply, but the
		// callers from OutsideApply will have a nil txPostLockInsideApplyHook.
		// So we should check the txPostLockInsideApplyHook before validating
		// the callstack.
		ValidateCalledInsideApply(t.backend.lg)
		t.backend.txPostLockInsideApplyHook()
	}
}

func (t *batchTx) LockOutsideApply() {
	ValidateCalledOutSideApply(t.backend.lg)
	t.lock()
}

func (t *batchTx) Unlock() {
	if t.pending >= t.backend.batchLimit {
		t.commit(false)
	}
	t.Mutex.Unlock()
}

func (t *batchTx) UnsafeCreateBucket(bucket bucket.Bucket) {
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

func (t *batchTx) UnsafeDeleteBucket(bucket bucket.Bucket) {
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
func (t *batchTx) UnsafePut(bucket bucket.Bucket, key []byte, value []byte) {
	t.unsafePut(bucket, key, value, false)
}

// UnsafeSeqPut must be called holding the lock on the tx.
func (t *batchTx) UnsafeSeqPut(bucket bucket.Bucket, key []byte, value []byte) {
	t.unsafePut(bucket, key, value, true)
}

func (t *batchTx) unsafePut(bucketType bucket.Bucket, key []byte, value []byte, seq bool) {
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
		bucket.SetFillPercent(0.9)
	}
	if err := bucket.Put(key, value); err != nil {
		t.backend.lg.Fatal(
			"failed to write to a bucket",
			zap.Stringer("bucket-name", bucketType),
			zap.Error(err),
		)
	}
	t.pending++
}

// UnsafeRange must be called holding the lock on the tx.
func (t *batchTx) UnsafeRange(bucketType bucket.Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	bucket := t.tx.Bucket(bucketType.Name())
	if bucket == nil {
		t.backend.lg.Fatal(
			"failed to find a bucket",
			zap.Stringer("bucket-name", bucketType),
			zap.Stack("stack"),
		)
	}
	return bucket.UnsafeRange(key, endKey, limit)
}

// UnsafeDelete must be called holding the lock on the tx.
func (t *batchTx) UnsafeDelete(bucketType bucket.Bucket, key []byte) {
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
func (t *batchTx) UnsafeForEach(bucket bucket.Bucket, visitor func(k, v []byte) error) error {
	return unsafeForEach(t.tx, bucket, visitor)
}

func unsafeForEach(tx interfaces.Tx, bucket bucket.Bucket, visitor func(k, v []byte) error) error {
	if b := tx.Bucket(bucket.Name()); b != nil {
		return b.ForEach(visitor)
	}
	return nil
}

// Commit commits a previous tx and begins a new writable one.
func (t *batchTx) Commit() {
	t.lock()
	t.commit(false)
	t.Unlock()
}

// CommitAndStop commits the previous tx and does not create a new one.
func (t *batchTx) CommitAndStop() {
	t.lock()
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

		t.tx.Observe(rebalanceSec, spillSec, writeSec)
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
			txBuffer:   txBuffer{make(map[bucket.BucketID]*bucketBuffer)},
			bucket2seq: make(map[bucket.BucketID]bool),
		},
	}
	tx.Commit()
	return tx
}

func (t *batchTxBuffered) Unlock() {
	if t.pending != 0 {
		t.backend.readTx.Lock() // blocks txReadBuffer for writing.
		// gofail: var beforeWritebackBuf struct{}
		t.buf.writeback(&t.backend.readTx.buf)
		// gofail: var afterWritebackBuf struct{}
		t.backend.readTx.Unlock()
		if t.pending >= t.backend.batchLimit {
			t.commit(false)
		}
	}
	t.batchTx.Unlock()
}

func (t *batchTxBuffered) Commit() {
	t.lock()
	t.commit(false)
	t.Unlock()
}

func (t *batchTxBuffered) CommitAndStop() {
	t.lock()
	t.commit(true)
	t.Unlock()
}

func (t *batchTxBuffered) commit(stop bool) {
	// all read txs must be closed to acquire boltdb commit rwlock
	t.backend.readTx.Lock()
	t.unsafeCommit(stop)
	t.backend.readTx.Unlock()
}

func (t *batchTxBuffered) unsafeCommit(stop bool) {
	if t.backend.hooks != nil {
		// gofail: var commitBeforePreCommitHook struct{}
		t.backend.hooks.OnPreCommitUnsafe(t)
		// gofail: var commitAfterPreCommitHook struct{}
	}

	if t.backend.readTx.tx != nil {
		// wait all store read transactions using the current boltdb tx to finish,
		// then close the boltdb tx
		go func(tx interfaces.Tx, wg *sync.WaitGroup) {
			wg.Wait()
			if err := tx.Rollback(); err != nil {
				t.backend.lg.Fatal("failed to rollback tx", zap.Error(err))
			}
		}(t.backend.readTx.tx, t.backend.readTx.txWg)
		t.backend.readTx.reset()
	}

	t.batchTx.commit(stop)

	if !stop {
		t.backend.readTx.tx = t.backend.begin(false)
	}
}

func (t *batchTxBuffered) UnsafePut(bucket bucket.Bucket, key []byte, value []byte) {
	t.batchTx.UnsafePut(bucket, key, value)
	t.buf.put(bucket, key, value)
}

func (t *batchTxBuffered) UnsafeSeqPut(bucket bucket.Bucket, key []byte, value []byte) {
	t.batchTx.UnsafeSeqPut(bucket, key, value)
	t.buf.putSeq(bucket, key, value)
}
