/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package raft

import (
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

func BenchmarkOneNode(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := newNode()
	s := NewMemoryStorage()
	r := newRaft(1, []uint64{1}, 10, 1, s)
	go n.run(r)

	defer n.Stop()

	n.Campaign(ctx)
	for i := 0; i < b.N; i++ {
		rd := <-n.Ready()
		s.Append(rd.Entries)
		n.Advance()
		n.Propose(ctx, []byte("foo"))
	}
	rd := <-n.Ready()
	if rd.HardState.Commit != uint64(b.N+1) {
		b.Errorf("commit = %d, want %d", rd.HardState.Commit, b.N+1)
	}
}
