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
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

type Raft interface {
	Process(ctx context.Context, m raftpb.Message) error
}

type Transporter interface {
	Handler() http.Handler
	Send(m []raftpb.Message)
	AddPeer(id types.ID, urls []string)
	RemovePeer(id types.ID)
	RemoveAllPeers()
	UpdatePeer(id types.ID, urls []string)
	Stop()
}

type transport struct {
	roundTripper http.RoundTripper
	id           types.ID
	clusterID    types.ID
	raft         Raft
	serverStats  *stats.ServerStats
	leaderStats  *stats.LeaderStats

	mu     sync.RWMutex       // protect the peer map
	peers  map[types.ID]*peer // remote peers
	errorc chan error
}

func NewTransporter(rt http.RoundTripper, id, cid types.ID, r Raft, errorc chan error, ss *stats.ServerStats, ls *stats.LeaderStats) Transporter {
	return &transport{
		roundTripper: rt,
		id:           id,
		clusterID:    cid,
		raft:         r,
		serverStats:  ss,
		leaderStats:  ls,
		peers:        make(map[types.ID]*peer),
		errorc:       errorc,
	}
}

func (t *transport) Handler() http.Handler {
	h := NewHandler(t.raft, t.clusterID)
	sh := NewStreamHandler(t, t.id, t.clusterID)
	mux := http.NewServeMux()
	mux.Handle(RaftPrefix, h)
	mux.Handle(RaftStreamPrefix+"/", sh)
	return mux
}

func (t *transport) Peer(id types.ID) *peer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.peers[id]
}

func (t *transport) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		// intentionally dropped message
		if m.To == 0 {
			continue
		}
		to := types.ID(m.To)
		p, ok := t.peers[to]
		if !ok {
			log.Printf("etcdserver: send message to unknown receiver %s", to)
			continue
		}

		if m.Type == raftpb.MsgApp {
			t.serverStats.SendAppendReq(m.Size())
		}

		p.Send(m)
	}
}

func (t *transport) Stop() {
	for _, p := range t.peers {
		p.Stop()
	}
	if tr, ok := t.roundTripper.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func (t *transport) AddPeer(id types.ID, urls []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.peers[id]; ok {
		return
	}
	// TODO: considering how to switch between all available peer urls
	peerURL := urls[0]
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, RaftPrefix)
	fs := t.leaderStats.Follower(id.String())
	t.peers[id] = NewPeer(t.roundTripper, u.String(), id, t.clusterID, t.raft, fs, t.errorc)
}

func (t *transport) RemovePeer(id types.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removePeer(id)
}

func (t *transport) RemoveAllPeers() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, _ := range t.peers {
		t.removePeer(id)
	}
}

// the caller of this function must have the peers mutex.
func (t *transport) removePeer(id types.ID) {
	if peer, ok := t.peers[id]; ok {
		peer.Stop()
	} else {
		log.Panicf("rafthttp: unexpected removal of unknown peer '%d'", id)
	}
	delete(t.peers, id)
	delete(t.leaderStats.Followers, id.String())
}

func (t *transport) UpdatePeer(id types.ID, urls []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// TODO: return error or just panic?
	if _, ok := t.peers[id]; !ok {
		return
	}
	peerURL := urls[0]
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, RaftPrefix)
	t.peers[id].Update(u.String())
}

type Pausable interface {
	Pause()
	Resume()
}

// for testing
func (t *transport) Pause() {
	for _, p := range t.peers {
		p.Pause()
	}
}

func (t *transport) Resume() {
	for _, p := range t.peers {
		p.Resume()
	}
}
