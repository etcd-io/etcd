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

func unsafeHashByRev(tx backend.ReadTx, lower, upper int64, keep map[revision]struct{}) (uint32, error) {
	h := newKVHasher(lower, upper, keep)
	err := tx.UnsafeForEach(buckets.Key, func(k, v []byte) error {
		h.WriteKeyValue(k, v)
		return nil
	})
	return h.Hash(), err
}

type kvHasher struct {
	hash         hash.Hash32
	lower, upper revision
	keep         map[revision]struct{}
}

func newKVHasher(lower, upper int64, keep map[revision]struct{}) kvHasher {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	h.Write(buckets.Key.Name())
	return kvHasher{
		hash:  h,
		lower: revision{main: lower},
		upper: revision{main: upper},
		keep:  keep,
	}
}

func (h *kvHasher) WriteKeyValue(k, v []byte) {
	kr := bytesToRev(k)
	if !h.upper.GreaterThan(kr) {
		return
	}
	// skip revisions that are scheduled for deletion
	// due to compacting; don't skip if there isn't one.
	if h.lower.GreaterThan(kr) && len(h.keep) > 0 {
		if _, ok := h.keep[kr]; !ok {
			return
		}
	}
	h.hash.Write(k)
	h.hash.Write(v)
}

func (h *kvHasher) Hash() uint32 {
	return h.hash.Sum32()
}
