// Copyright 2021 The etcd Authors
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
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	"go.etcd.io/etcd/server/v3/lease/leasepb"
	"go.etcd.io/etcd/server/v3/storage/backend"
)

func UnsafeCreateLeaseBucket(tx backend.UnsafeWriter) {
	tx.UnsafeCreateBucket(Lease)
}

func UnsafeGetAllLeases(tx backend.UnsafeReader) ([]*leasepb.Lease, error) {
	ls := make([]*leasepb.Lease, 0)
	err := tx.UnsafeForEach(Lease, func(k, v []byte) error {
		if len(k) != 8 {
			return fmt.Errorf("lease key has unexpected length %d, want 8 bytes; lease key=%x", len(k), k)
		}
		var lpb leasepb.Lease
		err := proto.Unmarshal(v, &lpb)
		if err != nil {
			return fmt.Errorf("failed to Unmarshal lease proto item; lease ID=%016x: %w", bytesToLeaseID(k), err)
		}
		ls = append(ls, &lpb)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ls, nil
}

func MustUnsafePutLease(tx backend.UnsafeWriter, lpb *leasepb.Lease) {
	key := leaseIDToBytes(lpb.ID)

	val, err := proto.Marshal(lpb)
	if err != nil {
		panic("failed to marshal lease proto item")
	}
	tx.UnsafePut(Lease, key, val)
}

func UnsafeDeleteLease(tx backend.UnsafeWriter, lpb *leasepb.Lease) {
	tx.UnsafeDelete(Lease, leaseIDToBytes(lpb.ID))
}

func MustUnsafeGetLease(tx backend.UnsafeReader, leaseID int64) *leasepb.Lease {
	_, vs := tx.UnsafeRange(Lease, leaseIDToBytes(leaseID), nil, 0)
	if len(vs) != 1 {
		return nil
	}
	var lpb leasepb.Lease
	err := proto.Unmarshal(vs[0], &lpb)
	if err != nil {
		panic("failed to unmarshal lease proto item")
	}
	return &lpb
}

func leaseIDToBytes(n int64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(n))
	return bytes
}

func bytesToLeaseID(bytes []byte) int64 {
	if len(bytes) != 8 {
		panic(fmt.Errorf("lease ID must be 8-byte"))
	}
	return int64(binary.BigEndian.Uint64(bytes))
}
