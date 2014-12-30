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

package rafthttp

import (
	"net/http"
	"testing"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

func TestTransportAdd(t *testing.T) {
	ls := stats.NewLeaderStats("")
	tr := &Transport{
		LeaderStats: ls,
	}
	tr.Start()
	tr.AddPeer(1, []string{"http://a"})

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
	tr := &Transport{
		LeaderStats: stats.NewLeaderStats(""),
	}
	tr.Start()
	tr.AddPeer(1, []string{"http://a"})
	tr.RemovePeer(types.ID(1))

	if _, ok := tr.peers[types.ID(1)]; ok {
		t.Fatalf("senders[1] exists, want removed")
	}
}

func TestTransportShouldStop(t *testing.T) {
	tr := &Transport{
		RoundTripper: newRespRoundTripper(http.StatusForbidden, nil),
		LeaderStats:  stats.NewLeaderStats(""),
	}
	tr.Start()
	tr.AddPeer(1, []string{"http://a"})

	shouldstop := tr.ShouldStopNotify()
	select {
	case <-shouldstop:
		t.Fatalf("received unexpected shouldstop notification")
	case <-time.After(10 * time.Millisecond):
	}
	tr.peers[1].Send(raftpb.Message{})

	testutil.ForceGosched()
	select {
	case <-shouldstop:
	default:
		t.Fatalf("cannot receive stop notification")
	}
}
