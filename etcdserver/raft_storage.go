// Copyright 2015 CoreOS, Inc.
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
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
)

type raftStorage struct {
	*raft.MemoryStorage
	// snapStore is the place to request snapshot when v3demo is enabled.
	// If snapStore is nil, it uses the snapshot saved in MemoryStorage.
	snapStore *snapshotStore
}

func newRaftStorage() *raftStorage {
	return &raftStorage{
		MemoryStorage: raft.NewMemoryStorage(),
	}
}

func (rs *raftStorage) reqsnap() <-chan struct{} {
	if rs.snapStore == nil {
		return nil
	}
	return rs.snapStore.reqsnapc
}

func (rs *raftStorage) raftsnap() chan<- raftpb.Snapshot {
	if rs.snapStore == nil {
		return nil
	}
	return rs.snapStore.raftsnapc
}

// Snapshot returns raft snapshot. If snapStore is nil, this method
// returns snapshot saved in MemoryStorage. If snapStore exists, this method
// returns snapshot from snapStore.
func (rs *raftStorage) Snapshot() (raftpb.Snapshot, error) {
	if rs.snapStore == nil {
		return rs.MemoryStorage.Snapshot()
	}
	snap, err := rs.snapStore.getSnap()
	if err != nil {
		return raftpb.Snapshot{}, err
	}
	return snap.raft(), nil
}
