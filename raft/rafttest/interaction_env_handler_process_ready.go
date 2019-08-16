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
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
)

func (env *InteractionEnv) handleProcessReady(t *testing.T, d datadriven.TestData) error {
	idx := firstAsNodeIdx(t, d)
	return env.ProcessReady(idx)
}

// ProcessReady runs Ready handling on the node with the given index.
func (env *InteractionEnv) ProcessReady(idx int) error {
	// TODO(tbg): Allow simulating crashes here.
	rn, s := env.Nodes[idx].RawNode, env.Nodes[idx].Storage
	rd := rn.Ready()
	env.Output.WriteString(raft.DescribeReady(rd, defaultEntryFormatter))
	// TODO(tbg): the order of operations here is not necessarily safe. See:
	// https://github.com/etcd-io/etcd/pull/10861
	if !raft.IsEmptyHardState(rd.HardState) {
		if err := s.SetHardState(rd.HardState); err != nil {
			return err
		}
	}
	if err := s.Append(rd.Entries); err != nil {
		return err
	}
	if !raft.IsEmptySnap(rd.Snapshot) {
		if err := s.ApplySnapshot(rd.Snapshot); err != nil {
			return err
		}
	}
	for _, ent := range rd.CommittedEntries {
		var update []byte
		var cs *raftpb.ConfState
		switch ent.Type {
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			if err := cc.Unmarshal(ent.Data); err != nil {
				return err
			}
			update = cc.Context
			cs = rn.ApplyConfChange(cc)
		case raftpb.EntryConfChangeV2:
			var cc raftpb.ConfChangeV2
			if err := cc.Unmarshal(ent.Data); err != nil {
				return err
			}
			cs = rn.ApplyConfChange(cc)
			update = cc.Context
		default:
			update = ent.Data
		}

		// Record the new state by starting with the current state and applying
		// the command.
		lastSnap := env.Nodes[idx].History[len(env.Nodes[idx].History)-1]
		var snap raftpb.Snapshot
		snap.Data = append(snap.Data, lastSnap.Data...)
		// NB: this hard-codes an "appender" state machine.
		snap.Data = append(snap.Data, update...)
		snap.Metadata.Index = ent.Index
		snap.Metadata.Term = ent.Term
		if cs == nil {
			sl := env.Nodes[idx].History
			cs = &sl[len(sl)-1].Metadata.ConfState
		}
		snap.Metadata.ConfState = *cs
		env.Nodes[idx].History = append(env.Nodes[idx].History, snap)
	}
	for _, msg := range rd.Messages {
		env.Messages = append(env.Messages, msg)
	}
	rn.Advance(rd)
	return nil
}
