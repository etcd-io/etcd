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
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/lease"
	"go.etcd.io/etcd/v3/mvcc/backend"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"
	"go.etcd.io/etcd/v3/pkg/testutil"
	"go.etcd.io/etcd/v3/pkg/traceutil"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/zap"
)

// Functional tests for features implemented in v3 store. It treats v3 store
// as a black box, and tests it by feeding the input and validating the output.

// TODO: add similar tests on operations in one txn/rev

type (
	rangeFunc       func(kv KV, key, end []byte, ro RangeOptions) (*RangeResult, error)
	putFunc         func(kv KV, key, value []byte, lease lease.LeaseID) int64
	deleteRangeFunc func(kv KV, key, end []byte) (n, rev int64)
)

var (
	normalRangeFunc = func(kv KV, key, end []byte, ro RangeOptions) (*RangeResult, error) {
		return kv.Range(key, end, ro)
	}
	txnRangeFunc = func(kv KV, key, end []byte, ro RangeOptions) (*RangeResult, error) {
		txn := kv.Read(traceutil.TODO())
		defer txn.End()
		return txn.Range(key, end, ro)
	}

	normalPutFunc = func(kv KV, key, value []byte, lease lease.LeaseID) int64 {
		return kv.Put(key, value, lease)
	}
	txnPutFunc = func(kv KV, key, value []byte, lease lease.LeaseID) int64 {
		txn := kv.Write(traceutil.TODO())
		defer txn.End()
		return txn.Put(key, value, lease)
	}

	normalDeleteRangeFunc = func(kv KV, key, end []byte) (n, rev int64) {
		return kv.DeleteRange(key, end)
	}
	txnDeleteRangeFunc = func(kv KV, key, end []byte) (n, rev int64) {
		txn := kv.Write(traceutil.TODO())
		defer txn.End()
		return txn.DeleteRange(key, end)
	}
)

func TestKVRange(t *testing.T)    { testKVRange(t, normalRangeFunc) }
func TestKVTxnRange(t *testing.T) { testKVRange(t, txnRangeFunc) }

func testKVRange(t *testing.T, f rangeFunc) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	kvs := put3TestKVs(s)

	wrev := int64(4)
	tests := []struct {
		key, end []byte
		wkvs     []mvccpb.KeyValue
	}{
		// get no keys
		{
			[]byte("doo"), []byte("foo"),
			nil,
		},
		// get no keys when key == end
		{
			[]byte("foo"), []byte("foo"),
			nil,
		},
		// get no keys when ranging single key
		{
			[]byte("doo"), nil,
			nil,
		},
		// get all keys
		{
			[]byte("foo"), []byte("foo3"),
			kvs,
		},
		// get partial keys
		{
			[]byte("foo"), []byte("foo1"),
			kvs[:1],
		},
		// get single key
		{
			[]byte("foo"), nil,
			kvs[:1],
		},
		// get entire keyspace
		{
			[]byte(""), []byte(""),
			kvs,
		},
	}

	for i, tt := range tests {
		r, err := f(s, tt.key, tt.end, RangeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Rev != wrev {
			t.Errorf("#%d: rev = %d, want %d", i, r.Rev, wrev)
		}
		if !reflect.DeepEqual(r.KVs, tt.wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, tt.wkvs)
		}
	}
}

func TestKVRangeRev(t *testing.T)    { testKVRangeRev(t, normalRangeFunc) }
func TestKVTxnRangeRev(t *testing.T) { testKVRangeRev(t, txnRangeFunc) }

func testKVRangeRev(t *testing.T, f rangeFunc) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	kvs := put3TestKVs(s)

	tests := []struct {
		rev  int64
		wrev int64
		wkvs []mvccpb.KeyValue
	}{
		{-1, 4, kvs},
		{0, 4, kvs},
		{2, 4, kvs[:1]},
		{3, 4, kvs[:2]},
		{4, 4, kvs},
	}

	for i, tt := range tests {
		r, err := f(s, []byte("foo"), []byte("foo3"), RangeOptions{Rev: tt.rev})
		if err != nil {
			t.Fatal(err)
		}
		if r.Rev != tt.wrev {
			t.Errorf("#%d: rev = %d, want %d", i, r.Rev, tt.wrev)
		}
		if !reflect.DeepEqual(r.KVs, tt.wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, tt.wkvs)
		}
	}
}

func TestKVRangeBadRev(t *testing.T)    { testKVRangeBadRev(t, normalRangeFunc) }
func TestKVTxnRangeBadRev(t *testing.T) { testKVRangeBadRev(t, txnRangeFunc) }

func testKVRangeBadRev(t *testing.T, f rangeFunc) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	put3TestKVs(s)
	if _, err := s.Compact(traceutil.TODO(), 4); err != nil {
		t.Fatalf("compact error (%v)", err)
	}

	tests := []struct {
		rev  int64
		werr error
	}{
		{-1, nil}, // <= 0 is most recent store
		{0, nil},
		{1, ErrCompacted},
		{2, ErrCompacted},
		{4, nil},
		{5, ErrFutureRev},
		{100, ErrFutureRev},
	}
	for i, tt := range tests {
		_, err := f(s, []byte("foo"), []byte("foo3"), RangeOptions{Rev: tt.rev})
		if err != tt.werr {
			t.Errorf("#%d: error = %v, want %v", i, err, tt.werr)
		}
	}
}

func TestKVRangeLimit(t *testing.T)    { testKVRangeLimit(t, normalRangeFunc) }
func TestKVTxnRangeLimit(t *testing.T) { testKVRangeLimit(t, txnRangeFunc) }

func testKVRangeLimit(t *testing.T, f rangeFunc) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	kvs := put3TestKVs(s)

	wrev := int64(4)
	tests := []struct {
		limit int64
		wkvs  []mvccpb.KeyValue
	}{
		// no limit
		{-1, kvs},
		// no limit
		{0, kvs},
		{1, kvs[:1]},
		{2, kvs[:2]},
		{3, kvs},
		{100, kvs},
	}
	for i, tt := range tests {
		r, err := f(s, []byte("foo"), []byte("foo3"), RangeOptions{Limit: tt.limit})
		if err != nil {
			t.Fatalf("#%d: range error (%v)", i, err)
		}
		if !reflect.DeepEqual(r.KVs, tt.wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, tt.wkvs)
		}
		if r.Rev != wrev {
			t.Errorf("#%d: rev = %d, want %d", i, r.Rev, wrev)
		}
		if tt.limit <= 0 || int(tt.limit) > len(kvs) {
			if r.Count != len(kvs) {
				t.Errorf("#%d: count = %d, want %d", i, r.Count, len(kvs))
			}
		} else if r.Count != int(tt.limit) {
			t.Errorf("#%d: count = %d, want %d", i, r.Count, tt.limit)
		}
	}
}

func TestKVPutMultipleTimes(t *testing.T)    { testKVPutMultipleTimes(t, normalPutFunc) }
func TestKVTxnPutMultipleTimes(t *testing.T) { testKVPutMultipleTimes(t, txnPutFunc) }

func testKVPutMultipleTimes(t *testing.T, f putFunc) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	for i := 0; i < 10; i++ {
		base := int64(i + 1)

		rev := f(s, []byte("foo"), []byte("bar"), lease.LeaseID(base))
		if rev != base+1 {
			t.Errorf("#%d: rev = %d, want %d", i, rev, base+1)
		}

		r, err := s.Range([]byte("foo"), nil, RangeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		wkvs := []mvccpb.KeyValue{
			{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: base + 1, Version: base, Lease: base},
		}
		if !reflect.DeepEqual(r.KVs, wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, wkvs)
		}
	}
}

func TestKVDeleteRange(t *testing.T)    { testKVDeleteRange(t, normalDeleteRangeFunc) }
func TestKVTxnDeleteRange(t *testing.T) { testKVDeleteRange(t, txnDeleteRangeFunc) }

func testKVDeleteRange(t *testing.T, f deleteRangeFunc) {
	tests := []struct {
		key, end []byte

		wrev int64
		wN   int64
	}{
		{
			[]byte("foo"), nil,
			5, 1,
		},
		{
			[]byte("foo"), []byte("foo1"),
			5, 1,
		},
		{
			[]byte("foo"), []byte("foo2"),
			5, 2,
		},
		{
			[]byte("foo"), []byte("foo3"),
			5, 3,
		},
		{
			[]byte("foo3"), []byte("foo8"),
			4, 0,
		},
		{
			[]byte("foo3"), nil,
			4, 0,
		},
	}

	for i, tt := range tests {
		b, tmpPath := backend.NewDefaultTmpBackend()
		s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})

		s.Put([]byte("foo"), []byte("bar"), lease.NoLease)
		s.Put([]byte("foo1"), []byte("bar1"), lease.NoLease)
		s.Put([]byte("foo2"), []byte("bar2"), lease.NoLease)

		n, rev := f(s, tt.key, tt.end)
		if n != tt.wN || rev != tt.wrev {
			t.Errorf("#%d: n = %d, rev = %d, want (%d, %d)", i, n, rev, tt.wN, tt.wrev)
		}

		cleanup(s, b, tmpPath)
	}
}

func TestKVDeleteMultipleTimes(t *testing.T)    { testKVDeleteMultipleTimes(t, normalDeleteRangeFunc) }
func TestKVTxnDeleteMultipleTimes(t *testing.T) { testKVDeleteMultipleTimes(t, txnDeleteRangeFunc) }

func testKVDeleteMultipleTimes(t *testing.T, f deleteRangeFunc) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	s.Put([]byte("foo"), []byte("bar"), lease.NoLease)

	n, rev := f(s, []byte("foo"), nil)
	if n != 1 || rev != 3 {
		t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 1, 3)
	}

	for i := 0; i < 10; i++ {
		n, rev := f(s, []byte("foo"), nil)
		if n != 0 || rev != 3 {
			t.Fatalf("#%d: n = %d, rev = %d, want (%d, %d)", i, n, rev, 0, 3)
		}
	}
}

// test that range, put, delete on single key in sequence repeatedly works correctly.
func TestKVOperationInSequence(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	for i := 0; i < 10; i++ {
		base := int64(i*2 + 1)

		// put foo
		rev := s.Put([]byte("foo"), []byte("bar"), lease.NoLease)
		if rev != base+1 {
			t.Errorf("#%d: put rev = %d, want %d", i, rev, base+1)
		}

		r, err := s.Range([]byte("foo"), nil, RangeOptions{Rev: base + 1})
		if err != nil {
			t.Fatal(err)
		}
		wkvs := []mvccpb.KeyValue{
			{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: base + 1, ModRevision: base + 1, Version: 1, Lease: int64(lease.NoLease)},
		}
		if !reflect.DeepEqual(r.KVs, wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, wkvs)
		}
		if r.Rev != base+1 {
			t.Errorf("#%d: range rev = %d, want %d", i, rev, base+1)
		}

		// delete foo
		n, rev := s.DeleteRange([]byte("foo"), nil)
		if n != 1 || rev != base+2 {
			t.Errorf("#%d: n = %d, rev = %d, want (%d, %d)", i, n, rev, 1, base+2)
		}

		r, err = s.Range([]byte("foo"), nil, RangeOptions{Rev: base + 2})
		if err != nil {
			t.Fatal(err)
		}
		if r.KVs != nil {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, nil)
		}
		if r.Rev != base+2 {
			t.Errorf("#%d: range rev = %d, want %d", i, r.Rev, base+2)
		}
	}
}

func TestKVTxnBlockWriteOperations(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})

	tests := []func(){
		func() { s.Put([]byte("foo"), nil, lease.NoLease) },
		func() { s.DeleteRange([]byte("foo"), nil) },
	}
	for i, tt := range tests {
		tf := tt
		txn := s.Write(traceutil.TODO())
		done := make(chan struct{}, 1)
		go func() {
			tf()
			done <- struct{}{}
		}()
		select {
		case <-done:
			t.Fatalf("#%d: operation failed to be blocked", i)
		case <-time.After(10 * time.Millisecond):
		}

		txn.End()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			testutil.FatalStack(t, fmt.Sprintf("#%d: operation failed to be unblocked", i))
		}
	}

	// only close backend when we know all the tx are finished
	cleanup(s, b, tmpPath)
}

func TestKVTxnNonBlockRange(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	txn := s.Write(traceutil.TODO())
	defer txn.End()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		s.Range([]byte("foo"), nil, RangeOptions{})
	}()
	select {
	case <-donec:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("range operation blocked on write txn")
	}
}

// test that txn range, put, delete on single key in sequence repeatedly works correctly.
func TestKVTxnOperationInSequence(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	for i := 0; i < 10; i++ {
		txn := s.Write(traceutil.TODO())
		base := int64(i + 1)

		// put foo
		rev := txn.Put([]byte("foo"), []byte("bar"), lease.NoLease)
		if rev != base+1 {
			t.Errorf("#%d: put rev = %d, want %d", i, rev, base+1)
		}

		r, err := txn.Range([]byte("foo"), nil, RangeOptions{Rev: base + 1})
		if err != nil {
			t.Fatal(err)
		}
		wkvs := []mvccpb.KeyValue{
			{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: base + 1, ModRevision: base + 1, Version: 1, Lease: int64(lease.NoLease)},
		}
		if !reflect.DeepEqual(r.KVs, wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, wkvs)
		}
		if r.Rev != base+1 {
			t.Errorf("#%d: range rev = %d, want %d", i, r.Rev, base+1)
		}

		// delete foo
		n, rev := txn.DeleteRange([]byte("foo"), nil)
		if n != 1 || rev != base+1 {
			t.Errorf("#%d: n = %d, rev = %d, want (%d, %d)", i, n, rev, 1, base+1)
		}

		r, err = txn.Range([]byte("foo"), nil, RangeOptions{Rev: base + 1})
		if err != nil {
			t.Errorf("#%d: range error (%v)", i, err)
		}
		if r.KVs != nil {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, nil)
		}
		if r.Rev != base+1 {
			t.Errorf("#%d: range rev = %d, want %d", i, r.Rev, base+1)
		}

		txn.End()
	}
}

func TestKVCompactReserveLastValue(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	s.Put([]byte("foo"), []byte("bar0"), 1)
	s.Put([]byte("foo"), []byte("bar1"), 2)
	s.DeleteRange([]byte("foo"), nil)
	s.Put([]byte("foo"), []byte("bar2"), 3)

	// rev in tests will be called in Compact() one by one on the same store
	tests := []struct {
		rev int64
		// wanted kvs right after the compacted rev
		wkvs []mvccpb.KeyValue
	}{
		{
			1,
			[]mvccpb.KeyValue{
				{Key: []byte("foo"), Value: []byte("bar0"), CreateRevision: 2, ModRevision: 2, Version: 1, Lease: 1},
			},
		},
		{
			2,
			[]mvccpb.KeyValue{
				{Key: []byte("foo"), Value: []byte("bar1"), CreateRevision: 2, ModRevision: 3, Version: 2, Lease: 2},
			},
		},
		{
			3,
			nil,
		},
		{
			4,
			[]mvccpb.KeyValue{
				{Key: []byte("foo"), Value: []byte("bar2"), CreateRevision: 5, ModRevision: 5, Version: 1, Lease: 3},
			},
		},
	}
	for i, tt := range tests {
		_, err := s.Compact(traceutil.TODO(), tt.rev)
		if err != nil {
			t.Errorf("#%d: unexpect compact error %v", i, err)
		}
		r, err := s.Range([]byte("foo"), nil, RangeOptions{Rev: tt.rev + 1})
		if err != nil {
			t.Errorf("#%d: unexpect range error %v", i, err)
		}
		if !reflect.DeepEqual(r.KVs, tt.wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, r.KVs, tt.wkvs)
		}
	}
}

func TestKVCompactBad(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	s.Put([]byte("foo"), []byte("bar0"), lease.NoLease)
	s.Put([]byte("foo"), []byte("bar1"), lease.NoLease)
	s.Put([]byte("foo"), []byte("bar2"), lease.NoLease)

	// rev in tests will be called in Compact() one by one on the same store
	tests := []struct {
		rev  int64
		werr error
	}{
		{0, nil},
		{1, nil},
		{1, ErrCompacted},
		{4, nil},
		{5, ErrFutureRev},
		{100, ErrFutureRev},
	}
	for i, tt := range tests {
		_, err := s.Compact(traceutil.TODO(), tt.rev)
		if err != tt.werr {
			t.Errorf("#%d: compact error = %v, want %v", i, err, tt.werr)
		}
	}
}

func TestKVHash(t *testing.T) {
	hashes := make([]uint32, 3)

	for i := 0; i < len(hashes); i++ {
		var err error
		b, tmpPath := backend.NewDefaultTmpBackend()
		kv := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
		kv.Put([]byte("foo0"), []byte("bar0"), lease.NoLease)
		kv.Put([]byte("foo1"), []byte("bar0"), lease.NoLease)
		hashes[i], _, err = kv.Hash()
		if err != nil {
			t.Fatalf("failed to get hash: %v", err)
		}
		cleanup(kv, b, tmpPath)
	}

	for i := 1; i < len(hashes); i++ {
		if hashes[i-1] != hashes[i] {
			t.Errorf("hash[%d](%d) != hash[%d](%d)", i-1, hashes[i-1], i, hashes[i])
		}
	}
}

func TestKVRestore(t *testing.T) {
	tests := []func(kv KV){
		func(kv KV) {
			kv.Put([]byte("foo"), []byte("bar0"), 1)
			kv.Put([]byte("foo"), []byte("bar1"), 2)
			kv.Put([]byte("foo"), []byte("bar2"), 3)
			kv.Put([]byte("foo2"), []byte("bar0"), 1)
		},
		func(kv KV) {
			kv.Put([]byte("foo"), []byte("bar0"), 1)
			kv.DeleteRange([]byte("foo"), nil)
			kv.Put([]byte("foo"), []byte("bar1"), 2)
		},
		func(kv KV) {
			kv.Put([]byte("foo"), []byte("bar0"), 1)
			kv.Put([]byte("foo"), []byte("bar1"), 2)
			kv.Compact(traceutil.TODO(), 1)
		},
	}
	for i, tt := range tests {
		b, tmpPath := backend.NewDefaultTmpBackend()
		s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
		tt(s)
		var kvss [][]mvccpb.KeyValue
		for k := int64(0); k < 10; k++ {
			r, _ := s.Range([]byte("a"), []byte("z"), RangeOptions{Rev: k})
			kvss = append(kvss, r.KVs)
		}

		keysBefore := readGaugeInt(keysGauge)
		s.Close()

		// ns should recover the the previous state from backend.
		ns := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})

		if keysRestore := readGaugeInt(keysGauge); keysBefore != keysRestore {
			t.Errorf("#%d: got %d key count, expected %d", i, keysRestore, keysBefore)
		}

		// wait for possible compaction to finish
		testutil.WaitSchedule()
		var nkvss [][]mvccpb.KeyValue
		for k := int64(0); k < 10; k++ {
			r, _ := ns.Range([]byte("a"), []byte("z"), RangeOptions{Rev: k})
			nkvss = append(nkvss, r.KVs)
		}
		cleanup(ns, b, tmpPath)

		if !reflect.DeepEqual(nkvss, kvss) {
			t.Errorf("#%d: kvs history = %+v, want %+v", i, nkvss, kvss)
		}
	}
}

func readGaugeInt(g prometheus.Gauge) int {
	ch := make(chan prometheus.Metric, 1)
	g.Collect(ch)
	m := <-ch
	mm := &dto.Metric{}
	m.Write(mm)
	return int(mm.GetGauge().GetValue())
}

func TestKVSnapshot(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer cleanup(s, b, tmpPath)

	wkvs := put3TestKVs(s)

	newPath := "new_test"
	f, err := os.Create(newPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(newPath)

	snap := s.b.Snapshot()
	defer snap.Close()
	_, err = snap.WriteTo(f)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	ns := NewStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{})
	defer ns.Close()
	r, err := ns.Range([]byte("a"), []byte("z"), RangeOptions{})
	if err != nil {
		t.Errorf("unexpect range error (%v)", err)
	}
	if !reflect.DeepEqual(r.KVs, wkvs) {
		t.Errorf("kvs = %+v, want %+v", r.KVs, wkvs)
	}
	if r.Rev != 4 {
		t.Errorf("rev = %d, want %d", r.Rev, 4)
	}
}

func TestWatchableKVWatch(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := WatchableKV(newWatchableStore(zap.NewExample(), b, &lease.FakeLessor{}, nil, StoreConfig{}))
	defer cleanup(s, b, tmpPath)

	w := s.NewWatchStream()
	defer w.Close()

	wid, _ := w.Watch(0, []byte("foo"), []byte("fop"), 0)

	wev := []mvccpb.Event{
		{Type: mvccpb.PUT,
			Kv: &mvccpb.KeyValue{
				Key:            []byte("foo"),
				Value:          []byte("bar"),
				CreateRevision: 2,
				ModRevision:    2,
				Version:        1,
				Lease:          1,
			},
		},
		{
			Type: mvccpb.PUT,
			Kv: &mvccpb.KeyValue{
				Key:            []byte("foo1"),
				Value:          []byte("bar1"),
				CreateRevision: 3,
				ModRevision:    3,
				Version:        1,
				Lease:          2,
			},
		},
		{
			Type: mvccpb.PUT,
			Kv: &mvccpb.KeyValue{
				Key:            []byte("foo1"),
				Value:          []byte("bar11"),
				CreateRevision: 3,
				ModRevision:    4,
				Version:        2,
				Lease:          3,
			},
		},
	}

	s.Put([]byte("foo"), []byte("bar"), 1)
	select {
	case resp := <-w.Chan():
		if resp.WatchID != wid {
			t.Errorf("resp.WatchID got = %d, want = %d", resp.WatchID, wid)
		}
		ev := resp.Events[0]
		if !reflect.DeepEqual(ev, wev[0]) {
			t.Errorf("watched event = %+v, want %+v", ev, wev[0])
		}
	case <-time.After(5 * time.Second):
		// CPU might be too slow, and the routine is not able to switch around
		testutil.FatalStack(t, "failed to watch the event")
	}

	s.Put([]byte("foo1"), []byte("bar1"), 2)
	select {
	case resp := <-w.Chan():
		if resp.WatchID != wid {
			t.Errorf("resp.WatchID got = %d, want = %d", resp.WatchID, wid)
		}
		ev := resp.Events[0]
		if !reflect.DeepEqual(ev, wev[1]) {
			t.Errorf("watched event = %+v, want %+v", ev, wev[1])
		}
	case <-time.After(5 * time.Second):
		testutil.FatalStack(t, "failed to watch the event")
	}

	w = s.NewWatchStream()
	wid, _ = w.Watch(0, []byte("foo1"), []byte("foo2"), 3)

	select {
	case resp := <-w.Chan():
		if resp.WatchID != wid {
			t.Errorf("resp.WatchID got = %d, want = %d", resp.WatchID, wid)
		}
		ev := resp.Events[0]
		if !reflect.DeepEqual(ev, wev[1]) {
			t.Errorf("watched event = %+v, want %+v", ev, wev[1])
		}
	case <-time.After(5 * time.Second):
		testutil.FatalStack(t, "failed to watch the event")
	}

	s.Put([]byte("foo1"), []byte("bar11"), 3)
	select {
	case resp := <-w.Chan():
		if resp.WatchID != wid {
			t.Errorf("resp.WatchID got = %d, want = %d", resp.WatchID, wid)
		}
		ev := resp.Events[0]
		if !reflect.DeepEqual(ev, wev[2]) {
			t.Errorf("watched event = %+v, want %+v", ev, wev[2])
		}
	case <-time.After(5 * time.Second):
		testutil.FatalStack(t, "failed to watch the event")
	}
}

func cleanup(s KV, b backend.Backend, path string) {
	s.Close()
	b.Close()
	os.Remove(path)
}

func put3TestKVs(s KV) []mvccpb.KeyValue {
	s.Put([]byte("foo"), []byte("bar"), 1)
	s.Put([]byte("foo1"), []byte("bar1"), 2)
	s.Put([]byte("foo2"), []byte("bar2"), 3)
	return []mvccpb.KeyValue{
		{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1, Lease: 1},
		{Key: []byte("foo1"), Value: []byte("bar1"), CreateRevision: 3, ModRevision: 3, Version: 1, Lease: 2},
		{Key: []byte("foo2"), Value: []byte("bar2"), CreateRevision: 4, ModRevision: 4, Version: 1, Lease: 3},
	}
}
