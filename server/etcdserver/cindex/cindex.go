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

package cindex

import (
	"encoding/binary"
	"sync"
	"sync/atomic"

	"go.etcd.io/etcd/server/v3/mvcc/backend"
)

var (
	MetaBucketName = []byte("meta")

	ConsistentIndexKeyName = []byte("consistent_index")
)

// ConsistentIndexer is an interface that wraps the Get/Set/Save method for consistentIndex.
type ConsistentIndexer interface {

	// ConsistentIndex returns the consistent index of current executing entry.
	ConsistentIndex() uint64

	// SetConsistentIndex set the consistent index of current executing entry.
	SetConsistentIndex(v uint64)

	// UnsafeSave must be called holding the lock on the tx.
	// It saves consistentIndex to the underlying stable storage.
	UnsafeSave(tx backend.BatchTx)

	// SetBatchTx set the available backend.BatchTx for ConsistentIndexer.
	SetBatchTx(tx backend.BatchTx)
}

// consistentIndex implements the ConsistentIndexer interface.
type consistentIndex struct {
	tx backend.BatchTx
	// consistentIndex represents the offset of an entry in a consistent replica log.
	// it caches the "consistent_index" key's value. Accessed
	// through atomics so must be 64-bit aligned.
	consistentIndex uint64
	mutex           sync.Mutex
}

func NewConsistentIndex(tx backend.BatchTx) ConsistentIndexer {
	return &consistentIndex{tx: tx}
}

func (ci *consistentIndex) ConsistentIndex() uint64 {

	if index := atomic.LoadUint64(&ci.consistentIndex); index > 0 {
		return index
	}
	ci.mutex.Lock()
	defer ci.mutex.Unlock()
	v := ReadConsistentIndex(ci.tx)
	return v
}

func (ci *consistentIndex) SetConsistentIndex(v uint64) {
	atomic.StoreUint64(&ci.consistentIndex, v)
}

func (ci *consistentIndex) UnsafeSave(tx backend.BatchTx) {
	index := atomic.LoadUint64(&ci.consistentIndex)
	if index == 0 {
		// Never save 0 as it means that we didn't loaded the real index yet.
		return
	}
	unsafeUpdateConsistentIndex(tx, index)
}

func (ci *consistentIndex) SetBatchTx(tx backend.BatchTx) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()
	ci.tx = tx
}

func NewFakeConsistentIndex(index uint64) ConsistentIndexer {
	return &fakeConsistentIndex{index: index}
}

type fakeConsistentIndex struct{ index uint64 }

func (f *fakeConsistentIndex) ConsistentIndex() uint64 { return f.index }

func (f *fakeConsistentIndex) SetConsistentIndex(index uint64) {
	atomic.StoreUint64(&f.index, index)
}

func (f *fakeConsistentIndex) UnsafeSave(tx backend.BatchTx) {}
func (f *fakeConsistentIndex) SetBatchTx(tx backend.BatchTx) {}

func UnsafeCreateMetaBucket(tx backend.BatchTx) {
	tx.UnsafeCreateBucket(MetaBucketName)
}

// unsafeGetConsistentIndex loads consistent index from given transaction.
// returns 0 if the data are not found.
func unsafeReadConsistentIndex(tx backend.ReadTx) uint64 {
	_, vs := tx.UnsafeRange(MetaBucketName, ConsistentIndexKeyName, nil, 0)
	if len(vs) == 0 {
		return 0
	}
	v := binary.BigEndian.Uint64(vs[0])
	return v
}

// ReadConsistentIndex loads consistent index from given transaction.
// returns 0 if the data are not found.
func ReadConsistentIndex(tx backend.ReadTx) uint64 {
	tx.Lock()
	defer tx.Unlock()
	return unsafeReadConsistentIndex(tx)
}

func unsafeUpdateConsistentIndex(tx backend.BatchTx, index uint64) {
	bs := make([]byte, 8) // this is kept on stack (not heap) so its quick.
	binary.BigEndian.PutUint64(bs, index)
	// put the index into the underlying backend
	// tx has been locked in TxnBegin, so there is no need to lock it again
	tx.UnsafePut(MetaBucketName, ConsistentIndexKeyName, bs)
}

func UpdateConsistentIndex(tx backend.BatchTx, index uint64) {
	tx.Lock()
	defer tx.Unlock()

	oldi := unsafeReadConsistentIndex(tx)
	if index <= oldi {
		return
	}
	unsafeUpdateConsistentIndex(tx, index)
}
