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
	"fmt"
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func (env *InteractionEnv) handleProcessReady(t *testing.T, d datadriven.TestData) error {
	idxs := nodeIdxs(t, d)
	for _, idx := range idxs {
		var err error
		if len(idxs) > 1 {
			fmt.Fprintf(env.Output, "> %d handling Ready\n", idx+1)
			env.withIndent(func() { err = env.ProcessReady(idx) })
		} else {
			err = env.ProcessReady(idx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ProcessReady runs Ready handling on the node with the given index.
func (env *InteractionEnv) ProcessReady(idx int) error {
	// TODO(tbg): Allow simulating crashes here.
	n := &env.Nodes[idx]
	rd := n.Ready()
	env.Output.WriteString(raft.DescribeReady(rd, defaultEntryFormatter))

	if !n.Config.AsyncStorageWrites {
		if err := processAppend(n, rd.HardState, rd.Entries, rd.Snapshot); err != nil {
			return err
		}
		if err := processApply(n, rd.CommittedEntries); err != nil {
			return err
		}
	}

	for _, m := range rd.Messages {
		if raft.IsLocalMsgTarget(m.To) {
			if !n.Config.AsyncStorageWrites {
				panic("unexpected local msg target")
			}
			switch m.Type {
			case raftpb.MsgStorageAppend:
				n.AppendWork = append(n.AppendWork, m)
			case raftpb.MsgStorageApply:
				n.ApplyWork = append(n.ApplyWork, m)
			default:
				panic(fmt.Sprintf("unexpected message type %s", m.Type))
			}
		} else {
			env.Messages = append(env.Messages, m)
		}
	}

	if !n.Config.AsyncStorageWrites {
		n.Advance(rd)
	}
	return nil
}
