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
	"testing"
	"time"
)

func BenchmarkOneNode(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newTestMemoryStorage(withPeers(1))
	rn := newTestRawNode(1, 10, 1, s)
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
