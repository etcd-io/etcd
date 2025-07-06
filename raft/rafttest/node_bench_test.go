// Copyright 2015 The etcd Authors
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
	"context"
	"testing"
	"time"

	"go.etcd.io/etcd/raft/v3"
)

func BenchmarkProposal3Nodes(b *testing.B) {
	peers := []raft.Peer{{ID: 1, Context: nil}, {ID: 2, Context: nil}, {ID: 3, Context: nil}}
	nt := newRaftNetwork(1, 2, 3)

	nodes := make([]*node, 0)

	for i := 1; i <= 3; i++ {
		n := startNode(uint64(i), peers, nt.nodeNetwork(uint64(i)))
		nodes = append(nodes, n)
	}
	// get ready and warm up
	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes[0].Propose(context.TODO(), []byte("somedata"))
	}

	for _, n := range nodes {
		if n.state.Commit != uint64(b.N+4) {
			continue
		}
	}
	b.StopTimer()

	for _, n := range nodes {
		n.stop()
	}
}
