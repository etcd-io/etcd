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

package rafthttp

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

func BenchmarkStandaloneTransport(b *testing.B) {
	p := &countProcessor{}
	ss := &stats.ServerStats{}
	ss.Initialize()
	tr := NewTransporter(&http.Transport{}, types.ID(1), types.ID(1), p, nil, ss, stats.NewLeaderStats("1"))
	srv := httptest.NewServer(tr.Handler())
	defer srv.Close()
	tr.AddPeer(types.ID(1), []string{srv.URL})
	defer tr.Stop()
	time.Sleep(time.Second)

	data := make([]byte, 64)
	b.ReportAllocs()
	b.SetBytes(64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Send([]raftpb.Message{{Type: raftpb.MsgApp, To: 1, Entries: []raftpb.Entry{{Data: data}}}})
	}
	for p.count() != b.N {
		time.Sleep(time.Millisecond)
	}
	b.StopTimer()
}

type countProcessor struct {
	mu  sync.Mutex
	cnt int
}

func (p *countProcessor) Process(ctx context.Context, m raftpb.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cnt++
	return nil
}

func (p *countProcessor) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cnt
}
