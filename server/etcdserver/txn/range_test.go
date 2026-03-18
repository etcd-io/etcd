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
