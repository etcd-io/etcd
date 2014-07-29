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

	"github.com/coreos/etcd/config"
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
	g := garbageHandler{t: t}
	ts := httptest.NewServer(&g)

	c := config.New()
	c.Discovery = ts.URL + "/v2/keys/_etcd/registry/1"
	_, _, err := buildServer(t, c, bootstrapId)
	w := `discovery service error`
	if err == nil || !strings.HasPrefix(err.Error(), w) {
		t.Errorf("err = %v, want %s prefix", err, w)
	}

	g.Lock()
	defer g.Unlock()
	if !g.success {
		t.Fatal("Discovery server never called")
	}
	ts.Close()
	afterTest(t)
}

func TestBadDiscoveryServiceWithAdvisedPeers(t *testing.T) {
	g := garbageHandler{t: t}
	ts := httptest.NewServer(&g)

	es, hs := buildCluster(1, false)
	waitCluster(t, es)

	c := config.New()
	c.Discovery = ts.URL + "/v2/keys/_etcd/registry/1"
	c.Peers = []string{hs[0].URL}
	_, _, err := buildServer(t, c, bootstrapId)
	w := `discovery service error`
	if err == nil || !strings.HasPrefix(err.Error(), w) {
		t.Errorf("err = %v, want %s prefix", err, w)
	}

	destoryCluster(t, es, hs)
	ts.Close()
	afterTest(t)
}

func TestBootstrapByEmptyPeers(t *testing.T) {
	c := config.New()
	id := genId()
	e, h, err := buildServer(t, c, id)

	if err != nil {
		t.Error(err)
	}
	if e.p.node.Leader() != id {
		t.Error("leader = %x, want %x", e.p.node.Leader(), id)
	}
	destroyServer(t, e, h)
	afterTest(t)
}

func TestBootstrapByDiscoveryService(t *testing.T) {
	de, dh, _ := buildServer(t, config.New(), genId())

	c := config.New()
	c.Discovery = dh.URL + "/v2/keys/_etcd/registry/1"
	e, h, err := buildServer(t, c, bootstrapId)
	if err != nil {
		t.Fatalf("build server err = %v, want nil", err)
	}

	destroyServer(t, e, h)
	destroyServer(t, de, dh)
	afterTest(t)
}

func TestRunByAdvisedPeers(t *testing.T) {
	es, hs := buildCluster(1, false)
	waitCluster(t, es)

	c := config.New()
	c.Peers = []string{hs[0].URL}
	e, h, err := buildServer(t, c, bootstrapId)
	if err != nil {
		t.Fatalf("build server err = %v, want nil", err)
	}
	w := es[0].id
	if g, _ := waitLeader(append(es, e)); g != w {
		t.Errorf("leader = %d, want %d", g, w)
	}

	destroyServer(t, e, h)
	destoryCluster(t, es, hs)
	afterTest(t)
}

func TestRunByDiscoveryService(t *testing.T) {
	de, dh, _ := buildServer(t, config.New(), genId())

	tc := NewTestClient()
	v := url.Values{}
	v.Set("value", "started")
	resp, _ := tc.PutForm(fmt.Sprintf("%s%s", dh.URL, "/v2/keys/_etcd/registry/1/_state"), v)
	if g := resp.StatusCode; g != http.StatusCreated {
		t.Fatalf("put status = %d, want %d", g, http.StatusCreated)
	}
	resp.Body.Close()

	v.Set("value", dh.URL)
	resp, _ = tc.PutForm(fmt.Sprintf("%s%s%d", dh.URL, "/v2/keys/_etcd/registry/1/", de.id), v)
	if g := resp.StatusCode; g != http.StatusCreated {
		t.Fatalf("put status = %d, want %d", g, http.StatusCreated)
	}
	resp.Body.Close()

	c := config.New()
	c.Discovery = dh.URL + "/v2/keys/_etcd/registry/1"
	e, h, err := buildServer(t, c, bootstrapId)
	if err != nil {
		t.Fatalf("build server err = %v, want nil", err)
	}
	w := de.id
	if g, _ := waitLeader([]*Server{e, de}); g != w {
		t.Errorf("leader = %d, want %d", g, w)
	}

	destroyServer(t, e, h)
	destroyServer(t, de, dh)
	afterTest(t)
}

func TestRunByDataDir(t *testing.T) {
	TestSingleNodeRecovery(t)
}

func buildServer(t *testing.T, c *config.Config, id int64) (e *Server, h *httptest.Server, err error) {
	e, h = initTestServer(c, id, false)
	go func() { err = e.Run() }()
	for {
		if e.mode.Get() == participantMode {
			break
		}
		if err != nil {
			destroyServer(t, e, h)
			return nil, nil, err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return e, h, nil
}
