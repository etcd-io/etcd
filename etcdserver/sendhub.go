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

package etcdserver

import (
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
)

const (
	raftPrefix = "/raft"
)

type SendHub interface {
	rafthttp.SenderFinder
	Send(m []raftpb.Message)
	Add(m *Member)
	Remove(id types.ID)
	Update(m *Member)
	Stop()
	ShouldStopNotify() <-chan struct{}
}

type sendHub struct {
	tr         http.RoundTripper
	cl         ClusterInfo
	p          rafthttp.Processor
	ss         *stats.ServerStats
	ls         *stats.LeaderStats
	senders    map[types.ID]rafthttp.Sender
	shouldstop chan struct{}
}

// newSendHub creates the default send hub used to transport raft messages
// to other members. The returned sendHub will update the given ServerStats and
// LeaderStats appropriately.
func newSendHub(t http.RoundTripper, cl ClusterInfo, p rafthttp.Processor, ss *stats.ServerStats, ls *stats.LeaderStats) *sendHub {
	return &sendHub{
		tr:         t,
		cl:         cl,
		p:          p,
		ss:         ss,
		ls:         ls,
		senders:    make(map[types.ID]rafthttp.Sender),
		shouldstop: make(chan struct{}, 1),
	}
}

func (h *sendHub) Sender(id types.ID) rafthttp.Sender { return h.senders[id] }

func (h *sendHub) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		to := types.ID(m.To)
		s, ok := h.senders[to]
		if !ok {
			if !h.cl.IsIDRemoved(to) {
				log.Printf("etcdserver: send message to unknown receiver %s", to)
			}
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

func (h *sendHub) Add(m *Member) {
	if _, ok := h.senders[m.ID]; ok {
		return
	}
	// TODO: considering how to switch between all available peer urls
	peerURL := m.PickPeerURL()
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, raftPrefix)
	fs := h.ls.Follower(m.ID.String())
	s := rafthttp.NewSender(h.tr, u.String(), h.cl.ID(), h.p, fs, h.shouldstop)
	h.senders[m.ID] = s
}

func (h *sendHub) Remove(id types.ID) {
	h.senders[id].Stop()
	delete(h.senders, id)
}

func (h *sendHub) Update(m *Member) {
	// TODO: return error or just panic?
	if _, ok := h.senders[m.ID]; !ok {
		return
	}
	peerURL := m.PickPeerURL()
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, raftPrefix)
	h.senders[m.ID].Update(u.String())
}

// for testing
func (h *sendHub) pause() {
	for _, s := range h.senders {
		s.Pause()
	}
}

func (h *sendHub) resume() {
	for _, s := range h.senders {
		s.Resume()
	}
}
