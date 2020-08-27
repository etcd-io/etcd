// Copyright 2016 The etcd Authors
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

package etcdserver

import (
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/etcdserver/api/membership"
	"go.etcd.io/etcd/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/etcdserver/api/snap"
	"go.etcd.io/etcd/pkg/types"
	"go.etcd.io/etcd/raft/raftpb"
)

func TestLongestConnected(t *testing.T) {
	umap, err := types.NewURLsMap("mem1=http://10.1:2379,mem2=http://10.2:2379,mem3=http://10.3:2379")
	if err != nil {
		t.Fatal(err)
	}
	clus, err := membership.NewClusterFromURLsMap(zap.NewExample(), "test", umap)
	if err != nil {
		t.Fatal(err)
	}
	memberIDs := clus.MemberIDs()

	tr := newNopTransporterWithActiveTime(memberIDs)
	transferee, ok := longestConnected(tr, memberIDs)
	if !ok {
		t.Fatalf("unexpected ok %v", ok)
	}
	if memberIDs[0] != transferee {
		t.Fatalf("expected first member %s to be transferee, got %s", memberIDs[0], transferee)
	}

	// make all members non-active
	amap := make(map[types.ID]time.Time)
	for _, id := range memberIDs {
		amap[id] = time.Time{}
	}
	tr.(*nopTransporterWithActiveTime).reset(amap)

	_, ok2 := longestConnected(tr, memberIDs)
	if ok2 {
		t.Fatalf("unexpected ok %v", ok)
	}
}

type nopTransporterWithActiveTime struct {
	activeMap map[types.ID]time.Time
}

// newNopTransporterWithActiveTime creates nopTransporterWithActiveTime with the first member
// being the most stable (longest active-since time).
func newNopTransporterWithActiveTime(memberIDs []types.ID) rafthttp.Transporter {
	am := make(map[types.ID]time.Time)
	for i, id := range memberIDs {
		am[id] = time.Now().Add(time.Duration(i) * time.Second)
	}
	return &nopTransporterWithActiveTime{activeMap: am}
}

func (s *nopTransporterWithActiveTime) Start() error                        { return nil }
func (s *nopTransporterWithActiveTime) Handler() http.Handler               { return nil }
func (s *nopTransporterWithActiveTime) Send(m []raftpb.Message)             {}
func (s *nopTransporterWithActiveTime) SendSnapshot(m snap.Message)         {}
func (s *nopTransporterWithActiveTime) AddRemote(id types.ID, us []string)  {}
func (s *nopTransporterWithActiveTime) AddPeer(id types.ID, us []string)    {}
func (s *nopTransporterWithActiveTime) RemovePeer(id types.ID)              {}
func (s *nopTransporterWithActiveTime) RemoveAllPeers()                     {}
func (s *nopTransporterWithActiveTime) UpdatePeer(id types.ID, us []string) {}
func (s *nopTransporterWithActiveTime) ActiveSince(id types.ID) time.Time   { return s.activeMap[id] }
func (s *nopTransporterWithActiveTime) ActivePeers() int                    { return 0 }
func (s *nopTransporterWithActiveTime) Stop()                               {}
func (s *nopTransporterWithActiveTime) Pause()                              {}
func (s *nopTransporterWithActiveTime) Resume()                             {}
func (s *nopTransporterWithActiveTime) reset(am map[types.ID]time.Time)     { s.activeMap = am }

func TestPanicAlternativeStringer(t *testing.T) {
	p := panicAlternativeStringer{alternative: func() string { return "alternative" }}

	p.stringer = testStringerFunc(func() string { panic("here") })
	if s := p.String(); s != "alternative" {
		t.Fatalf("expected 'alternative', got %q", s)
	}

	p.stringer = testStringerFunc(func() string { return "test" })
	if s := p.String(); s != "test" {
		t.Fatalf("expected 'test', got %q", s)
	}
}

type testStringerFunc func() string

func (s testStringerFunc) String() string {
	return s()
}
