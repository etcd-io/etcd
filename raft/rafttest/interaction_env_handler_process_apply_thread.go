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

package rafttest

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func (env *InteractionEnv) handleProcessApplyThread(t *testing.T, d datadriven.TestData) error {
	idxs := nodeIdxs(t, d)
	for _, idx := range idxs {
		var err error
		if len(idxs) > 1 {
			fmt.Fprintf(env.Output, "> %d processing apply thread\n", idx+1)
			env.withIndent(func() { err = env.ProcessApplyThread(idx) })
		} else {
			err = env.ProcessApplyThread(idx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ProcessApplyThread runs processes a single message on the "apply" thread of
// the node with the given index.
func (env *InteractionEnv) ProcessApplyThread(idx int) error {
	n := &env.Nodes[idx]
	if len(n.ApplyWork) == 0 {
		env.Output.WriteString("no apply work to perform")
		return nil
	}
	m := n.ApplyWork[0]
	n.ApplyWork = n.ApplyWork[1:]

	resps := m.Responses
	m.Responses = nil
	env.Output.WriteString("Processing:\n")
	env.Output.WriteString(raft.DescribeMessage(m, defaultEntryFormatter) + "\n")
	if err := processApply(n, m.Entries); err != nil {
		return err
	}

	env.Output.WriteString("Responses:\n")
	for _, m := range resps {
		env.Output.WriteString(raft.DescribeMessage(m, defaultEntryFormatter) + "\n")
	}
	env.Messages = append(env.Messages, resps...)
	return nil
}

func processApply(n *Node, ents []raftpb.Entry) error {
	for _, ent := range ents {
		var update []byte
		var cs *raftpb.ConfState
		switch ent.Type {
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			if err := cc.Unmarshal(ent.Data); err != nil {
				return err
			}
			update = cc.Context
			cs = n.RawNode.ApplyConfChange(cc)
		case raftpb.EntryConfChangeV2:
			var cc raftpb.ConfChangeV2
			if err := cc.Unmarshal(ent.Data); err != nil {
				return err
			}
			cs = n.RawNode.ApplyConfChange(cc)
			update = cc.Context
		default:
			update = ent.Data
		}

		// Record the new state by starting with the current state and applying
		// the command.
		lastSnap := n.History[len(n.History)-1]
		var snap raftpb.Snapshot
		snap.Data = append(snap.Data, lastSnap.Data...)
		// NB: this hard-codes an "appender" state machine.
		snap.Data = append(snap.Data, update...)
		snap.Metadata.Index = ent.Index
		snap.Metadata.Term = ent.Term
		if cs == nil {
			sl := n.History
			cs = &sl[len(sl)-1].Metadata.ConfState
		}
		snap.Metadata.ConfState = *cs
		n.History = append(n.History, snap)
	}
	return nil
}
