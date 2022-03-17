// Copyright 2022 The etcd Authors
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

package schema

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/server/v3/lease/leasepb"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

func TestLeaseBackend(t *testing.T) {
	tcs := []struct {
		name  string
		setup func(tx backend.BatchTx)
		want  []*leasepb.Lease
	}{
		{
			name:  "Empty by default",
			setup: func(tx backend.BatchTx) {},
			want:  []*leasepb.Lease{},
		},
		{
			name: "Returns data put before",
			setup: func(tx backend.BatchTx) {
				MustUnsafePutLease(tx, &leasepb.Lease{
					ID:  -1,
					TTL: 2,
				})
			},
			want: []*leasepb.Lease{
				{
					ID:  -1,
					TTL: 2,
				},
			},
		},
		{
			name: "Skips deleted",
			setup: func(tx backend.BatchTx) {
				MustUnsafePutLease(tx, &leasepb.Lease{
					ID:  -1,
					TTL: 2,
				})
				MustUnsafePutLease(tx, &leasepb.Lease{
					ID:  math.MinInt64,
					TTL: 2,
				})
				MustUnsafePutLease(tx, &leasepb.Lease{
					ID:  math.MaxInt64,
					TTL: 3,
				})
				UnsafeDeleteLease(tx, &leasepb.Lease{
					ID:  -1,
					TTL: 2,
				})
			},
			want: []*leasepb.Lease{
				{
					ID:  math.MaxInt64,
					TTL: 3,
				},
				{
					ID:  math.MinInt64, // bytes bigger than MaxInt64
					TTL: 2,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			tx.Lock()
			UnsafeCreateLeaseBucket(tx)
			tc.setup(tx)
			tx.Unlock()

			be.ForceCommit()
			be.Close()

			be2 := backend.NewDefaultBackend(tmpPath)
			defer be2.Close()
			leases := MustUnsafeGetAllLeases(be2.ReadTx())

			assert.Equal(t, tc.want, leases)
		})
	}
}
