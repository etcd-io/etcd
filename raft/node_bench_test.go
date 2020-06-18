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

package raft

import (
	"context"
	pb "go.etcd.io/etcd/v3/raft/raftpb"
	"sync"
	"testing"
	"time"
)

func BenchmarkOneNode(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewMemoryStorage()
	rn := newTestRawNode(1, []uint64{1}, 10, 1, s)
	n := newNode(rn)
	go n.run()

	defer n.Stop()

	n.Campaign(ctx)
	go func() {
		for i := 0; i < b.N; i++ {
			n.Propose(ctx, []byte("foo"))
		}
	}()

	for {
		rd := <-n.Ready()
		s.Append(rd.Entries)
		// a reasonable disk sync latency
		time.Sleep(1 * time.Millisecond)
		n.Advance()
		if rd.HardState.Commit == uint64(b.N+1) {
			return
		}
	}
}

type tNode struct {
	node *node
	s    *MemoryStorage
}

func BenchmarkCluster(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peers := []uint64{1, 2, 3}
	nodes := make(map[uint64]*tNode)
	for _, peer := range peers {
		s := NewMemoryStorage()
		cfg := newTestConfig(peer, peers, 10, 1, s)
		cfg.Logger = discardLogger
		rn, err := NewRawNode(cfg)
		if err != nil {
			panic(err)
		}
		n := newNode(rn)
		nodes[peer] = &tNode{
			node: &n,
			s:    s,
		}
		go n.run()
	}
	defer func() {
		for _, n := range nodes {
			n.node.Stop()
		}
	}()

	n1 := nodes[1].node
	n1.Campaign(ctx)
	wg := sync.WaitGroup{}
	for _, n := range nodes {
		wg.Add(1)
		go func(n *tNode) {
			var prevHardSt pb.HardState
			for {
				rd := <-n.node.Ready()
				if isHardStateEqual(prevHardSt, rd.HardState) {
					n.s.SetHardState(rd.HardState)
					prevHardSt = rd.HardState
				}
				n.s.Append(rd.Entries)
				// a reasonable disk sync latency
				// time.Sleep(1 * time.Millisecond)
				n.node.Advance()
				if rd.HardState.Commit == uint64(b.N+1) {
					wg.Done()
				}
				sendAllMessages(rd.Messages, nodes)
			}
		}(n)
	}
	go func() {
		for i := 0; i < b.N; i++ {
			if err := n1.Propose(ctx, []byte("foo")); err != nil {
				panic(err)
			}
		}
	}()
	wg.Wait()
}

func sendAllMessages(msgs []pb.Message, nodes map[uint64]*tNode) {
	for _, m := range msgs {
		n := nodes[m.To]
		n.node.recvc <- m
	}
}
