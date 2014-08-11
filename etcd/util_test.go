/*
Copyright 2014 CoreOS Inc.

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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/conf"
)

func TestMultipleNodes(t *testing.T) {
	defer afterTest(t)
	tests := []int{1, 3, 5, 9, 11}

	for _, tt := range tests {
		c := &testCluster{Size: tt}
		c.Start()
		c.Destroy()
	}
}

func TestMultipleTLSNodes(t *testing.T) {
	defer afterTest(t)
	tests := []int{1, 3, 5}

	for _, tt := range tests {
		c := &testCluster{Size: tt, TLS: true}
		c.Start()
		c.Destroy()
	}
}

type testCluster struct {
	Size int
	TLS  bool

	nodes []*testServer
}

func (c *testCluster) Start() {
	if c.Size <= 0 {
		panic("cluster size <= 0")
	}

	nodes := make([]*testServer, c.Size)
	c.nodes = nodes
	nodes[0] = &testServer{Id: 0, TLS: c.TLS}
	nodes[0].Start()
	if !nodes[0].WaitMode(participantMode, 1) {
		panic("cannot wait until participantMode")
	}

	seed := nodes[0].URL
	for i := 1; i < c.Size; i++ {
		conf := newTestConfig()
		conf.Peers = []string{seed}
		id := int64(i)
		s := &testServer{Config: conf, Id: id, TLS: c.TLS}
		s.Start()
		nodes[i] = s

		// Wait for the previous configuration change to be committed
		// or this configuration request might be dropped.
		// Or it could be a slow join because it needs to retry.
		// TODO: this might not be true if we add param for retry interval.
		if !s.WaitMode(participantMode, 3) {
			panic("cannot wait until participantMode")
		}
		w, err := s.P().Watch(v2machineKVPrefix, true, false, uint64(i))
		if err != nil {
			panic(err)
		}
		<-w.EventChan
	}
	c.wait()
}

func (c *testCluster) wait() {
	size := c.Size
	for i := 0; i < size; i++ {
		for k := 0; k < size; k++ {
			s := c.At(i)
			w, err := s.P().Watch(v2machineKVPrefix+fmt.Sprintf("/%d", c.At(k).Id), false, false, 1)
			if err != nil {
				panic(err)
			}
			<-w.EventChan
		}
	}

	clusterId := c.P(0).node.ClusterId()
	for i := 0; i < size; i++ {
		if g := c.P(i).node.ClusterId(); g != clusterId {
			panic(fmt.Sprintf("#%d: clusterId = %x, want %x", i, g, clusterId))
		}
	}
}

func (c *testCluster) At(i int) *testServer {
	return c.nodes[i]
}

func (c *testCluster) P(i int) *participant {
	return c.At(i).P()
}

func (c *testCluster) Destroy() {
	for _, s := range c.nodes {
		s.Destroy()
	}
}

type testServer struct {
	Config *conf.Config
	Id     int64
	TLS    bool

	// base URL of form http://ipaddr:port with no trailing slash
	URL string

	e *Server
	h *httptest.Server
}

func (s *testServer) Start() {
	if s.Config == nil {
		s.Config = newTestConfig()
	}
	c := s.Config
	if !strings.HasPrefix(c.DataDir, os.TempDir()) {
		panic("dataDir may pollute file system")
	}
	if c.Peer.CAFile != "" || c.Peer.CertFile != "" || c.Peer.KeyFile != "" {
		panic("use TLS field instead")
	}

	nc := new(conf.Config)
	*nc = *c
	e, err := New(nc)
	if err != nil {
		panic(err)
	}
	s.e = e
	e.setId(s.Id)
	tick := time.Duration(c.Peer.HeartbeatInterval) * time.Millisecond
	e.SetTick(tick)

	m := http.NewServeMux()
	m.Handle("/", e)
	m.Handle("/raft", e.RaftHandler())
	m.Handle("/raft/", e.RaftHandler())
	m.Handle("/v2/admin/", e.RaftHandler())

	addr := c.Addr
	if s.URL != "" {
		addr = urlHost(s.URL)
	}
	s.h = startServingAddr(addr, m, s.TLS)
	s.URL = s.h.URL

	e.pubAddr = s.URL
	e.raftPubAddr = s.URL
	go e.Run()
}

func (s *testServer) WaitMode(mode int64, ms int) bool {
	for i := 0; i < ms+1; i++ {
		if s.e.mode.Get() == mode {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func (s *testServer) P() *participant {
	if s.e.mode.Get() != participantMode {
		panic("cannot get P if not in participant mode")
	}
	return s.e.p
}

func (s *testServer) Stop() error {
	err := s.e.Stop()
	s.h.Close()
	return err
}

func (s *testServer) Destroy() {
	s.Stop()
	if err := os.RemoveAll(s.Config.DataDir); err != nil {
		panic(err)
	}
}

func startServingAddr(addr string, h http.Handler, tls bool) *httptest.Server {
	var l net.Listener
	var err error
	for i := 0; i < 4; i++ {
		l, err = net.Listen("tcp", addr)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "address already in use") {
			panic(err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	if l == nil {
		panic("cannot listen on " + addr)
	}
	hs := &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: h},
	}
	if tls {
		hs.StartTLS()
	} else {
		hs.Start()
	}
	return hs
}

func newTestConfig() *conf.Config {
	c := conf.New()
	c.Addr = "127.0.0.1:0"
	c.Peer.Addr = "127.0.0.1:0"
	c.Peer.HeartbeatInterval = 5
	c.Peer.ElectionTimeout = 25
	dataDir, err := ioutil.TempDir(os.TempDir(), "etcd")
	if err != nil {
		panic(err)
	}
	c.DataDir = dataDir
	return c
}

func urlHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		panic(err)
	}
	return u.Host
}
