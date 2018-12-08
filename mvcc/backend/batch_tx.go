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

	"github.com/dgraph-io/badger"
	"go.uber.org/zap"
)

type BatchTx interface {
	ReadTx
	UnsafeCreateBucket(name []byte)
	UnsafePut(bucketName []byte, key []byte, value []byte)
	UnsafeSeqPut(bucketName []byte, key []byte, value []byte)
	UnsafeDelete(bucketName []byte, key []byte)
	// Commit commits a previous tx and begins a new writable one.
	Commit()
	// CommitAndStop commits the previous tx and does not create a new one.
	CommitAndStop()
}

type batchTx struct {
	sync.Mutex
	tx      *badger.Txn
	backend *backend

	pending int
}

func (t *batchTx) UnsafeCreateBucket(name []byte) {}

// UnsafePut must be called holding the lock on the tx.
func (t *batchTx) UnsafePut(bucketName []byte, key []byte, value []byte) {
	t.unsafePut(bucketName, key, value, false)
}

// UnsafeSeqPut must be called holding the lock on the tx.
func (t *batchTx) UnsafeSeqPut(bucketName []byte, key []byte, value []byte) {
	t.unsafePut(bucketName, key, value, true)
}

func (t *batchTx) unsafePut(bucketName []byte, key []byte, value []byte, seq bool) {
	err := t.tx.Set(append(bucketName, key...), value)
	if err != nil {
		panic(err)
	}
	t.pending++
}

// UnsafeRange must be called holding the lock on the tx.
func (t *batchTx) UnsafeRange(bucketName, key, endKey []byte, limit int64) (keys [][]byte, vs [][]byte) {
	return unsafeRange(t.tx, bucketName, key, endKey, limit)
}

func unsafeRange(tx *badger.Txn, bucketName, key, endKey []byte, limit int64) (keys [][]byte, vs [][]byte) {
	if limit <= 0 {
		limit = math.MaxInt64
	}
	key = append(bucketName, key...)
	if len(endKey) != 0 {
		endKey = append(bucketName, endKey...)
	}

	var isMatch func(b []byte) bool
	if len(endKey) > 0 {
		isMatch = func(b []byte) bool { return bytes.Compare(b, endKey) < 0 }
	} else {
		isMatch = func(b []byte) bool { return bytes.Equal(b, key) }
		limit = 1
	}

	opt := badger.DefaultIteratorOptions

	it := tx.NewIterator(opt)
	defer it.Close()
	for it.Seek(key); it.Valid(); it.Next() {
		if !isMatch(it.Item().Key()) {
			break
		}
		keys = append(keys, it.Item().KeyCopy(nil)[len(bucketName):])
		v, err := it.Item().ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		vs = append(vs, v)

		if limit == int64(len(keys)) {
			break
		}
	}

	return keys, vs
}

// UnsafeDelete must be called holding the lock on the tx.
func (t *batchTx) UnsafeDelete(bucketName []byte, key []byte) {
	err := t.tx.Delete(append(bucketName, key...))
	if err != nil {
		panic(err)
	}
	t.pending++
}

// UnsafeForEach must be called holding the lock on the tx.
func (t *batchTx) UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error {
	return unsafeForEach(t.tx, bucketName, visitor)
}

func unsafeForEach(tx *badger.Txn, bucketName []byte, visitor func(k, v []byte) error) error {
	it := tx.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	for it.Seek(bucketName); it.ValidForPrefix(bucketName); it.Next() {
		item := it.Item()
		k := item.Key()
		v, err := item.Value()
		if err != nil {
			panic(err)
		}
		visitor(k[len(bucketName):], v)
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

func (t *batchTx) Unlock() {
	if t.pending >= t.backend.batchLimit {
		t.commit(false)
	}
	t.Mutex.Unlock()
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
		err := t.tx.Commit(nil)
		// gofail: var afterCommit struct{}

		commitSec.Observe(time.Since(start).Seconds())
		atomic.AddInt64(&t.backend.commits, 1)

		t.pending = 0
		if err != nil {
			if t.backend.lg != nil {
				t.backend.lg.Fatal("failed to commit tx", zap.Error(err))
			} else {
				plog.Fatalf("cannot commit tx (%s)", err)
			}
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
			txBuffer: txBuffer{make(map[string]*bucketBuffer)},
			seq:      true,
		},
	}
	tx.Commit()
	return tx
}

func (t *batchTxBuffered) Unlock() {
	if t.pending != 0 {
		t.backend.readTx.mu.Lock()
		t.buf.writeback(&t.backend.readTx.buf)
		t.backend.readTx.mu.Unlock()
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
	// all read txs must be closed to acquire boltdb commit rwlock
	t.backend.readTx.mu.Lock()
	t.unsafeCommit(stop)
	t.backend.readTx.mu.Unlock()
}

func (t *batchTxBuffered) unsafeCommit(stop bool) {
	if t.backend.readTx.tx != nil {
		t.backend.readTx.tx.Discard()
		t.backend.readTx.reset()
	}

	t.batchTx.commit(stop)

	if !stop {
		t.backend.readTx.tx = t.backend.begin(false)
	}
}

func (t *batchTxBuffered) UnsafePut(bucketName []byte, key []byte, value []byte) {
	t.batchTx.UnsafePut(bucketName, key, value)
	t.buf.put(bucketName, key, value)
}

func (t *batchTxBuffered) UnsafeSeqPut(bucketName []byte, key []byte, value []byte) {
	t.batchTx.UnsafeSeqPut(bucketName, key, value)
	t.buf.putSeq(bucketName, key, value)
}
