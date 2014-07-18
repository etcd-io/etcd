/*
Copyright 2013 CoreOS Inc.

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

package etcd

import (
	"crypto/tls"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/etcd/config"
)

const (
	participantMode int64 = iota
	standbyMode
	stopMode
)

type Server struct {
	config       *config.Config
	id           int64
	pubAddr      string
	raftPubAddr  string
	tickDuration time.Duration

	mode  atomicInt
	nodes map[string]bool
	p     *participant
	s     *standby

	client  *v2client
	peerHub *peerHub

	stopped bool
	mu      sync.Mutex
	stopc   chan struct{}
}

func New(c *config.Config, id int64) *Server {
	if err := c.Sanitize(); err != nil {
		log.Fatalf("failed sanitizing configuration: %v", err)
	}

	tc := &tls.Config{
		InsecureSkipVerify: true,
	}
	var err error
	if c.PeerTLSInfo().Scheme() == "https" {
		tc, err = c.PeerTLSInfo().ClientConfig()
		if err != nil {
			log.Fatal("failed to create raft transporter tls:", err)
		}
	}

	tr := new(http.Transport)
	tr.TLSClientConfig = tc
	client := &http.Client{Transport: tr}

	s := &Server{
		config:       c,
		id:           id,
		pubAddr:      c.Addr,
		raftPubAddr:  c.Peer.Addr,
		tickDuration: defaultTickDuration,

		mode:  atomicInt(stopMode),
		nodes: make(map[string]bool),

		client:  newClient(tc),
		peerHub: newPeerHub(c.Peers, client),

		stopc: make(chan struct{}),
	}
	for _, seed := range c.Peers {
		s.nodes[seed] = true
	}

	return s
}

func (s *Server) SetTick(tick time.Duration) {
	s.tickDuration = tick
}

// Stop stops the server elegently.
func (s *Server) Stop() {
	if s.mode.Get() == stopMode {
		return
	}
	s.mu.Lock()
	s.stopped = true
	switch s.mode.Get() {
	case participantMode:
		s.p.stop()
	case standbyMode:
		s.s.stop()
	}
	s.mu.Unlock()
	<-s.stopc
	s.client.CloseConnections()
	s.peerHub.stop()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch s.mode.Get() {
	case participantMode:
		s.p.ServeHTTP(w, r)
	case standbyMode:
		s.s.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) RaftHandler() http.Handler {
	return http.HandlerFunc(s.ServeRaftHTTP)
}

func (s *Server) ServeRaftHTTP(w http.ResponseWriter, r *http.Request) {
	switch s.mode.Get() {
	case participantMode:
		s.p.raftHandler().ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) Run() {
	next := participantMode
	for {
		s.mu.Lock()
		if s.stopped {
			next = stopMode
		}
		switch next {
		case participantMode:
			s.p = newParticipant(s.id, s.pubAddr, s.raftPubAddr, s.client, s.peerHub, s.tickDuration)
			s.mode.Set(participantMode)
			s.mu.Unlock()
			next = s.p.run()
		case standbyMode:
			s.s = newStandby(s.id, s.pubAddr, s.raftPubAddr, s.nodes, s.client, s.peerHub)
			s.mode.Set(standbyMode)
			s.mu.Unlock()
			next = s.s.run()
		case stopMode:
			s.mode.Set(stopMode)
			s.mu.Unlock()
			s.stopc <- struct{}{}
			return
		default:
			panic("unsupport mode")
		}
	}
}
