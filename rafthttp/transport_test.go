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
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

// TestTransportSend tests that transport can send messages using correct
// underlying peer, and drop local or unknown-target messages.
func TestTransportSend(t *testing.T) {
	ss := &stats.ServerStats{}
	ss.Initialize()
	peer1 := newFakePeer()
	peer2 := newFakePeer()
	tr := &transport{
		serverStats: ss,
		peers:       map[types.ID]Peer{types.ID(1): peer1, types.ID(2): peer2},
	}
	wmsgsIgnored := []raftpb.Message{
		// bad local message
		{Type: raftpb.MsgBeat},
		// bad remote message
		{Type: raftpb.MsgProp, To: 3},
	}
	wmsgsTo1 := []raftpb.Message{
		// good message
		{Type: raftpb.MsgProp, To: 1},
		{Type: raftpb.MsgApp, To: 1},
	}
	wmsgsTo2 := []raftpb.Message{
		// good message
		{Type: raftpb.MsgProp, To: 2},
		{Type: raftpb.MsgApp, To: 2},
	}
	tr.Send(wmsgsIgnored)
	tr.Send(wmsgsTo1)
	tr.Send(wmsgsTo2)

	if !reflect.DeepEqual(peer1.msgs, wmsgsTo1) {
		t.Errorf("msgs to peer 1 = %+v, want %+v", peer1.msgs, wmsgsTo1)
	}
	if !reflect.DeepEqual(peer2.msgs, wmsgsTo2) {
		t.Errorf("msgs to peer 2 = %+v, want %+v", peer2.msgs, wmsgsTo2)
	}
}

func TestTransportAdd(t *testing.T) {
	ls := stats.NewLeaderStats("")
	term := uint64(10)
	tr := &transport{
		roundTripper: &roundTripperRecorder{},
		leaderStats:  ls,
		term:         term,
		peers:        make(map[types.ID]Peer),
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})

	if _, ok := ls.Followers["1"]; !ok {
		t.Errorf("FollowerStats[1] is nil, want exists")
	}
	s, ok := tr.peers[types.ID(1)]
	if !ok {
		tr.Stop()
		t.Fatalf("senders[1] is nil, want exists")
	}

	// duplicate AddPeer is ignored
	tr.AddPeer(1, []string{"http://localhost:2380"})
	ns := tr.peers[types.ID(1)]
	if s != ns {
		t.Errorf("sender = %v, want %v", ns, s)
	}

	tr.Stop()

	if g := s.(*peer).msgAppReader.msgAppTerm; g != term {
		t.Errorf("peer.term = %d, want %d", g, term)
	}
}

func TestTransportRemove(t *testing.T) {
	tr := &transport{
		roundTripper: &roundTripperRecorder{},
		leaderStats:  stats.NewLeaderStats(""),
		peers:        make(map[types.ID]Peer),
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})
	tr.RemovePeer(types.ID(1))
	defer tr.Stop()

	if _, ok := tr.peers[types.ID(1)]; ok {
		t.Fatalf("senders[1] exists, want removed")
	}
}

func TestTransportUpdate(t *testing.T) {
	peer := newFakePeer()
	tr := &transport{
		peers: map[types.ID]Peer{types.ID(1): peer},
	}
	u := "http://localhost:2380"
	tr.UpdatePeer(types.ID(1), []string{u})
	wurls := types.URLs(testutil.MustNewURLs(t, []string{"http://localhost:2380"}))
	if !reflect.DeepEqual(peer.urls, wurls) {
		t.Errorf("urls = %+v, want %+v", peer.urls, wurls)
	}
}

func TestTransportErrorc(t *testing.T) {
	errorc := make(chan error, 1)
	tr := &transport{
		roundTripper: newRespRoundTripper(http.StatusForbidden, nil),
		leaderStats:  stats.NewLeaderStats(""),
		peers:        make(map[types.ID]Peer),
		errorc:       errorc,
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})
	defer tr.Stop()

	select {
	case <-errorc:
		t.Fatalf("received unexpected from errorc")
	case <-time.After(10 * time.Millisecond):
	}
	tr.peers[1].Send(raftpb.Message{})

	testutil.WaitSchedule()
	select {
	case <-errorc:
	default:
		t.Fatalf("cannot receive error from errorc")
	}
}
