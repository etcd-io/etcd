// Copyright 2019 The etcd Authors
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

package rafttest

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft"
	pb "go.etcd.io/etcd/raft/raftpb"
)

func (env *InteractionEnv) handleAddNodes(t *testing.T, d datadriven.TestData) error {
	n := firstAsInt(t, d)
	var snap pb.Snapshot
	for _, arg := range d.CmdArgs[1:] {
		for i := range arg.Vals {
			switch arg.Key {
			case "voters":
				var id uint64
				arg.Scan(t, i, &id)
				snap.Metadata.ConfState.Voters = append(snap.Metadata.ConfState.Voters, id)
			case "learners":
				var id uint64
				arg.Scan(t, i, &id)
				snap.Metadata.ConfState.Learners = append(snap.Metadata.ConfState.Learners, id)
			case "index":
				arg.Scan(t, i, &snap.Metadata.Index)
			case "content":
				arg.Scan(t, i, &snap.Data)
			}
		}
	}
	return env.AddNodes(n, snap)
}

type snapOverrideStorage struct {
	Storage
	snapshotOverride func() (pb.Snapshot, error)
}

func (s snapOverrideStorage) Snapshot() (pb.Snapshot, error) {
	if s.snapshotOverride != nil {
		return s.snapshotOverride()
	}
	return s.Storage.Snapshot()
}

var _ raft.Storage = snapOverrideStorage{}

// AddNodes adds n new nodes initializes from the given snapshot (which may be
// empty). They will be assigned consecutive IDs.
func (env *InteractionEnv) AddNodes(n int, snap pb.Snapshot) error {
	bootstrap := !reflect.DeepEqual(snap, pb.Snapshot{})
	for i := 0; i < n; i++ {
		id := uint64(1 + len(env.Nodes))
		s := snapOverrideStorage{
			Storage: raft.NewMemoryStorage(),
			// When you ask for a snapshot, you get the most recent snapshot.
			//
			// TODO(tbg): this is sort of clunky, but MemoryStorage itself will
			// give you some fixed snapshot and also the snapshot changes
			// whenever you compact the logs and vice versa, so it's all a bit
			// awkward to use.
			snapshotOverride: func() (pb.Snapshot, error) {
				snaps := env.Nodes[int(id-1)].History
				return snaps[len(snaps)-1], nil
			},
		}
		if bootstrap {
			// NB: we could make this work with 1, but MemoryStorage just
			// doesn't play well with that and it's not a loss of generality.
			if snap.Metadata.Index <= 1 {
				return errors.New("index must be specified as > 1 due to bootstrap")
			}
			snap.Metadata.Term = 1
			if err := s.ApplySnapshot(snap); err != nil {
				return err
			}
			fi, err := s.FirstIndex()
			if err != nil {
				return err
			}
			// At the time of writing and for *MemoryStorage, applying a
			// snapshot also truncates appropriately, but this would change with
			// other storage engines potentially.
			if exp := snap.Metadata.Index + 1; fi != exp {
				return fmt.Errorf("failed to establish first index %d; got %d", exp, fi)
			}
		}
		cfg := defaultRaftConfig(id, snap.Metadata.Index, s)
		if env.Options.OnConfig != nil {
			env.Options.OnConfig(cfg)
			if cfg.ID != id {
				// This could be supported but then we need to do more work
				// translating back and forth -- not worth it.
				return errors.New("OnConfig must not change the ID")
			}
		}
		if cfg.Logger != nil {
			return errors.New("OnConfig must not set Logger")
		}
		cfg.Logger = env.Output

		rn, err := raft.NewRawNode(cfg)
		if err != nil {
			return err
		}

		node := Node{
			RawNode: rn,
			// TODO(tbg): allow a more general Storage, as long as it also allows
			// us to apply snapshots, append entries, and update the HardState.
			Storage: s,
			Config:  cfg,
			History: []pb.Snapshot{snap},
		}
		env.Nodes = append(env.Nodes, node)
	}
	return nil
}
