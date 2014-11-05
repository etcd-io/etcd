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
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	raftPrefix    = "/raft"
	connPerSender = 4
)

type sendHub struct {
	tr      *http.Transport
	cl      ClusterInfo
	ss      *stats.ServerStats
	ls      *stats.LeaderStats
	senders map[types.ID]*sender
}

// newSendHub creates the default send hub used to transport raft messages
// to other members. The returned sendHub will update the given ServerStats and
// LeaderStats appropriately.
func newSendHub(t *http.Transport, cl *Cluster, ss *stats.ServerStats, ls *stats.LeaderStats) *sendHub {
	h := &sendHub{
		tr:      t,
		cl:      cl,
		ss:      ss,
		ls:      ls,
		senders: make(map[types.ID]*sender),
	}
	for _, m := range cl.Members() {
		h.Add(m)
	}
	return h
}

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

		// TODO: don't block. we should be able to have 1000s
		// of messages out at a time.
		data, err := m.Marshal()
		if err != nil {
			log.Println("sender: dropping message:", err)
			return // drop bad message
		}
		if m.Type == raftpb.MsgApp {
			h.ss.SendAppendReq(len(data))
		}

		// TODO (xiangli): reasonable retry logic
		s.send(data)
	}
}

func (h *sendHub) Stop() {
	for _, s := range h.senders {
		s.stop()
	}
}

func (h *sendHub) Add(m *Member) {
	// TODO: considering how to switch between all available peer urls
	u := fmt.Sprintf("%s%s", m.PickPeerURL(), raftPrefix)
	c := &http.Client{Transport: h.tr}
	fs := h.ls.Follower(m.ID.String())
	s := newSender(u, h.cl.ID(), c, fs)
	h.senders[m.ID] = s
}

func (h *sendHub) Remove(id types.ID) {
	h.senders[id].stop()
	delete(h.senders, id)
}

type sender struct {
	u   string
	cid types.ID
	c   *http.Client
	fs  *stats.FollowerStats
	q   chan []byte
}

func newSender(u string, cid types.ID, c *http.Client, fs *stats.FollowerStats) *sender {
	s := &sender{
		u:   u,
		cid: cid,
		c:   c,
		fs:  fs,
		q:   make(chan []byte),
	}
	for i := 0; i < connPerSender; i++ {
		go s.handle()
	}
	return s
}

func (s *sender) send(data []byte) {
	select {
	case s.q <- data:
	default:
		log.Printf("sender: reach the maximal serving to %s", s.u)
	}
}

func (s *sender) stop() {
	close(s.q)
}

func (s *sender) handle() {
	for d := range s.q {
		start := time.Now()
		err := s.post(d)
		end := time.Now()
		if err != nil {
			s.fs.Fail()
			log.Printf("sender: %v", err)
			continue
		}
		s.fs.Succ(end.Sub(start))
	}
}

// post POSTs a data payload to a url. Returns nil if the POST succeeds,
// error on any failure.
func (s *sender) post(data []byte) error {
	req, err := http.NewRequest("POST", s.u, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("new request to %s error: %v", s.u, err)
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Etcd-Cluster-ID", s.cid.String())
	resp, err := s.c.Do(req)
	if err != nil {
		return fmt.Errorf("do request %+v error: %v", req, err)
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		// TODO: shutdown the etcdserver gracefully?
		log.Fatalf("etcd: conflicting cluster ID with the target cluster (%s != %s)", resp.Header.Get("X-Etcd-Cluster-ID"), s.cid)
		return nil
	case http.StatusForbidden:
		// TODO: stop the server
		log.Println("etcd: this member has been permanently removed from the cluster")
		log.Fatalln("etcd: the data-dir used by this member must be removed so that this host can be re-added with a new member ID")
		return nil
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unhandled status %s", http.StatusText(resp.StatusCode))
	}
}
