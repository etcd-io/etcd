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

	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

// BenchmarkRange benchmarks Range requests with different configurations.
// This helps demonstrate the performance improvement from the KEY ASCEND optimization.
func BenchmarkRange(b *testing.B) {
	benchmarks := []struct {
		name       string
		limit      int64
		sortOrder  pb.RangeRequest_SortOrder
		sortTarget pb.RangeRequest_SortTarget
	}{
		{
			name:  "no_sort_no_limit",
			limit: 0,
		},
		{
			name:  "no_sort_limit_10",
			limit: 10,
		},
		{
			name:       "key_ascend_limit_10",
			limit:      10,
			sortOrder:  pb.RangeRequest_ASCEND,
			sortTarget: pb.RangeRequest_KEY,
		},
		{
			name:       "key_descend_limit_10",
			limit:      10,
			sortOrder:  pb.RangeRequest_DESCEND,
			sortTarget: pb.RangeRequest_KEY,
		},
		{
			name:       "create_ascend_limit_10",
			limit:      10,
			sortOrder:  pb.RangeRequest_ASCEND,
			sortTarget: pb.RangeRequest_CREATE,
		},
	}

	for _, keyCount := range []int{100, 1000, 10000} {
		for _, bm := range benchmarks {
			b.Run(fmt.Sprintf("keys_%d/%s", keyCount, bm.name), func(b *testing.B) {
				be, _ := betesting.NewDefaultTmpBackend(b)
				defer betesting.Close(b, be)
				s := mvcc.NewStore(zaptest.NewLogger(b), be, &lease.FakeLessor{}, mvcc.StoreConfig{})
				defer s.Close()

				// Insert keys
				for i := range keyCount {
					s.Put([]byte(fmt.Sprintf("key-%05d", i)), []byte("value"), lease.NoLease)
				}
				s.Commit()

				ctx := context.Background()
				lg := zaptest.NewLogger(b)
				req := &pb.RangeRequest{
					Key:        []byte{0},
					RangeEnd:   []byte{0},
					Limit:      bm.limit,
					SortOrder:  bm.sortOrder,
					SortTarget: bm.sortTarget,
				}

				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_, _, err := Range(ctx, lg, s, req)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
