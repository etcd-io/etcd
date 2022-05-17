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

package mvcc

import (
	"hash"
	"hash/crc32"

	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
)

func unsafeHashByRev(tx backend.ReadTx, lower, upper revision, keep map[revision]struct{}) (uint32, error) {
	h := newKVHasher()
	err := tx.UnsafeForEach(buckets.Key, func(k, v []byte) error {
		kr := bytesToRev(k)
		if !upper.GreaterThan(kr) {
			return nil
		}
		// skip revisions that are scheduled for deletion
		// due to compacting; don't skip if there isn't one.
		if lower.GreaterThan(kr) && len(keep) > 0 {
			if _, ok := keep[kr]; !ok {
				return nil
			}
		}
		h.WriteKeyValue(k, v)
		return nil
	})
	return h.Hash(), err
}

type hasher struct {
	h hash.Hash32
}

func newKVHasher() hasher {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	h.Write(buckets.Key.Name())
	return hasher{h}
}

func (h *hasher) WriteKeyValue(k, v []byte) {
	h.h.Write(k)
	h.h.Write(v)
}

func (h *hasher) Hash() uint32 {
	return h.h.Sum32()
}
