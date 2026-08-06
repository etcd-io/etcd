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
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

func TestScheduleCompaction(t *testing.T) {
	revs := []Revision{{Main: 1}, {Main: 2}, {Main: 3}}

	tests := []struct {
		rev   int64
		keep  map[Revision]struct{}
		wrevs []Revision
	}{
		// compact at 1 and discard all history
		{
			1,
			nil,
			revs[1:],
		},
		// compact at 3 and discard all history
		{
			3,
			nil,
			nil,
		},
		// compact at 1 and keeps history one step earlier
		{
			1,
			map[Revision]struct{}{
				{Main: 1}: {},
			},
			revs,
		},
		// compact at 1 and keeps history two steps earlier
		{
			3,
			map[Revision]struct{}{
				{Main: 2}: {},
				{Main: 3}: {},
			},
			revs[1:],
		},
	}
	for i, tt := range tests {
		b, _ := betesting.NewDefaultTmpBackend(t)
		s := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
		fi := newFakeIndex()
		fi.indexCompactRespc <- tt.keep
		s.kvindex = fi

		tx := s.b.BatchTx()

		tx.Lock()
		for _, rev := range revs {
			ibytes := NewRevBytes()
			ibytes = RevToBytes(rev, ibytes)
			tx.UnsafePut(schema.Key, ibytes, []byte("bar"))
		}
		tx.Unlock()

		_, err := s.scheduleCompaction(tt.rev, 0)
		if err != nil {
			t.Error(err)
		}

		tx.Lock()
		for _, rev := range tt.wrevs {
			ibytes := NewRevBytes()
			ibytes = RevToBytes(rev, ibytes)
			keys, _ := tx.UnsafeRange(schema.Key, ibytes, nil, 0)
			if len(keys) != 1 {
				t.Errorf("#%d: range on %v = %d, want 1", i, rev, len(keys))
			}
		}
		vals, _ := UnsafeReadFinishedCompact(tx)
		if !reflect.DeepEqual(vals, tt.rev) {
			t.Errorf("#%d: finished compact equal %+v, want %+v", i, vals, tt.rev)
		}
		tx.Unlock()

		cleanup(s, b)
	}
}

// TestScheduleCompactionIndexDriven verifies that index-driven compaction leaves
// exactly the same key bucket as the standard compaction for the same data and
// compaction revision. This is the data-consistency check between the two
// implementations, and in particular exercises deletion of tombstone-marked
// revisions, whose backend keys differ in length from normal revisions.
func TestScheduleCompactionIndexDriven(t *testing.T) {
	tcs := []struct {
		name string
		ops  func(s *store)
		rev  int64
	}{
		{
			name: "overwrites only",
			ops: func(s *store) {
				s.Put([]byte("foo"), []byte("bar0"), lease.NoLease) // rev 2
				s.Put([]byte("foo"), []byte("bar1"), lease.NoLease) // rev 3
				s.Put([]byte("foo"), []byte("bar2"), lease.NoLease) // rev 4
			},
			rev: 3,
		},
		{
			name: "keep tombstone",
			ops: func(s *store) {
				s.Put([]byte("foo"), []byte("bar0"), lease.NoLease) // rev 2
				s.Put([]byte("foo"), []byte("bar1"), lease.NoLease) // rev 3
				s.DeleteRange([]byte("foo"), nil)                   // rev 4 (tombstone)
			},
			rev: 4,
		},
		{
			name: "drop key including tombstone",
			ops: func(s *store) {
				s.Put([]byte("foo"), []byte("bar0"), lease.NoLease) // rev 2
				s.DeleteRange([]byte("foo"), nil)                   // rev 3 (tombstone)
				s.Put([]byte("other"), []byte("v"), lease.NoLease)  // rev 4
			},
			rev: 4,
		},
		{
			name: "delete tombstone across generations",
			ops: func(s *store) {
				s.Put([]byte("k"), []byte("1"), lease.NoLease) // rev 2
				s.DeleteRange([]byte("k"), nil)                // rev 3 (tombstone)
				s.Put([]byte("k"), []byte("2"), lease.NoLease) // rev 4
				s.DeleteRange([]byte("k"), nil)                // rev 5 (tombstone)
				s.Put([]byte("k"), []byte("3"), lease.NoLease) // rev 6
			},
			rev: 5,
		},
		{
			name: "multiple keys mixed",
			ops: func(s *store) {
				s.Put([]byte("a"), []byte("1"), lease.NoLease) // rev 2
				s.Put([]byte("b"), []byte("1"), lease.NoLease) // rev 3
				s.Put([]byte("a"), []byte("2"), lease.NoLease) // rev 4
				s.DeleteRange([]byte("b"), nil)                // rev 5 (tombstone)
				s.Put([]byte("c"), []byte("1"), lease.NoLease) // rev 6
				s.Put([]byte("a"), []byte("3"), lease.NoLease) // rev 7
			},
			rev: 6,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			standard := compactKeyBucket(t, tc.ops, tc.rev, false)
			indexDriven := compactKeyBucket(t, tc.ops, tc.rev, true)
			require.Equal(t, standard, indexDriven, "index-driven compaction diverged from standard")
		})
	}
}

func BenchmarkScheduleCompaction(b *testing.B) {
	for _, tc := range []struct {
		name string
		opts compactBenchmarkOptions
	}{
		{
			name: "single-key-overwrites",
			opts: compactBenchmarkOptions{keys: 1, revisions: 10000, compactRev: 10000},
		},
		{
			name: "many-key-overwrites",
			opts: compactBenchmarkOptions{keys: 1000, revisions: 10000, compactRev: 10000},
		},
		{
			name: "mostly-live",
			opts: compactBenchmarkOptions{keys: 10000, revisions: 10000, compactRev: 10000},
		},
	} {
		b.Run(tc.name+"/standard", func(b *testing.B) {
			benchmarkScheduleCompaction(b, tc.opts, false)
		})
		b.Run(tc.name+"/index-driven", func(b *testing.B) {
			benchmarkScheduleCompaction(b, tc.opts, true)
		})
	}
}

type compactBenchmarkOptions struct {
	keys       int
	revisions  int
	compactRev int64
}

func benchmarkScheduleCompaction(b *testing.B, opts compactBenchmarkOptions, indexDriven bool) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		be := newBenchmarkBackend(b)
		s := NewStore(zap.NewNop(), be, &lease.FakeLessor{}, StoreConfig{
			CompactionBatchLimit: opts.revisions + 1,
		})
		for rev := 0; rev < opts.revisions; rev++ {
			key := []byte(fmt.Sprintf("key-%08d", rev%opts.keys))
			value := []byte(fmt.Sprintf("value-%08d", rev))
			s.Put(key, value, lease.NoLease)
		}
		b.StartTimer()

		var err error
		if indexDriven {
			_, err = s.scheduleCompactionIndexDriven(opts.compactRev, 0)
		} else {
			_, err = s.scheduleCompaction(opts.compactRev, 0)
		}

		b.StopTimer()
		require.NoError(b, err)
		cleanup(s, be)
	}
}

func newBenchmarkBackend(b *testing.B) backend.Backend {
	b.Helper()
	dir, err := os.MkdirTemp(b.TempDir(), "etcd_backend_bench")
	require.NoError(b, err)
	cfg := backend.DefaultBackendConfig(zap.NewNop())
	cfg.Path = filepath.Join(dir, "database")
	return backend.New(cfg)
}

// compactKeyBucket builds a store, applies ops, compacts at rev using either
// the standard or the index-driven path, and returns the remaining key bucket.
func compactKeyBucket(t *testing.T, ops func(s *store), rev int64, indexDriven bool) map[string]string {
	t.Helper()
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	ops(s)

	var err error
	if indexDriven {
		_, err = s.scheduleCompactionIndexDriven(rev, 0)
	} else {
		_, err = s.scheduleCompaction(rev, 0)
	}
	require.NoError(t, err)

	return readKeyBucket(t, b)
}

// readKeyBucket returns all committed key/value pairs in the key bucket.
func readKeyBucket(t *testing.T, b backend.Backend) map[string]string {
	t.Helper()
	b.ForceCommit()
	got := map[string]string{}
	tx := b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	err := tx.UnsafeForEach(schema.Key, func(k, v []byte) error {
		got[string(k)] = string(v)
		return nil
	})
	require.NoError(t, err)
	return got
}

func TestCompactAllAndRestore(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s0 := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer b.Close()

	s0.Put([]byte("foo"), []byte("bar"), lease.NoLease)
	s0.Put([]byte("foo"), []byte("bar1"), lease.NoLease)
	s0.Put([]byte("foo"), []byte("bar2"), lease.NoLease)
	s0.DeleteRange([]byte("foo"), nil)

	rev := s0.Rev()
	// compact all keys
	done, err := s0.Compact(traceutil.TODO(), rev)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for compaction to finish")
	}

	err = s0.Close()
	if err != nil {
		t.Fatal(err)
	}

	s1 := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	if s1.Rev() != rev {
		t.Errorf("rev = %v, want %v", s1.Rev(), rev)
	}
	_, err = s1.Range(t.Context(), []byte("foo"), nil, RangeOptions{})
	if err != nil {
		t.Errorf("unexpected range error %v", err)
	}
	err = s1.Close()
	if err != nil {
		t.Fatal(err)
	}
}
