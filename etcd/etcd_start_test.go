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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	bootstrapId = 0xBEEF
)

type garbageHandler struct {
	t       *testing.T
	success bool
	sync.Mutex
}

func (g *garbageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, client")
	wp := fmt.Sprint("/v2/keys/_etcd/registry/1/", bootstrapId)
	if gp := r.URL.String(); gp != wp {
		g.t.Fatalf("url = %s, want %s", gp, wp)
	}
	g.Lock()
	defer g.Unlock()

	g.success = true
}

func TestBadDiscoveryService(t *testing.T) {
	defer afterTest(t)
	g := garbageHandler{t: t}
	httpts := httptest.NewServer(&g)
	defer httpts.Close()

	c := newTestConfig()
	c.Discovery = httpts.URL + "/v2/keys/_etcd/registry/1"
	ts := testServer{Config: c, Id: bootstrapId}
	ts.Start()

	err := ts.Destroy()
	w := `discovery service error`
	if err == nil || !strings.HasPrefix(err.Error(), w) {
		t.Errorf("err = %v, want %s prefix", err, w)
	}

	g.Lock()
	defer g.Unlock()
	if !g.success {
		t.Fatal("Discovery server never called")
	}
}

func TestBadDiscoveryServiceWithAdvisedPeers(t *testing.T) {
	defer afterTest(t)
	g := garbageHandler{t: t}
	httpts := httptest.NewServer(&g)
	defer httpts.Close()

	c := newTestConfig()
	c.Discovery = httpts.URL + "/v2/keys/_etcd/registry/1"
	c.Peers = []string{"a peer"}
	ts := testServer{Config: c, Id: bootstrapId}
	ts.Start()

	err := ts.Destroy()
	w := `discovery service error`
	if err == nil || !strings.HasPrefix(err.Error(), w) {
		t.Errorf("err = %v, want %s prefix", err, w)
	}
}

func TestBootstrapByEmptyPeers(t *testing.T) {
	defer afterTest(t)
	id := genId()
	ts := testServer{Id: id}
	ts.Start()
	defer ts.Destroy()
	ts.WaitMode(participantMode, 3)
	if ts.Participant().node.Leader() != id {
		t.Errorf("leader = %x, want %x", ts.Participant().node.Leader(), id)
	}
}

func TestBootstrapByDiscoveryService(t *testing.T) {
	defer afterTest(t)
	discoverService := testCluster{Size: 1}
	discoverService.Start()
	defer discoverService.Destroy()

	c := newTestConfig()
	c.Discovery = discoverService.URL(0) + "/v2/keys/_etcd/registry/1"
	ts := testServer{Id: bootstrapId, Config: c}
	ts.Start()
	ts.WaitMode(participantMode, 3)
	err := ts.Destroy()
	if err != nil {
		t.Fatalf("server stop err = %v, want nil", err)
	}
}

func TestRunByAdvisedPeers(t *testing.T) {
	t.Skip("test covered by TestMultipleNodes")
}

func TestRunByDiscoveryService(t *testing.T) {
	ds := testCluster{Size: 1}
	ds.Start()
	defer ds.Destroy()

	tc := NewTestClient()
	v := url.Values{}
	v.Set("value", "started")
	resp, _ := tc.PutForm(fmt.Sprintf("%s%s", ds.URL(0), "/v2/keys/_etcd/registry/1/_state"), v)
	if g := resp.StatusCode; g != http.StatusCreated {
		t.Fatalf("put status = %d, want %d", g, http.StatusCreated)
	}
	resp.Body.Close()

	v.Set("value", ds.URL(0))
	resp, _ = tc.PutForm(fmt.Sprintf("%s%s%d", ds.URL(0), "/v2/keys/_etcd/registry/1/", ds.Participant(0).id), v)
	if g := resp.StatusCode; g != http.StatusCreated {
		t.Fatalf("put status = %d, want %d", g, http.StatusCreated)
	}
	resp.Body.Close()

	c := newTestConfig()
	c.Discovery = ds.URL(0) + "/v2/keys/_etcd/registry/1"
	ts := testServer{Config: c, Id: bootstrapId}
	ts.Start()
	defer ts.Destroy()

	ts.WaitMode(participantMode, 3)
	// wait for the leader to do a heartbeat
	// it will update the lead field of the follower
	time.Sleep(ds.Node(0).e.tickDuration * defaultHeartbeat * 2)
	w := ds.Participant(0).id
	if g := ts.Lead().lead; g != w {
		t.Errorf("leader = %d, want %d", g, w)
	}
}

func TestRunByDataDir(t *testing.T) {
	t.Skip("test covered by TestSingleNodeRecovery")
}
