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
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
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
		require.NoError(t, err)

		tx.Lock()
		for _, rev := range tt.wrevs {
			ibytes := NewRevBytes()
			ibytes = RevToBytes(rev, ibytes)
			keys, _ := tx.UnsafeRange(schema.Key, ibytes, nil, 0)
			assert.Lenf(t, keys, 1, "#%d: range on %v = %d, want 1", i, rev, len(keys))
		}
		vals, _ := UnsafeReadFinishedCompact(tx)
		assert.Truef(t, reflect.DeepEqual(vals, tt.rev), "#%d: finished compact equal %+v, want %+v", i, vals, tt.rev)
		tx.Unlock()

		cleanup(s, b)
	}
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
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for compaction to finish")
	}

	require.NoError(t, s0.Close())

	s1 := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	assert.Equalf(t, s1.Rev(), rev, "rev = %v, want %v", s1.Rev(), rev)
	_, err = s1.Range(t.Context(), []byte("foo"), nil, RangeOptions{})
	require.NoErrorf(t, err, "unexpect range error %v", err)
	require.NoError(t, s1.Close())
}
