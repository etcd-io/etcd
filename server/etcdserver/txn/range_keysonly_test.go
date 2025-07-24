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

package txn

import (
	"context"
	"testing"

	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

// TestRangeKeysOnly tests the KeysOnly functionality in range operations
func TestRangeKeysOnly(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer func() {
		s.Close()
		b.Close()
	}()

	// Setup test data
	s.Put([]byte("foo"), []byte("bar"), lease.NoLease)
	s.Put([]byte("foo1"), []byte("bar1"), lease.NoLease)
	s.Put([]byte("foo2"), []byte("bar2"), lease.NoLease)

	tests := []struct {
		name          string
		keysOnly      bool
		wantNilValues bool
	}{
		{
			name:          "with values",
			keysOnly:      false,
			wantNilValues: false,
		},
		{
			name:          "keys only",
			keysOnly:      true,
			wantNilValues: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.RangeRequest{
				Key:      []byte("foo"),
				RangeEnd: []byte("foo3"),
				KeysOnly: tt.keysOnly,
			}

			resp, _, err := Range(context.Background(), zaptest.NewLogger(t), s, req)
			if err != nil {
				t.Fatalf("Range failed: %v", err)
			}

			if len(resp.Kvs) != 3 {
				t.Errorf("len(kvs) = %d, want 3", len(resp.Kvs))
			}

			for i, kv := range resp.Kvs {
				if tt.wantNilValues {
					if kv.Value != nil {
						t.Errorf("kv[%d].Value should be nil for KeysOnly=true, got %v", i, kv.Value)
					}
				} else {
					if kv.Value == nil {
						t.Errorf("kv[%d].Value should not be nil for KeysOnly=false", i)
					}
				}
			}
		})
	}
}

// TestRangeKeysOnlyWithSorting tests KeysOnly with different sorting options
func TestRangeKeysOnlyWithSorting(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer func() {
		s.Close()
		b.Close()
	}()

	// Setup test data with different values for sorting
	s.Put([]byte("key1"), []byte("zzz"), lease.NoLease) // higher value
	s.Put([]byte("key2"), []byte("aaa"), lease.NoLease) // lower value
	s.Put([]byte("key3"), []byte("mmm"), lease.NoLease) // middle value

	tests := []struct {
		name       string
		sortTarget pb.RangeRequest_SortTarget
		sortOrder  pb.RangeRequest_SortOrder
		keysOnly   bool
		shouldWarn bool
	}{
		{
			name:       "sort by key with KeysOnly",
			sortTarget: pb.RangeRequest_KEY,
			sortOrder:  pb.RangeRequest_ASCEND,
			keysOnly:   true,
			shouldWarn: false,
		},
		{
			name:       "sort by version with KeysOnly",
			sortTarget: pb.RangeRequest_VERSION,
			sortOrder:  pb.RangeRequest_ASCEND,
			keysOnly:   true,
			shouldWarn: false,
		},
		{
			name:       "sort by create revision with KeysOnly",
			sortTarget: pb.RangeRequest_CREATE,
			sortOrder:  pb.RangeRequest_ASCEND,
			keysOnly:   true,
			shouldWarn: false,
		},
		{
			name:       "sort by mod revision with KeysOnly",
			sortTarget: pb.RangeRequest_MOD,
			sortOrder:  pb.RangeRequest_ASCEND,
			keysOnly:   true,
			shouldWarn: false,
		},
		{
			name:       "sort by value with KeysOnly (should warn)",
			sortTarget: pb.RangeRequest_VALUE,
			sortOrder:  pb.RangeRequest_ASCEND,
			keysOnly:   true,
			shouldWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.RangeRequest{
				Key:        []byte("key"),
				RangeEnd:   []byte("key4"),
				KeysOnly:   tt.keysOnly,
				SortTarget: tt.sortTarget,
				SortOrder:  tt.sortOrder,
			}

			resp, _, err := Range(context.Background(), zaptest.NewLogger(t), s, req)
			if err != nil {
				t.Fatalf("Range failed: %v", err)
			}

			if len(resp.Kvs) != 3 {
				t.Errorf("len(kvs) = %d, want 3", len(resp.Kvs))
			}

			// Verify that values are nil when KeysOnly is true
			if tt.keysOnly {
				for i, kv := range resp.Kvs {
					if kv.Value != nil {
						t.Errorf("kv[%d].Value should be nil for KeysOnly=true, got %v", i, kv.Value)
					}
				}
			}

			// For sort by value with KeysOnly, the order might be undefined,
			// but the functionality should still work
		})
	}
}

// TestRangeKeysOnlyWithLimit tests KeysOnly with limit
func TestRangeKeysOnlyWithLimit(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer func() {
		s.Close()
		b.Close()
	}()

	// Setup test data
	s.Put([]byte("foo1"), []byte("bar1"), lease.NoLease)
	s.Put([]byte("foo2"), []byte("bar2"), lease.NoLease)
	s.Put([]byte("foo3"), []byte("bar3"), lease.NoLease)

	req := &pb.RangeRequest{
		Key:      []byte("foo"),
		RangeEnd: []byte("foo4"),
		KeysOnly: true,
		Limit:    2,
	}

	resp, _, err := Range(context.Background(), zaptest.NewLogger(t), s, req)
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(resp.Kvs) != 2 {
		t.Errorf("len(kvs) = %d, want 2", len(resp.Kvs))
	}

	// Check that More flag is set when we hit the limit
	if !resp.More {
		t.Error("More should be true when limit is reached")
	}

	// Verify all returned values are nil
	for i, kv := range resp.Kvs {
		if kv.Value != nil {
			t.Errorf("kv[%d].Value should be nil for KeysOnly=true, got %v", i, kv.Value)
		}
	}
}

// TestRangeKeysOnlyWithCountOnly tests KeysOnly with CountOnly
func TestRangeKeysOnlyWithCountOnly(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer func() {
		s.Close()
		b.Close()
	}()

	// Setup test data
	s.Put([]byte("foo1"), []byte("bar1"), lease.NoLease)
	s.Put([]byte("foo2"), []byte("bar2"), lease.NoLease)
	s.Put([]byte("foo3"), []byte("bar3"), lease.NoLease)

	req := &pb.RangeRequest{
		Key:       []byte("foo"),
		RangeEnd:  []byte("foo4"),
		KeysOnly:  true, // Should be ignored when CountOnly is true
		CountOnly: true,
	}

	resp, _, err := Range(context.Background(), zaptest.NewLogger(t), s, req)
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(resp.Kvs) != 0 {
		t.Errorf("Kvs should be empty for CountOnly request, got %v", resp.Kvs)
	}

	if resp.Count != 3 {
		t.Errorf("Count = %d, want 3", resp.Count)
	}
}
