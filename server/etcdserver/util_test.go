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

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/raft/v3/raftpb"
)

func TestLongestConnected(t *testing.T) {
	umap, err := types.NewURLsMap("mem1=http://10.1:2379,mem2=http://10.2:2379,mem3=http://10.3:2379")
	require.NoError(t, err)
	clus, err := membership.NewClusterFromURLsMap(zaptest.NewLogger(t), "test", umap)
	require.NoError(t, err)
	memberIDs := clus.MemberIDs()

	tr := newNopTransporterWithActiveTime(memberIDs)
	transferee, ok := longestConnected(tr, memberIDs)
	require.Truef(t, ok, "unexpected ok %v", ok)
	require.Equalf(t, memberIDs[0], transferee, "expected first member %s to be transferee, got %s", memberIDs[0], transferee)

	// make all members non-active
	amap := make(map[types.ID]time.Time)
	for _, id := range memberIDs {
		amap[id] = time.Time{}
	}
	tr.(*nopTransporterWithActiveTime).reset(amap)

	_, ok2 := longestConnected(tr, memberIDs)
	require.Falsef(t, ok2, "unexpected ok %v", ok)
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
	s := p.String()
	require.Equalf(t, "alternative", s, "expected 'alternative', got %q", s)

	p.stringer = testStringerFunc(func() string { return "test" })
	s = p.String()
	require.Equalf(t, "test", s, "expected 'test', got %q", s)
}

type testStringerFunc func() string

func (s testStringerFunc) String() string {
	return s()
}
