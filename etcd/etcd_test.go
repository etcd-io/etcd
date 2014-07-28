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
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/store"
)

func TestMultipleNodes(t *testing.T) {
	tests := []int{1, 3, 5, 9, 11}

	for _, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)
		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestMultipleTLSNodes(t *testing.T) {
	tests := []int{1, 3, 5}

	for _, tt := range tests {
		es, hs := buildCluster(tt, true)
		waitCluster(t, es)
		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestV2Redirect(t *testing.T) {
	es, hs := buildCluster(3, false)
	waitCluster(t, es)
	u := hs[1].URL
	ru := fmt.Sprintf("%s%s", hs[0].URL, "/v2/keys/foo")
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
	destoryCluster(t, es, hs)
	afterTest(t)
}

func TestAdd(t *testing.T) {
	tests := []int{3, 4, 5, 6}

	for _, tt := range tests {
		es := make([]*Server, tt)
		hs := make([]*httptest.Server, tt)
		for i := 0; i < tt; i++ {
			c := config.New()
			if i > 0 {
				c.Peers = []string{hs[0].URL}
			}
			es[i], hs[i] = initTestServer(c, int64(i), false)
		}

		go es[0].Run()
		waitMode(participantMode, es[0])

		for i := 1; i < tt; i++ {
			id := int64(i)
			for {
				lead := es[0].p.node.Leader()
				if lead == -1 {
					time.Sleep(defaultElection * es[0].tickDuration)
					continue
				}

				err := es[lead].p.add(id, es[id].raftPubAddr, es[id].pubAddr)
				if err == nil {
					break
				}
				switch err {
				case tmpErr:
					time.Sleep(defaultElection * es[0].tickDuration)
				case raftStopErr, stopErr:
					t.Fatalf("#%d on %d: unexpected stop", i, lead)
				default:
					t.Fatal(err)
				}
			}
			go es[i].Run()
			waitMode(participantMode, es[i])

			for j := 0; j <= i; j++ {
				p := fmt.Sprintf("%s/%d", v2machineKVPrefix, id)
				w, err := es[j].p.Watch(p, false, false, 1)
				if err != nil {
					t.Errorf("#%d on %d: %v", i, j, err)
					break
				}
				<-w.EventChan
			}
		}

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestRemove(t *testing.T) {
	tests := []int{3, 4, 5, 6}

	for k, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)

		lead, _ := waitLeader(es)
		config := config.NewClusterConfig()
		config.ActiveSize = 0
		if err := es[lead].p.setClusterConfig(config); err != nil {
			t.Fatalf("#%d: setClusterConfig err = %v", k, err)
		}

		// we don't remove the machine from 2-node cluster because it is
		// not 100 percent safe in our raft.
		// TODO(yichengq): improve it later.
		for i := 0; i < tt-2; i++ {
			id := int64(i)
			send := id
			for {
				send++
				if send > int64(tt-1) {
					send = id
				}

				lead := es[send].p.node.Leader()
				if lead == -1 {
					time.Sleep(defaultElection * 5 * time.Millisecond)
					continue
				}

				err := es[lead].p.remove(id)
				if err == nil {
					break
				}
				switch err {
				case tmpErr:
					time.Sleep(defaultElection * 5 * time.Millisecond)
				case raftStopErr, stopErr:
					if lead == id {
						break
					}
				default:
					t.Fatal(err)
				}

			}

			waitMode(standbyMode, es[i])
		}

		destoryCluster(t, es, hs)
	}
	afterTest(t)
	// ensure that no goroutines are running
	TestGoroutinesRunning(t)
}

func TestBecomeStandby(t *testing.T) {
	size := 5
	round := 1

	for j := 0; j < round; j++ {
		es, hs := buildCluster(size, false)
		waitCluster(t, es)

		lead, _ := waitActiveLeader(es)
		i := rand.Intn(size)
		// cluster only demotes follower
		if int64(i) == lead {
			i = (i + 1) % size
		}
		id := int64(i)

		config := config.NewClusterConfig()
		config.SyncInterval = 1000

		config.ActiveSize = size - 1
		if err := es[lead].p.setClusterConfig(config); err != nil {
			t.Fatalf("#%d: setClusterConfig err = %v", i, err)
		}
		for {
			err := es[lead].p.remove(id)
			if err == nil {
				break
			}
			switch err {
			case tmpErr:
				time.Sleep(defaultElection * 5 * time.Millisecond)
			default:
				t.Fatalf("#%d: remove err = %v", i, err)
			}
		}

		waitMode(standbyMode, es[i])

		var leader int64
		for k := 0; k < 3; k++ {
			leader, _ = es[i].s.leaderInfo()
			if leader != noneId {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if g := leader; g != lead {
			t.Errorf("#%d: lead = %d, want %d", i, g, lead)
		}

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestReleaseVersion(t *testing.T) {
	es, hs := buildCluster(1, false)

	resp, err := http.Get(hs[0].URL + "/version")
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

	for i := range hs {
		es[len(hs)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
}

func TestVersionCheck(t *testing.T) {
	es, hs := buildCluster(1, false)
	u := hs[0].URL

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

	for i := range hs {
		es[len(hs)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
}

func buildCluster(number int, tls bool) ([]*Server, []*httptest.Server) {
	bootstrapper := 0
	es := make([]*Server, number)
	hs := make([]*httptest.Server, number)
	var seed string

	for i := range es {
		c := config.New()
		if seed != "" {
			c.Peers = []string{seed}
		}
		es[i], hs[i] = initTestServer(c, int64(i), tls)

		if i == bootstrapper {
			seed = hs[i].URL
		} else {
			// wait for the previous configuration change to be committed
			// or this configuration request might be dropped
			w, err := es[0].p.Watch(v2machineKVPrefix, true, false, uint64(i))
			if err != nil {
				panic(err)
			}
			<-w.EventChan
		}
		go es[i].Run()
		waitMode(participantMode, es[i])
	}
	return es, hs
}

func initTestServer(c *config.Config, id int64, tls bool) (e *Server, h *httptest.Server) {
	n, err := ioutil.TempDir(os.TempDir(), "etcd")
	if err != nil {
		panic(err)
	}
	c.DataDir = n

	e, err = New(c)
	if err != nil {
		panic(err)
	}
	e.setId(id)
	e.SetTick(time.Millisecond * 5)
	m := http.NewServeMux()
	m.Handle("/", e)
	m.Handle("/raft", e.RaftHandler())
	m.Handle("/raft/", e.RaftHandler())

	if tls {
		h = httptest.NewTLSServer(m)
	} else {
		h = httptest.NewServer(m)
	}

	e.raftPubAddr = h.URL
	e.pubAddr = h.URL
	return
}

func destoryCluster(t *testing.T, es []*Server, hs []*httptest.Server) {
	for i := range es {
		e := es[len(es)-i-1]
		e.Stop()
		err := os.RemoveAll(e.config.DataDir)
		if err != nil {
			panic(err)
			t.Fatal(err)
		}
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
}

func destroyServer(t *testing.T, e *Server, h *httptest.Server) {
	e.Stop()
	h.Close()
	err := os.RemoveAll(e.config.DataDir)
	if err != nil {
		panic(err)
		t.Fatal(err)
	}
}

func waitCluster(t *testing.T, es []*Server) {
	n := len(es)
	for _, e := range es {
		for k := 0; k < n; k++ {
			w, err := e.p.Watch(v2machineKVPrefix+fmt.Sprintf("/%d", es[k].id), true, false, 1)
			if err != nil {
				panic(err)
			}
			<-w.EventChan
		}
	}

	clusterId := es[0].p.node.ClusterId()
	for i, e := range es {
		if e.p.node.ClusterId() != clusterId {
			t.Errorf("#%d: clusterId = %x, want %x", i, e.p.node.ClusterId(), clusterId)
		}
	}
}

func waitMode(mode int64, e *Server) {
	for {
		if e.mode.Get() == mode {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// checkParticipant checks the i-th server works well as participant.
func checkParticipant(i int, es []*Server) error {
	lead, _ := waitActiveLeader(es)
	key := fmt.Sprintf("/%d", rand.Int31())
	ev, err := es[lead].p.Set(key, false, "bar", store.Permanent)
	if err != nil {
		return err
	}

	w, err := es[i].p.Watch(key, false, false, ev.Index())
	if err != nil {
		return err
	}
	select {
	case <-w.EventChan:
	case <-time.After(8 * defaultHeartbeat * es[i].tickDuration):
		return fmt.Errorf("watch timeout")
	}
	return nil
}
