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

package cache

import (
	"github.com/google/btree"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

// snapshot captures a full, point-in-time view of the KV state at rev.
type snapshot struct {
	rev  int64
	tree *btree.BTree
}

func newClonedSnapshot(rev int64, t *btree.BTree) *snapshot {
	return &snapshot{rev: rev, tree: t.Clone()}
}

func (s *snapshot) Range(startKey, endKey []byte) []*mvccpb.KeyValue {
	var out []*mvccpb.KeyValue
	switch {
	case len(endKey) == 0:
		if item := s.tree.Get(probeItemFromKey(startKey)); item != nil {
			out = append(out, item.(*kvItem).kv)
		}
	case isPrefixScan(endKey):
		s.tree.AscendGreaterOrEqual(probeItemFromKey(startKey), func(item btree.Item) bool {
			out = append(out, item.(*kvItem).kv)
			return true
		})
	default:
		s.tree.AscendRange(probeItemFromKey(startKey), probeItemFromKey(endKey), func(item btree.Item) bool {
			out = append(out, item.(*kvItem).kv)
			return true
		})
	}
	return out
}

func isPrefixScan(endKey []byte) bool {
	return len(endKey) == 1 && endKey[0] == 0
}

func probeItemFromKey(key []byte) *kvItem { return &kvItem{key: string(key)} }
