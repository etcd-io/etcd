// Copyright 2025 The etcd Authors
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

package cache

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestStoreGet(t *testing.T) {
	tests := []struct {
		name       string
		initialKVs []*mvccpb.KeyValue
		initialRev int64

		start []byte
		end   []byte

		expectedKVs []*mvccpb.KeyValue
		expectedRev int64
		expectedErr error
	}{
		{
			name:        "empty_store_returns_ErrNotReady",
			initialKVs:  nil,
			start:       []byte("a"),
			expectedErr: ErrNotReady,
		},
		{
			name:        "Get_single_key_hit",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/b", "2", 5), makeKV("/a", "1", 5), makeKV("/c", "3", 5)},
			initialRev:  5,
			start:       []byte("/b"),
			expectedKVs: []*mvccpb.KeyValue{makeKV("/b", "2", 5)},
			expectedRev: 5,
		},
		{
			name:        "Get_single_key_miss_returns_empty",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/b", "2", 5), makeKV("/a", "1", 5), makeKV("/c", "3", 5)},
			initialRev:  5,
			start:       []byte("/zzz"),
			expectedKVs: nil,
			expectedRev: 5,
		},
		{
			name:        "Get_explicit_range",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/a", "1", 10), makeKV("/b", "2", 10), makeKV("/c", "3", 10), makeKV("/d", "4", 10)},
			initialRev:  10,
			start:       []byte("/b"),
			end:         []byte("/d"),
			expectedKVs: []*mvccpb.KeyValue{makeKV("/b", "2", 10), makeKV("/c", "3", 10)},
			expectedRev: 10,
		},
		{
			name:        "Get_range_includes_prefix_excludes_end",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/a", "1", 4), makeKV("/aa", "2", 4), makeKV("/ab", "3", 4), makeKV("/b", "4", 4)},
			initialRev:  4,
			start:       []byte("/a"),
			end:         []byte("/b"),
			expectedKVs: []*mvccpb.KeyValue{makeKV("/a", "1", 4), makeKV("/aa", "2", 4), makeKV("/ab", "3", 4)},
			expectedRev: 4,
		},
		{
			name:        "Get_empty_range_returns_empty",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/a", "1", 2), makeKV("/b", "2", 2)},
			initialRev:  2,
			start:       []byte("/a"),
			end:         []byte("/a"),
			expectedKVs: nil,
			expectedRev: 2,
		},
		{
			name:        "Get_invalid_range_returns_empty",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/a", "1", 6), makeKV("/z", "9", 6)},
			initialRev:  6,
			start:       []byte("/z"),
			end:         []byte("/a"),
			expectedKVs: nil,
			expectedRev: 6,
		},
		{
			name:        "Get_fromKey_scans_ordered",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/a", "1", 7), makeKV("/b", "2", 7), makeKV("/c", "3", 7), makeKV("/d", "4", 7)},
			initialRev:  7,
			start:       []byte("/b"),
			end:         []byte{0},
			expectedKVs: []*mvccpb.KeyValue{makeKV("/b", "2", 7), makeKV("/c", "3", 7), makeKV("/d", "4", 7)},
			expectedRev: 7,
		},
		{
			name:        "Get_fromKey_with_no_results",
			initialKVs:  []*mvccpb.KeyValue{makeKV("/a", "1", 9), makeKV("/b", "2", 9)},
			initialRev:  9,
			start:       []byte("/zzz"),
			end:         []byte{0},
			expectedKVs: nil,
			expectedRev: 9,
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			s := newStore(8)
			if test.initialKVs != nil {
				s.Restore(test.initialKVs, test.initialRev)
			}

			kvs, rev, err := s.Get(test.start, test.end)

			if test.expectedErr != nil {
				if !errors.Is(err, test.expectedErr) {
					t.Fatalf("Get error = %v; want %v", err, test.expectedErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Get returned unexpected error: %v", err)
			}
			if rev != test.expectedRev {
				t.Fatalf("revision=%d; want %d", rev, test.expectedRev)
			}

			if diff := cmp.Diff(test.expectedKVs, kvs); diff != "" {
				t.Fatalf("Get mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStoreApply(t *testing.T) {
	type testCase struct {
		name         string
		initialKVs   []*mvccpb.KeyValue
		initialRev   int64
		eventBatches [][]*clientv3.Event

		expectedLatestRev int64
		expectedSnapshot  []*mvccpb.KeyValue
		expectErr         bool
	}
	tests := []testCase{
		{
			name:              "put_overwrites_key",
			initialKVs:        []*mvccpb.KeyValue{makeKV("/k", "v1", 10)},
			initialRev:        10,
			eventBatches:      [][]*clientv3.Event{{makePutEvent("/k", "v2", 11)}},
			expectedLatestRev: 11,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/k", "v2", 11)},
		},
		{
			name:       "put_contiguous_revision",
			initialKVs: []*mvccpb.KeyValue{makeKV("/a", "A1", 20)},
			initialRev: 20,
			eventBatches: [][]*clientv3.Event{
				{makePutEvent("/a", "A2", 21)},
				{makePutEvent("/b", "B1", 22)},
				{makePutEvent("/c", "C1", 23)},
			},
			expectedLatestRev: 23,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/a", "A2", 21), makeKV("/b", "B1", 22), makeKV("/c", "C1", 23)},
		},
		{
			name:              "put_single_non_contiguous_batch",
			initialKVs:        []*mvccpb.KeyValue{makeKV("/a", "A1", 20)},
			initialRev:        20,
			eventBatches:      [][]*clientv3.Event{{makePutEvent("/a", "A2", 25)}},
			expectedLatestRev: 25,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/a", "A2", 25)},
		},
		{
			name:       "put_multiple_non_contiguous_batches",
			initialKVs: []*mvccpb.KeyValue{makeKV("/a", "A1", 21), makeKV("/b", "B1", 22)},
			initialRev: 22,
			eventBatches: [][]*clientv3.Event{
				{makePutEvent("/a", "A2", 25)},
				{makePutEvent("/b", "B2", 27)},
			},
			expectedLatestRev: 27,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/a", "A2", 25), makeKV("/b", "B2", 27)},
		},
		{
			name:       "apply_mixed_operations",
			initialKVs: []*mvccpb.KeyValue{makeKV("/a", "A1", 20)},
			initialRev: 20,
			eventBatches: [][]*clientv3.Event{
				{makePutEvent("/a", "A2", 21), makePutEvent("/b", "B1", 21), makePutEvent("/c", "C1", 21)},
				{makePutEvent("/b", "B2", 22)},
				{makeDelEvent("/c", 23), makePutEvent("/a", "A3", 23)},
				{makePutEvent("/b", "B3", 24)},
			},
			expectedLatestRev: 24,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/a", "A3", 23), makeKV("/b", "B3", 24)},
		},
		{
			name:       "delete_same_key",
			initialKVs: []*mvccpb.KeyValue{makeKV("/a", "X", 10)},
			initialRev: 10,
			eventBatches: [][]*clientv3.Event{
				{makeDelEvent("/a", 11)},
			},
			expectedLatestRev: 11,
			expectedSnapshot:  nil,
		},
		{
			name:              "delete_nonexistent_returns_error",
			initialKVs:        []*mvccpb.KeyValue{makeKV("/p", "X", 5)},
			initialRev:        5,
			eventBatches:      [][]*clientv3.Event{{makeDelEvent("/zzz", 6)}},
			expectedLatestRev: 5,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/p", "X", 5)},
			expectErr:         true,
		},
		{
			name:              "mixed_delete_nonexistent_returns_error",
			initialKVs:        []*mvccpb.KeyValue{makeKV("/p", "X", 5)},
			initialRev:        5,
			eventBatches:      [][]*clientv3.Event{{makeDelEvent("/zzz", 6), makePutEvent("/r", "Y", 6)}},
			expectedLatestRev: 5,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/p", "X", 5)},
			expectErr:         true,
		},
		{
			name:       "delete_then_delete_again_returns_error",
			initialKVs: []*mvccpb.KeyValue{makeKV("/p", "X", 5)},
			initialRev: 5,
			eventBatches: [][]*clientv3.Event{
				{makeDelEvent("/p", 6)},
				{makeDelEvent("/p", 7)},
			},
			expectedLatestRev: 6,
			expectedSnapshot:  nil,
			expectErr:         true,
		},
		{
			name:              "stale_batch_rejected",
			initialKVs:        []*mvccpb.KeyValue{makeKV("/x", "1", 20)},
			initialRev:        20,
			eventBatches:      [][]*clientv3.Event{{makePutEvent("/x", "2", 19)}},
			expectedLatestRev: 20,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/x", "1", 20)},
			expectErr:         true,
		},
		{
			name:       "mixed_stale_batch_returns_error",
			initialKVs: []*mvccpb.KeyValue{makeKV("/x", "1", 20)},
			initialRev: 20,
			eventBatches: [][]*clientv3.Event{
				{makePutEvent("/x", "should-not-apply", 19)},
				{makeDelEvent("/x", 21), makePutEvent("/y", "new", 21)},
				{makeDelEvent("/y", 22)},
			},
			expectedLatestRev: 20,
			expectedSnapshot:  []*mvccpb.KeyValue{makeKV("/x", "1", 20)},
			expectErr:         true,
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			s := newStore(4)
			s.Restore(test.initialKVs, test.initialRev)

			var gotErr error
			for batchIndex, batch := range test.eventBatches {
				if err := s.Apply(batch); err != nil {
					gotErr = err
					if !test.expectErr {
						t.Fatalf("Apply(batch %d) unexpected error: %v", batchIndex, err)
					}
					break
				}
			}
			if test.expectErr && gotErr == nil {
				t.Fatalf("expected Apply() to error, but got nil")
			}
			if latest := s.LatestRev(); latest != test.expectedLatestRev {
				t.Fatalf("LatestRev=%d; want %d", latest, test.expectedLatestRev)
			}
			verifyStoreSnapshot(t, s, test.expectedSnapshot, test.expectedLatestRev)
		})
	}
}

func TestStoreRestore(t *testing.T) {
	type restoreSeq struct {
		kvs []*mvccpb.KeyValue
		rev int64
	}
	tests := []struct {
		name         string
		seq          []restoreSeq
		expectedSnap []*mvccpb.KeyValue
		expectedRev  int64
	}{
		{
			name: "rebuilds_tree_and_resets_rev",
			seq: []restoreSeq{
				{[]*mvccpb.KeyValue{makeKV("/a", "1", 3), makeKV("/b", "2", 3)}, 3},
				{[]*mvccpb.KeyValue{makeKV("/c", "3", 15)}, 15},
			},
			expectedSnap: []*mvccpb.KeyValue{makeKV("/c", "3", 15)},
			expectedRev:  15,
		},
		{
			name: "restore_to_revision_zero_returns_ErrNotReady",
			seq: []restoreSeq{
				{[]*mvccpb.KeyValue{makeKV("/a", "1", 5)}, 5},
				{nil, 0},
			},
			expectedSnap: nil,
			expectedRev:  0,
		},
		{
			name:         "restore_empty_ready",
			seq:          []restoreSeq{{nil, 5}},
			expectedSnap: nil,
			expectedRev:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStore(8)
			for _, step := range tt.seq {
				s.Restore(step.kvs, step.rev)
			}
			if tt.expectedRev == 0 {
				if _, _, err := s.Get([]byte("/"), []byte{0}); !errors.Is(err, ErrNotReady) {
					t.Fatalf("Get after restore to rev=0 err=%v; want %v", err, ErrNotReady)
				}
				return
			}
			verifyStoreSnapshot(t, s, tt.expectedSnap, tt.expectedRev)
		})
	}
}

func makeKV(key, val string, rev int64) *mvccpb.KeyValue {
	return &mvccpb.KeyValue{Key: []byte(key), Value: []byte(val), ModRevision: rev, CreateRevision: rev, Version: 1}
}

func makePutEvent(key, val string, rev int64) *clientv3.Event {
	return &clientv3.Event{Type: clientv3.EventTypePut, Kv: &mvccpb.KeyValue{Key: []byte(key), Value: []byte(val), ModRevision: rev, CreateRevision: rev, Version: 1}}
}

func makeDelEvent(key string, rev int64) *clientv3.Event {
	return &clientv3.Event{Type: clientv3.EventTypeDelete, Kv: &mvccpb.KeyValue{Key: []byte(key), ModRevision: rev}}
}

func verifyStoreSnapshot(t *testing.T, s *store, want []*mvccpb.KeyValue, wantRev int64) {
	kvs, rev, err := s.Get([]byte("/"), []byte{0})
	if err != nil {
		t.Fatalf("Get all keys: %v", err)
	}
	if rev != wantRev {
		t.Fatalf("snapshot revision=%d; want %d", rev, wantRev)
	}
	if diff := cmp.Diff(want, kvs); diff != "" {
		t.Fatalf("snapshot mismatch (-want +got):\n%s", diff)
	}
}
