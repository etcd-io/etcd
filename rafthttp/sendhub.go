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
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	raftPrefix = "/raft"
)

type sendHub struct {
	tr         http.RoundTripper
	cid        types.ID
	p          Processor
	ss         *stats.ServerStats
	ls         *stats.LeaderStats
	mu         sync.RWMutex // protect the sender map
	senders    map[types.ID]Sender
	shouldstop chan struct{}
}

// newSendHub creates the default send hub used to transport raft messages
// to other members. The returned sendHub will update the given ServerStats and
// LeaderStats appropriately.
func newSendHub(t http.RoundTripper, cid types.ID, p Processor, ss *stats.ServerStats, ls *stats.LeaderStats) *sendHub {
	return &sendHub{
		tr:         t,
		cid:        cid,
		p:          p,
		ss:         ss,
		ls:         ls,
		senders:    make(map[types.ID]Sender),
		shouldstop: make(chan struct{}, 1),
	}
}

func (h *sendHub) Sender(id types.ID) Sender {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.senders[id]
}

func (h *sendHub) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		// intentionally dropped message
		if m.To == 0 {
			continue
		}
		to := types.ID(m.To)
		s, ok := h.senders[to]
		if !ok {
			log.Printf("etcdserver: send message to unknown receiver %s", to)
			continue
		}

		if m.Type == raftpb.MsgApp {
			h.ss.SendAppendReq(m.Size())
		}

		s.Send(m)
	}
}

func (h *sendHub) Stop() {
	for _, s := range h.senders {
		s.Stop()
	}
	if tr, ok := h.tr.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func (h *sendHub) ShouldStopNotify() <-chan struct{} {
	return h.shouldstop
}

func (h *sendHub) AddPeer(id types.ID, urls []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.senders[id]; ok {
		return
	}
	// TODO: considering how to switch between all available peer urls
	peerURL := urls[0]
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, raftPrefix)
	fs := h.ls.Follower(id.String())
	s := NewSender(h.tr, u.String(), id, h.cid, h.p, fs, h.shouldstop)
	h.senders[id] = s
}

func (h *sendHub) RemovePeer(id types.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.senders[id].Stop()
	delete(h.senders, id)
}

func (h *sendHub) UpdatePeer(id types.ID, urls []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// TODO: return error or just panic?
	if _, ok := h.senders[id]; !ok {
		return
	}
	peerURL := urls[0]
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, raftPrefix)
	h.senders[id].Update(u.String())
}

// for testing
func (h *sendHub) Pause() {
	for _, s := range h.senders {
		s.Pause()
	}
}

func (h *sendHub) Resume() {
	for _, s := range h.senders {
		s.Resume()
	}
}
