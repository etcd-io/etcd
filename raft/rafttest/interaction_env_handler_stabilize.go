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
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func (env *InteractionEnv) handleStabilize(t *testing.T, d datadriven.TestData) error {
	idxs := nodeIdxs(t, d)
	return env.Stabilize(idxs...)
}

// Stabilize repeatedly runs Ready handling on and message delivery to the set
// of nodes specified via the idxs slice until reaching a fixed point.
func (env *InteractionEnv) Stabilize(idxs ...int) error {
	var nodes []Node
	for _, idx := range idxs {
		nodes = append(nodes, env.Nodes[idx])
	}
	if len(nodes) == 0 {
		nodes = env.Nodes
	}

	for {
		done := true
		for _, rn := range nodes {
			if rn.HasReady() {
				done = false
				idx := int(rn.Status().ID - 1)
				fmt.Fprintf(env.Output, "> %d handling Ready\n", idx+1)
				env.withIndent(func() { env.ProcessReady(idx) })
			}
		}
		for _, rn := range nodes {
			id := rn.Status().ID
			// NB: we grab the messages just to see whether to print the header.
			// DeliverMsgs will do it again.
			if msgs, _ := splitMsgs(env.Messages, id); len(msgs) > 0 {
				fmt.Fprintf(env.Output, "> %d receiving messages\n", id)
				env.withIndent(func() { env.DeliverMsgs(Recipient{ID: id}) })
				done = false
			}
		}
		if done {
			return nil
		}
	}
}

func splitMsgs(msgs []raftpb.Message, to uint64) (toMsgs []raftpb.Message, rmdr []raftpb.Message) {
	// NB: this method does not reorder messages.
	for _, msg := range msgs {
		if msg.To == to {
			toMsgs = append(toMsgs, msg)
		} else {
			rmdr = append(rmdr, msg)
		}
	}
	return toMsgs, rmdr
}
