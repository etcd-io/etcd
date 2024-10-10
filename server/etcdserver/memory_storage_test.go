// Copyright 2024 The etcd Authors
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

package etcdserver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// TestMemoryStorageCompactInclusive tests that compacting is inclusive,
// meaning the first index after compaction is larger by one than compacted index.
func TestMemoryStorageCompactInclusive(t *testing.T) {
	// entries: [ {Index: 0} ]
	raftStorage := raft.NewMemoryStorage()

	firstIndex, err := raftStorage.FirstIndex()
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), firstIndex)

	// after appending, entries should be:
	// [ {Index: 0}, {Index: 1}, {Index: 2}, {Index: 3}, {Index: 4}, {Index: 5} ]
	appliedIndex := uint64(1)
	for ; appliedIndex <= 5; appliedIndex++ {
		e := raftpb.Entry{
			Type:  raftpb.EntryNormal,
			Term:  1,
			Index: appliedIndex,
		}
		err := raftStorage.Append([]raftpb.Entry{e})
		assert.NoError(t, err)
	}

	firstIndex, err = raftStorage.FirstIndex()
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), firstIndex)

	lastIndex, err := raftStorage.LastIndex()
	assert.NoError(t, err)
	assert.Equal(t, uint64(5), lastIndex)

	// after compacting, entries should be:
	// [ {Index: 3}, {Index: 4}, {Index: 5} ]
	compacti := uint64(3)
	err = raftStorage.Compact(compacti)
	assert.NoError(t, err)


	firstIndex, err = raftStorage.FirstIndex()
	assert.NoError(t, err)
	assert.Equal(t, compacti+1, firstIndex)
}
