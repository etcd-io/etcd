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

package mvcc

import (
	"reflect"
	"testing"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

// TestKVRangeKeysOnly tests the KeysOnly functionality for range operations
func TestKVRangeKeysOnly(t *testing.T)    { testKVRangeKeysOnly(t, normalRangeFunc) }
func TestKVTxnRangeKeysOnly(t *testing.T) { testKVRangeKeysOnly(t, txnRangeFunc) }

func testKVRangeKeysOnly(t *testing.T, f rangeFunc) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	// Setup test data
	s.Put([]byte("foo"), []byte("bar"), lease.NoLease)
	s.Put([]byte("foo1"), []byte("bar1"), lease.NoLease)
	s.Put([]byte("foo2"), []byte("bar2"), lease.NoLease)

	tests := []struct {
		name     string
		key      []byte
		end      []byte
		keysOnly bool
		wantKVs  []mvccpb.KeyValue
	}{
		{
			name:     "range with values",
			key:      []byte("foo"),
			end:      []byte("foo3"),
			keysOnly: false,
			wantKVs: []mvccpb.KeyValue{
				{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
				{Key: []byte("foo1"), Value: []byte("bar1"), CreateRevision: 3, ModRevision: 3, Version: 1},
				{Key: []byte("foo2"), Value: []byte("bar2"), CreateRevision: 4, ModRevision: 4, Version: 1},
			},
		},
		{
			name:     "range keys only",
			key:      []byte("foo"),
			end:      []byte("foo3"),
			keysOnly: true,
			wantKVs: []mvccpb.KeyValue{
				{Key: []byte("foo"), Value: nil, CreateRevision: 2, ModRevision: 2, Version: 1},
				{Key: []byte("foo1"), Value: nil, CreateRevision: 3, ModRevision: 3, Version: 1},
				{Key: []byte("foo2"), Value: nil, CreateRevision: 4, ModRevision: 4, Version: 1},
			},
		},
		{
			name:     "single key with value",
			key:      []byte("foo1"),
			end:      nil,
			keysOnly: false,
			wantKVs: []mvccpb.KeyValue{
				{Key: []byte("foo1"), Value: []byte("bar1"), CreateRevision: 3, ModRevision: 3, Version: 1},
			},
		},
		{
			name:     "single key keys only",
			key:      []byte("foo1"),
			end:      nil,
			keysOnly: true,
			wantKVs: []mvccpb.KeyValue{
				{Key: []byte("foo1"), Value: nil, CreateRevision: 3, ModRevision: 3, Version: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro := RangeOptions{KeysOnly: tt.keysOnly}
			r, err := f(s, tt.key, tt.end, ro)
			if err != nil {
				t.Fatalf("range failed: %v", err)
			}

			if len(r.KVs) != len(tt.wantKVs) {
				t.Errorf("len(kvs) = %d, want %d", len(r.KVs), len(tt.wantKVs))
			}

			for i, kv := range r.KVs {
				if i >= len(tt.wantKVs) {
					break
				}
				want := tt.wantKVs[i]

				if !reflect.DeepEqual(kv.Key, want.Key) {
					t.Errorf("kv[%d].Key = %v, want %v", i, kv.Key, want.Key)
				}
				if !reflect.DeepEqual(kv.Value, want.Value) {
					t.Errorf("kv[%d].Value = %v, want %v", i, kv.Value, want.Value)
				}
				if kv.CreateRevision != want.CreateRevision {
					t.Errorf("kv[%d].CreateRevision = %d, want %d", i, kv.CreateRevision, want.CreateRevision)
				}
				if kv.ModRevision != want.ModRevision {
					t.Errorf("kv[%d].ModRevision = %d, want %d", i, kv.ModRevision, want.ModRevision)
				}
				if kv.Version != want.Version {
					t.Errorf("kv[%d].Version = %d, want %d", i, kv.Version, want.Version)
				}
			}
		})
	}
}

// TestKVRangeKeysOnlyWithLimit tests KeysOnly with various limits
func TestKVRangeKeysOnlyWithLimit(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	// Setup test data with larger values to test memory efficiency
	largeValue := make([]byte, 1024) // 1KB value
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	s.Put([]byte("key1"), largeValue, lease.NoLease)
	s.Put([]byte("key2"), largeValue, lease.NoLease)
	s.Put([]byte("key3"), largeValue, lease.NoLease)

	tests := []struct {
		name     string
		limit    int64
		keysOnly bool
		wantLen  int
	}{
		{
			name:     "limit 2 with values",
			limit:    2,
			keysOnly: false,
			wantLen:  2,
		},
		{
			name:     "limit 2 keys only",
			limit:    2,
			keysOnly: true,
			wantLen:  2,
		},
		{
			name:     "no limit keys only",
			limit:    0,
			keysOnly: true,
			wantLen:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro := RangeOptions{
				Limit:    tt.limit,
				KeysOnly: tt.keysOnly,
			}
			r, err := normalRangeFunc(s, []byte("key"), []byte("key4"), ro)
			if err != nil {
				t.Fatalf("range failed: %v", err)
			}

			if len(r.KVs) != tt.wantLen {
				t.Errorf("len(kvs) = %d, want %d", len(r.KVs), tt.wantLen)
			}

			for i, kv := range r.KVs {
				if tt.keysOnly {
					if kv.Value != nil {
						t.Errorf("kv[%d].Value should be nil for KeysOnly=true, got %v", i, kv.Value)
					}
				} else {
					if len(kv.Value) != len(largeValue) {
						t.Errorf("kv[%d].Value length = %d, want %d", i, len(kv.Value), len(largeValue))
					}
				}
			}
		})
	}
}

// TestKVRangeKeysOnlyWithRevision tests KeysOnly with different revisions
func TestKVRangeKeysOnlyWithRevision(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	// Setup test data with updates
	s.Put([]byte("foo"), []byte("bar1"), lease.NoLease)    // rev 2
	s.Put([]byte("foo"), []byte("bar2"), lease.NoLease)    // rev 3
	s.Put([]byte("foo2"), []byte("value2"), lease.NoLease) // rev 4

	tests := []struct {
		name     string
		rev      int64
		keysOnly bool
		wantLen  int
		wantVal  []byte
	}{
		{
			name:     "revision 2 with value",
			rev:      2,
			keysOnly: false,
			wantLen:  1,
			wantVal:  []byte("bar1"),
		},
		{
			name:     "revision 2 keys only",
			rev:      2,
			keysOnly: true,
			wantLen:  1,
			wantVal:  nil,
		},
		{
			name:     "revision 4 keys only",
			rev:      4,
			keysOnly: true,
			wantLen:  2,
			wantVal:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro := RangeOptions{
				Rev:      tt.rev,
				KeysOnly: tt.keysOnly,
			}
			r, err := normalRangeFunc(s, []byte("foo"), []byte("foo3"), ro)
			if err != nil {
				t.Fatalf("range failed: %v", err)
			}

			if len(r.KVs) != tt.wantLen {
				t.Errorf("len(kvs) = %d, want %d", len(r.KVs), tt.wantLen)
			}

			if len(r.KVs) > 0 {
				if !reflect.DeepEqual(r.KVs[0].Value, tt.wantVal) {
					t.Errorf("kv[0].Value = %v, want %v", r.KVs[0].Value, tt.wantVal)
				}
			}
		})
	}
}

// TestKVRangeKeysOnlyCount tests KeysOnly with Count option
func TestKVRangeKeysOnlyCount(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	// Setup test data
	s.Put([]byte("foo1"), []byte("bar1"), lease.NoLease)
	s.Put([]byte("foo2"), []byte("bar2"), lease.NoLease)
	s.Put([]byte("foo3"), []byte("bar3"), lease.NoLease)

	// Test count-only operation - KeysOnly should not affect count-only results
	ro := RangeOptions{
		Count:    true,
		KeysOnly: true, // This should be ignored when Count is true
	}
	r, err := normalRangeFunc(s, []byte("foo"), []byte("foo4"), ro)
	if err != nil {
		t.Fatalf("range failed: %v", err)
	}

	if r.KVs != nil {
		t.Errorf("KVs should be nil for count-only operation, got %v", r.KVs)
	}
	if r.Count != 3 {
		t.Errorf("Count = %d, want 3", r.Count)
	}
}
