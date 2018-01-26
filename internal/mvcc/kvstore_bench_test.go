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

package mvcc

import (
	"sync/atomic"
	"testing"

	"github.com/coreos/etcd/internal/lease"
	"github.com/coreos/etcd/internal/mvcc/backend"
)

type fakeConsistentIndex uint64

func (i *fakeConsistentIndex) ConsistentIndex() uint64 {
	return atomic.LoadUint64((*uint64)(i))
}

func BenchmarkStorePut(b *testing.B) {
	var i fakeConsistentIndex
	be, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(be, &lease.FakeLessor{}, &i)
	defer cleanup(s, be, tmpPath)

	// arbitrary number of bytes
	bytesN := 64
	keys := createBytesSlice(bytesN, b.N)
	vals := createBytesSlice(bytesN, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[i], vals[i], lease.NoLease)
	}
}

func BenchmarkStoreRangeKey1(b *testing.B)   { benchmarkStoreRange(b, 1) }
func BenchmarkStoreRangeKey100(b *testing.B) { benchmarkStoreRange(b, 100) }

func benchmarkStoreRange(b *testing.B, n int) {
	var i fakeConsistentIndex
	be, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(be, &lease.FakeLessor{}, &i)
	defer cleanup(s, be, tmpPath)

	// 64 byte key/val
	keys, val := createBytesSlice(64, n), createBytesSlice(64, 1)
	for i := range keys {
		s.Put(keys[i], val[0], lease.NoLease)
	}
	// Force into boltdb tx instead of backend read tx.
	s.Commit()

	var begin, end []byte
	if n == 1 {
		begin, end = keys[0], nil
	} else {
		begin, end = []byte{}, []byte{}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Range(begin, end, RangeOptions{})
	}
}

func BenchmarkConsistentIndex(b *testing.B) {
	fci := fakeConsistentIndex(10)
	be, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(be, &lease.FakeLessor{}, &fci)
	defer cleanup(s, be, tmpPath)

	tx := s.b.BatchTx()
	tx.Lock()
	s.saveIndex(tx)
	tx.Unlock()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ConsistentIndex()
	}
}

// BenchmarkStoreTxnPutUpdate is same as above, but instead updates single key
func BenchmarkStorePutUpdate(b *testing.B) {
	var i fakeConsistentIndex
	be, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(be, &lease.FakeLessor{}, &i)
	defer cleanup(s, be, tmpPath)

	// arbitrary number of bytes
	keys := createBytesSlice(64, 1)
	vals := createBytesSlice(1024, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[0], vals[0], lease.NoLease)
	}
}

// BenchmarkStoreTxnPut benchmarks the Put operation
// with transaction begin and end, where transaction involves
// some synchronization operations, such as mutex locking.
func BenchmarkStoreTxnPut(b *testing.B) {
	var i fakeConsistentIndex
	be, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(be, &lease.FakeLessor{}, &i)
	defer cleanup(s, be, tmpPath)

	// arbitrary number of bytes
	bytesN := 64
	keys := createBytesSlice(bytesN, b.N)
	vals := createBytesSlice(bytesN, b.N)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		txn := s.Write()
		txn.Put(keys[i], vals[i], lease.NoLease)
		txn.End()
	}
}

// benchmarkStoreRestore benchmarks the restore operation
func benchmarkStoreRestore(revsPerKey int, b *testing.B) {
	var i fakeConsistentIndex
	be, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(be, &lease.FakeLessor{}, &i)
	// use closure to capture 's' to pick up the reassignment
	defer func() { cleanup(s, be, tmpPath) }()

	// arbitrary number of bytes
	bytesN := 64
	keys := createBytesSlice(bytesN, b.N)
	vals := createBytesSlice(bytesN, b.N)

	for i := 0; i < b.N; i++ {
		for j := 0; j < revsPerKey; j++ {
			txn := s.Write()
			txn.Put(keys[i], vals[i], lease.NoLease)
			txn.End()
		}
	}
	s.Close()

	b.ReportAllocs()
	b.ResetTimer()
	s = NewStore(be, &lease.FakeLessor{}, &i)
}

func BenchmarkStoreRestoreRevs1(b *testing.B) {
	benchmarkStoreRestore(1, b)
}

func BenchmarkStoreRestoreRevs10(b *testing.B) {
	benchmarkStoreRestore(10, b)
}

func BenchmarkStoreRestoreRevs20(b *testing.B) {
	benchmarkStoreRestore(20, b)
}
