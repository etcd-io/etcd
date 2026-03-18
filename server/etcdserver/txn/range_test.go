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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

// TestRangeWithSortAndLimit tests that Range returns correct results
// with various sort orders, sort targets, and limits.
// This validates that the optimization for KEY ASCEND sorting with limit
// does not break correctness.
func TestRangeWithSortAndLimit(t *testing.T) {
	tests := []struct {
		name      string
		keys      []string // keys to insert (in order)
		request   *pb.RangeRequest
		wantKeys  []string
		wantMore  bool
		wantCount int64
	}{
		{
			name: "no sort, no limit - returns all keys in KEY ASCEND order",
			keys: []string{"c", "a", "b"},
			request: &pb.RangeRequest{
				Key:      []byte{0},
				RangeEnd: []byte{0},
			},
			wantKeys:  []string{"a", "b", "c"},
			wantMore:  false,
			wantCount: 3,
		},
		{
			name: "no sort, with limit - returns limited keys in KEY ASCEND order",
			keys: []string{"c", "a", "b", "d", "e"},
			request: &pb.RangeRequest{
				Key:      []byte{0},
				RangeEnd: []byte{0},
				Limit:    2,
			},
			wantKeys:  []string{"a", "b"},
			wantMore:  true,
			wantCount: 5,
		},
		{
			name: "KEY ASCEND sort, with limit - optimization should work correctly",
			keys: []string{"c", "a", "b", "d", "e"},
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      2,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
			wantKeys:  []string{"a", "b"},
			wantMore:  true,
			wantCount: 5,
		},
		{
			name: "KEY ASCEND sort, limit equals total keys",
			keys: []string{"c", "a", "b"},
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      3,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
			wantKeys:  []string{"a", "b", "c"},
			wantMore:  false,
			wantCount: 3,
		},
		{
			name: "KEY ASCEND sort, limit greater than total keys",
			keys: []string{"c", "a", "b"},
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      10,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
			wantKeys:  []string{"a", "b", "c"},
			wantMore:  false,
			wantCount: 3,
		},
		{
			name: "KEY DESCEND sort, with limit - should fetch all and sort",
			keys: []string{"c", "a", "b", "d", "e"},
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      2,
				SortOrder:  pb.RangeRequest_DESCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
			wantKeys:  []string{"e", "d"},
			wantMore:  true,
			wantCount: 5,
		},
		{
			name: "CREATE ASCEND sort, with limit - should fetch all and sort",
			keys: []string{"c", "a", "b", "d", "e"},
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      2,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_CREATE,
			},
			wantKeys:  []string{"c", "a"}, // order of creation
			wantMore:  true,
			wantCount: 5,
		},
		{
			name: "MOD DESCEND sort, with limit - should fetch all and sort",
			keys: []string{"c", "a", "b", "d", "e"},
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      2,
				SortOrder:  pb.RangeRequest_DESCEND,
				SortTarget: pb.RangeRequest_MOD,
			},
			wantKeys:  []string{"e", "d"}, // reverse order of modification
			wantMore:  true,
			wantCount: 5,
		},
		{
			name: "KEY ASCEND with MinModRevision - should fetch all due to filter",
			keys: []string{"c", "a", "b", "d", "e"},
			request: &pb.RangeRequest{
				Key:            []byte{0},
				RangeEnd:       []byte{0},
				Limit:          2,
				SortOrder:      pb.RangeRequest_ASCEND,
				SortTarget:     pb.RangeRequest_KEY,
				MinModRevision: 3, // filter: only keys with ModRevision >= 3
			},
			wantKeys:  []string{"a", "b"}, // a (rev 3), b (rev 4), d (rev 5), e (rev 6) - first 2
			wantMore:  true,
			wantCount: 5, // count is total before filtering
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := betesting.NewDefaultTmpBackend(t)
			t.Cleanup(func() {
				betesting.Close(t, b)
			})
			s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
			t.Cleanup(func() {
				s.Close()
			})

			// Insert keys in the specified order
			for _, key := range tt.keys {
				s.Put([]byte(key), []byte("value-"+key), lease.NoLease)
			}

			ctx := context.Background()
			resp, _, err := Range(ctx, zaptest.NewLogger(t), s, tt.request)
			require.NoError(t, err)

			// Verify returned keys
			gotKeys := make([]string, len(resp.Kvs))
			for i, kv := range resp.Kvs {
				gotKeys[i] = string(kv.Key)
			}
			assert.Equalf(t, tt.wantKeys, gotKeys, "returned keys mismatch")
			assert.Equalf(t, tt.wantMore, resp.More, "More flag mismatch")
			assert.Equalf(t, tt.wantCount, resp.Count, "Count mismatch")
		})
	}
}

// TestRangeKeyAscendOptimizationCorrectness specifically tests that the
// KEY ASCEND optimization produces the same results as the non-optimized path.
func TestRangeKeyAscendOptimizationCorrectness(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	t.Cleanup(func() {
		betesting.Close(t, b)
	})
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	t.Cleanup(func() {
		s.Close()
	})

	// Insert 100 keys in random order
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("key-%03d", (i*17)%100) // pseudo-random order
	}
	for _, key := range keys {
		s.Put([]byte(key), []byte("value"), lease.NoLease)
	}

	ctx := context.Background()
	lg := zaptest.NewLogger(t)

	// Test various limits with KEY ASCEND (optimized path)
	for _, limit := range []int64{1, 5, 10, 50, 99, 100, 150} {
		t.Run(fmt.Sprintf("limit-%d", limit), func(t *testing.T) {
			// Request with KEY ASCEND (uses optimization)
			reqAscend := &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      limit,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_KEY,
			}

			// Request with no sort (natural order, same as KEY ASCEND)
			reqNoSort := &pb.RangeRequest{
				Key:      []byte{0},
				RangeEnd: []byte{0},
				Limit:    limit,
			}

			respAscend, _, err := Range(ctx, lg, s, reqAscend)
			require.NoError(t, err)

			respNoSort, _, err := Range(ctx, lg, s, reqNoSort)
			require.NoError(t, err)

			// Results should be identical
			assert.Lenf(t, respAscend.Kvs, len(respNoSort.Kvs), "number of keys mismatch")
			for i := range respNoSort.Kvs {
				assert.Equalf(t, string(respNoSort.Kvs[i].Key), string(respAscend.Kvs[i].Key),
					"key mismatch at index %d", i)
			}
			assert.Equalf(t, respNoSort.More, respAscend.More, "More flag mismatch")
			assert.Equalf(t, respNoSort.Count, respAscend.Count, "Count mismatch")
		})
	}
}

// BenchmarkRange benchmarks Range requests with different configurations.
// This helps demonstrate the performance improvement from the KEY ASCEND optimization.
func BenchmarkRange(b *testing.B) {
	benchmarks := []struct {
		name    string
		request *pb.RangeRequest
	}{
		{
			name: "no_sort_no_limit",
			request: &pb.RangeRequest{
				Key:      []byte{0},
				RangeEnd: []byte{0},
			},
		},
		{
			name: "no_sort_limit_10",
			request: &pb.RangeRequest{
				Key:      []byte{0},
				RangeEnd: []byte{0},
				Limit:    10,
			},
		},
		{
			name: "key_ascend_limit_10",
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      10,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
		},
		{
			name: "key_descend_limit_10",
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      10,
				SortOrder:  pb.RangeRequest_DESCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
		},
		{
			name: "create_ascend_limit_10",
			request: &pb.RangeRequest{
				Key:        []byte{0},
				RangeEnd:   []byte{0},
				Limit:      10,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_CREATE,
			},
		},
	}

	for _, keyCount := range []int{100, 1000, 10000} {
		for _, bm := range benchmarks {
			b.Run(fmt.Sprintf("keys_%d/%s", keyCount, bm.name), func(b *testing.B) {
				be, _ := betesting.NewDefaultTmpBackend(b)
				s := mvcc.NewStore(zaptest.NewLogger(b), be, &lease.FakeLessor{}, mvcc.StoreConfig{})
				defer func() {
					s.Close()
					betesting.Close(b, be)
				}()

				// Insert keys
				for i := 0; i < keyCount; i++ {
					s.Put([]byte(fmt.Sprintf("key-%05d", i)), []byte("value"), lease.NoLease)
				}
				s.Commit()

				ctx := context.Background()
				lg := zaptest.NewLogger(b)

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _, err := Range(ctx, lg, s, bm.request)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
