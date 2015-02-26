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

func TestTransportSend(t *testing.T) {
	ss := &stats.ServerStats{}
	ss.Initialize()
	peer := newPeerRecorder()
	tr := &transport{
		serverStats: ss,
		peers:       map[types.ID]Peer{types.ID(1): peer},
	}
	msgs := []raftpb.Message{
		// bad local message
		{Type: raftpb.MsgBeat},
		// bad remote message
		{Type: raftpb.MsgProp, To: 2},
		// good message
		{Type: raftpb.MsgProp, To: 1},
		{Type: raftpb.MsgApp, To: 1},
	}
	tr.Send(msgs)

	wmsgs := msgs[2:]
	if !reflect.DeepEqual(peer.msgs, wmsgs) {
		t.Errorf("msgs = %+v, want %+v", peer.msgs, wmsgs)
	}
	if ss.SendAppendRequestCnt != 1 {
		t.Errorf("msgapp count = %d, want 1", ss.SendAppendRequestCnt)
	}
}

func TestTransportAdd(t *testing.T) {
	ls := stats.NewLeaderStats("")
	tr := &transport{
		roundTripper: &roundTripperRecorder{},
		leaderStats:  ls,
		peers:        make(map[types.ID]Peer),
	}
	tr.AddPeer(1, []string{"http://a"})
	defer tr.Stop()

	if _, ok := ls.Followers["1"]; !ok {
		t.Errorf("FollowerStats[1] is nil, want exists")
	}
	s, ok := tr.peers[types.ID(1)]
	if !ok {
		t.Fatalf("senders[1] is nil, want exists")
	}

	// duplicate AddPeer is ignored
	tr.AddPeer(1, []string{"http://a"})
	ns := tr.peers[types.ID(1)]
	if s != ns {
		t.Errorf("sender = %v, want %v", ns, s)
	}
}

func TestTransportRemove(t *testing.T) {
	tr := &transport{
		roundTripper: &roundTripperRecorder{},
		leaderStats:  stats.NewLeaderStats(""),
		peers:        make(map[types.ID]Peer),
	}
	tr.AddPeer(1, []string{"http://a"})
	tr.RemovePeer(types.ID(1))
	defer tr.Stop()

	if _, ok := tr.peers[types.ID(1)]; ok {
		t.Fatalf("senders[1] exists, want removed")
	}
}

func TestTransportUpdate(t *testing.T) {
	peer := newPeerRecorder()
	tr := &transport{
		peers: map[types.ID]Peer{types.ID(1): peer},
	}
	u := "http://localhost:7001"
	tr.UpdatePeer(types.ID(1), []string{u})
	if w := "http://localhost:7001/raft"; peer.u != w {
		t.Errorf("url = %s, want %s", peer.u, w)
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
	tr.AddPeer(1, []string{"http://a"})
	defer tr.Stop()

	select {
	case <-errorc:
		t.Fatalf("received unexpected from errorc")
	case <-time.After(10 * time.Millisecond):
	}
	tr.peers[1].Send(raftpb.Message{})

	testutil.ForceGosched()
	select {
	case <-errorc:
	default:
		t.Fatalf("cannot receive error from errorc")
	}
}
