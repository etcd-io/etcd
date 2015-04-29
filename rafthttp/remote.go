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

	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

type remote struct {
	id   types.ID
	peer *peer
}

func startRemote(tr http.RoundTripper, u string, local, to, cid types.ID, r Raft, errorc chan error) *remote {
	return &remote{
		id:   to,
		peer: NewPeer(tr, u, to, cid, r, nil, errorc),
	}
}

func (g *remote) Send(m raftpb.Message) {
	g.peer.send(m)
}

func (g *remote) Stop() {
	g.peer.Stop()
}
