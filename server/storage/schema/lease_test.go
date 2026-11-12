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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"go.etcd.io/etcd/server/v3/lease/leasepb"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

func TestLeaseBackend(t *testing.T) {
	tcs := []struct {
		name  string
		setup func(tx backend.UnsafeWriter)
		want  []*leasepb.Lease
	}{
		{
			name:  "Empty by default",
			setup: func(tx backend.UnsafeWriter) {},
			want:  []*leasepb.Lease{},
		},
		{
			name: "Returns data put before",
			setup: func(tx backend.UnsafeWriter) {
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
			setup: func(tx backend.UnsafeWriter) {
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
			lg := zaptest.NewLogger(t)
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			tx.Lock()
			UnsafeCreateLeaseBucket(tx)
			tc.setup(tx)
			tx.Unlock()

			be.ForceCommit()
			be.Close()

			be2 := backend.NewDefaultBackend(lg, tmpPath)
			defer be2.Close()
			leases, err := UnsafeGetAllLeases(be2.ReadTx())
			if err != nil {
				t.Fatalf("unexpected error getting leases (%v)", err)
			}

			if diff := cmp.Diff(tc.want, leases, protocmp.Transform()); diff != "" {
				t.Fatalf("leases mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestUnsafeGetAllLeasesCorrupted ensures that corrupted lease bucket
// records surface as errors instead of panicking the process
// (https://github.com/etcd-io/etcd/issues/22748).
func TestUnsafeGetAllLeasesCorrupted(t *testing.T) {
	validValue, err := proto.Marshal(&leasepb.Lease{ID: 0x11, TTL: 2})
	if err != nil {
		t.Fatalf("could not marshal lease (%v)", err)
	}
	tcs := []struct {
		name    string
		key     []byte
		value   []byte
		wantErr string
	}{
		{
			name:    "Unparseable value with valid key",
			key:     leaseIDToBytes(0x11),
			value:   []byte{0xff, 0xff},
			wantErr: "failed to Unmarshal lease proto item; lease ID=0000000000000011",
		},
		{
			name:    "Short key with unparseable value",
			key:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11},
			value:   []byte{0xff, 0xff},
			wantErr: "lease key has unexpected length 7, want 8 bytes; lease key=00000000000011",
		},
		{
			name:    "Short key with valid value",
			key:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11},
			value:   validValue,
			wantErr: "lease key has unexpected length 7, want 8 bytes",
		},
		{
			name:    "Long key with valid value",
			key:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11, 0x00},
			value:   validValue,
			wantErr: "lease key has unexpected length 9, want 8 bytes",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			tx.Lock()
			UnsafeCreateLeaseBucket(tx)
			tx.UnsafePut(Lease, tc.key, tc.value)
			tx.Unlock()

			be.ForceCommit()
			be.Close()

			be2 := backend.NewDefaultBackend(lg, tmpPath)
			defer be2.Close()
			leases, err := UnsafeGetAllLeases(be2.ReadTx())
			if err == nil {
				t.Fatalf("expected error for corrupted lease record, got nil (leases=%v)", leases)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
