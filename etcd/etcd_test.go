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
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/conf"
	"github.com/coreos/etcd/store"
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

func TestV2Redirect(t *testing.T) {
	defer afterTest(t)

	c := &testCluster{Size: 3}
	c.Start()
	defer c.Destroy()

	u := c.URL(1)
	ru := fmt.Sprintf("%s%s", c.URL(0), "/v2/keys/foo")
	tc := NewTestClient()

	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := tc.PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTemporaryRedirect)
	}
	location, err := resp.Location()
	if err != nil {
		t.Errorf("want err = %, want nil", err)
	}

	if location.String() != ru {
		t.Errorf("location = %v, want %v", location.String(), ru)
	}
	resp.Body.Close()
}

func TestRemove(t *testing.T) {
	defer afterTest(t)
	tests := []int{3, 4, 5, 6}
	for aa := 0; aa < 1; aa++ {
		for k, tt := range tests {
			cl := testCluster{Size: tt}
			cl.Start()

			lead, _ := cl.Leader()
			config := conf.NewClusterConfig()
			config.ActiveSize = 0
			if err := cl.Participant(lead).setClusterConfig(config); err != nil {
				t.Fatalf("#%d: setClusterConfig err = %v", k, err)
			}

			// we don't remove the machine from 2-node cluster because it is
			// not 100 percent safe in our raft.
			// TODO(yichengq): improve it later.
			for i := 0; i < tt-2; i++ {
				id := cl.Id(i)
				for {
					n := cl.Node(i)
					if n.e.mode.Get() == standbyMode {
						break
					}
					err := n.Participant().remove(id)
					if err == nil {
						break
					}
					switch err {
					case tmpErr:
						time.Sleep(defaultElection * 5 * time.Millisecond)
					case raftStopErr, stopErr:
					default:
						t.Fatal(err)
					}
				}
				cl.Node(i).WaitMode(standbyMode)
			}
			cl.Destroy()
		}
	}
}

// TODO(yicheng) Add test for becoming standby
// maxSize -> standby
// auto-demote -> standby
// remove -> standby

func TestReleaseVersion(t *testing.T) {
	defer afterTest(t)

	cl := testCluster{Size: 1}
	cl.Start()
	defer cl.Destroy()

	resp, err := http.Get(cl.URL(0) + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	g, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	gs := string(g)
	w := fmt.Sprintf("etcd %s", releaseVersion)
	if gs != w {
		t.Errorf("version = %v, want %v", gs, w)
	}
}

func TestVersionCheck(t *testing.T) {
	defer afterTest(t)

	cl := testCluster{Size: 1}
	cl.Start()
	defer cl.Destroy()

	u := cl.URL(0)

	currentVersion := 2
	tests := []struct {
		version int
		wStatus int
	}{
		{currentVersion - 1, http.StatusForbidden},
		{currentVersion, http.StatusOK},
		{currentVersion + 1, http.StatusForbidden},
	}

	for i, tt := range tests {
		resp, err := http.Get(fmt.Sprintf("%s/raft/version/%d/check", u, tt.version))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tt.wStatus {
			t.Fatal("#%d: status = %d, want %d", i, resp.StatusCode, tt.wStatus)
		}
	}
}

func TestSingleNodeRecovery(t *testing.T) {
	defer afterTest(t)
	c := newTestConfig()
	ts := testServer{Config: c}
	ts.Start()
	defer ts.Destroy()

	ts.WaitMode(participantMode)

	key := "/foo"
	ev, err := ts.Participant().Set(key, false, "bar", time.Now().Add(time.Second*100))
	if err != nil {
		t.Fatal(err)
	}
	ts.Stop()

	ts = testServer{Config: c}
	ts.Start()
	ts.WaitMode(participantMode)
	w, err := ts.Participant().Store.Watch(key, false, false, ev.Index())
	if err != nil {
		t.Fatal(err)
	}
	// give testing server time to load the previous WAL file
	select {
	case <-w.EventChan:
	case <-time.After(time.Second):
		t.Fatal("watch timeout")
	}
}

func TestTakingSnapshot(t *testing.T) {
	defer afterTest(t)

	cl := testCluster{Size: 1}
	cl.Start()
	defer cl.Destroy()

	// TODO(xiangli): tunable compact; reduce testing time
	for i := 0; i < defaultCompact; i++ {
		cl.Participant(0).Set("/foo", false, "bar", store.Permanent)
	}
	snap := cl.Participant(0).node.GetSnap()
	if snap.Index != defaultCompact {
		t.Errorf("snap.Index = %d, want %d", snap.Index, defaultCompact)
	}
}

func TestRestoreSnapshotFromLeader(t *testing.T) {
	defer afterTest(t)

	cl := testCluster{Size: 1}
	cl.Start()
	defer cl.Destroy()

	// let leader do snapshot
	for i := 0; i < defaultCompact; i++ {
		cl.Participant(0).Set(fmt.Sprint("/foo", i), false, fmt.Sprint("bar", i), store.Permanent)
	}

	// create one to join the cluster
	c := newTestConfig()
	c.Name = "1"
	c.Peers = []string{cl.URL(0)}
	ts := &testServer{Config: c}
	cl.Add(ts)

	// check new proposal could be submitted
	if _, err := cl.Participant(0).Set("/foo", false, "bar", store.Permanent); err != nil {
		t.Fatal(err)
	}

	// check store is recovered
	for i := 0; i < defaultCompact; i++ {
		ev, err := ts.Participant().Store.Get(fmt.Sprint("/foo", i), false, false)
		if err != nil {
			t.Errorf("get err = %v", err)
			continue
		}
		w := fmt.Sprint("bar", i)
		if g := *ev.Node.Value; g != w {
			t.Errorf("value = %v, want %v", g, w)
		}
	}

	// check new proposal could be committed in the new machine
	wch, err := ts.Participant().Watch("/foo", false, false, defaultCompact)
	if err != nil {
		t.Errorf("watch err = %v", err)
	}
	<-wch.EventChan

	// check node map of two machines are the same
	g := ts.Participant().node.Nodes()
	w := cl.Participant(0).node.Nodes()
	if !reflect.DeepEqual(g, w) {
		t.Errorf("nodes = %v, want %v", g, w)
	}
}

func TestSaveSnapshot(t *testing.T) {
	defer afterTest(t)

	cl := testCluster{Size: 1}
	cl.Start()
	defer cl.Destroy()

	n := cl.Node(0)
	// TODO(xiangli): tunable compact; reduce testing time
	for i := 0; i < defaultCompact; i++ {
		n.Participant().Set("/foo", false, "bar", store.Permanent)
	}
	snapname := fmt.Sprintf("%016x-%016x-%016x.snap", n.Participant().clusterId, 1, defaultCompact)
	snappath := path.Join(n.Config.DataDir, "snap", snapname)
	if _, err := os.Stat(snappath); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	walname := fmt.Sprintf("%016x-%016x.wal", 1, defaultCompact)
	walpath := path.Join(n.Config.DataDir, "wal", walname)
	if _, err := os.Stat(walpath); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestRestoreSnapshotFromDisk(t *testing.T) {
	defer afterTest(t)

	cl := testCluster{Size: 1}
	cl.Start()
	defer cl.Destroy()

	lead, _ := cl.Leader()
	for i := 0; i < defaultCompact+10; i++ {
		cl.Participant(lead).Set(fmt.Sprint("/foo", i), false, fmt.Sprint("bar", i), store.Permanent)
	}

	cl.Stop()
	cl.Restart()

	lead, _ = cl.Leader()
	// check store is recovered
	for i := 0; i < defaultCompact+10; i++ {
		ev, err := cl.Participant(lead).Store.Get(fmt.Sprint("/foo", i), false, false)
		if err != nil {
			t.Errorf("get err = %v", err)
			continue
		}
		w := fmt.Sprint("bar", i)
		if g := *ev.Node.Value; g != w {
			t.Errorf("value = %v, want %v", g, w)
		}
	}
	// check new proposal could be submitted
	if _, err := cl.Participant(lead).Set("/foo", false, "bar", store.Permanent); err != nil {
		t.Fatal(err)
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
	cfg := newTestConfig()
	cfg.Name = "testServer-0"
	nodes[0] = &testServer{Config: cfg, TLS: c.TLS}
	nodes[0].Start()
	nodes[0].WaitMode(participantMode)

	seed := nodes[0].URL
	for i := 1; i < c.Size; i++ {
		cfg := newTestConfig()
		cfg.Name = "testServer-" + fmt.Sprint(i)
		cfg.Peers = []string{seed}
		s := &testServer{Config: cfg, TLS: c.TLS}
		s.Start()
		nodes[i] = s

		// Wait for the previous configuration change to be committed
		// or this configuration request might be dropped.
		// Or it could be a slow join because it needs to retry.
		// TODO: this might not be true if we add param for retry interval.
		s.WaitMode(participantMode)
		w, err := s.Participant().Watch(v2machineKVPrefix, true, false, uint64(i))
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
			s := c.Node(i)
			wp := v2machineKVPrefix + fmt.Sprintf("/%d", c.Id(k))
			w, err := s.Participant().Watch(wp, false, false, 1)
			if err != nil {
				panic(err)
			}
			<-w.EventChan
		}
	}

	clusterId := c.Participant(0).node.ClusterId()
	for i := 0; i < size; i++ {
		if g := c.Participant(i).node.ClusterId(); g != clusterId {
			panic(fmt.Sprintf("#%d: clusterId = %x, want %x", i, g, clusterId))
		}
	}
}

func (c *testCluster) Add(s *testServer) {
	lead, _ := c.Leader()
	// wait for the node to join the cluster
	// TODO(yichengq): remove this when we get rid of all timeouts
	wch, err := c.Participant(int(lead)).Watch(v2machineKVPrefix, true, false, 0)
	if err != nil {
		panic(err)
	}
	s.Start()
	<-wch.EventChan
	c.Size++
	c.nodes = append(c.nodes, s)
}

func (c *testCluster) Node(i int) *testServer {
	return c.nodes[i]
}

func (c *testCluster) Participant(i int) *participant {
	return c.Node(i).Participant()
}

func (c *testCluster) Standby(i int) *standby {
	return c.Node(i).Standby()
}

func (c *testCluster) URL(i int) string {
	return c.nodes[i].h.URL
}

func (c *testCluster) Id(i int) int64 {
	return c.Participant(i).id
}

func (c *testCluster) Restart() {
	for _, s := range c.nodes {
		s.Start()
	}
}

func (c *testCluster) Stop() {
	for _, s := range c.nodes {
		s.Stop()
	}
}

func (c *testCluster) Destroy() {
	for _, s := range c.nodes {
		s.Destroy()
	}
}

// Leader returns the index of leader in the cluster and its leader term.
func (c *testCluster) Leader() (leadIdx int, term int64) {
	ids := make(map[int64]int)
	for {
		ls := make([]leadterm, 0, c.Size)
		for i := range c.nodes {
			switch c.Node(i).e.mode.Get() {
			case participantMode:
				ls = append(ls, c.Node(i).Lead())
				ids[c.Id(i)] = i
			case standbyMode:
				//TODO(xiangli) add standby support
			case stopMode:
			}
		}
		if isSameLead(ls) {
			return ids[ls[0].lead], ls[0].term
		}
		time.Sleep(c.Node(0).e.tickDuration * defaultElection)
	}
}

type leadterm struct {
	lead int64
	term int64
}

func isSameLead(ls []leadterm) bool {
	m := make(map[leadterm]int)
	for i := range ls {
		m[ls[i]] = m[ls[i]] + 1
	}
	if len(m) == 1 {
		if ls[0].lead == -1 {
			return false
		}
		return true
	}
	// todo(xiangli): printout the current cluster status for debugging....
	return false
}

type testServer struct {
	Config *conf.Config
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

	nc := *c
	e, err := New(&nc)
	if err != nil {
		panic(err)
	}
	s.e = e
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
	e.cfg.Addr = s.URL
	e.cfg.Peer.Addr = s.URL
	go e.Run()
}

func (s *testServer) WaitMode(mode int64) {
	for i := 0; i < 30; i++ {
		if s.e.mode.Get() == mode {
			return
		}
		time.Sleep(time.Millisecond)
	}
	panic("waitMode should never take more than 30ms.")
}

func (s *testServer) Participant() *participant {
	if s.e.mode.Get() != participantMode {
		return nil
	}
	return s.e.p
}

func (s *testServer) Standby() *standby {
	return s.e.s
}

func (s *testServer) Lead() leadterm {
	return leadterm{s.Participant().node.Leader(), s.Participant().node.Term()}
}

func (s *testServer) Stop() error {
	err := s.e.Stop()
	s.h.Close()
	return err
}

func (s *testServer) Destroy() error {
	err := s.Stop()
	if err := os.RemoveAll(s.Config.DataDir); err != nil {
		panic(err)
	}
	return err
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
	c.RetryInterval = 1 / 10.0
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
