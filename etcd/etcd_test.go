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
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		for i := range es {
			es[len(es)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
}

func TestMultipleTLSNodes(t *testing.T) {
	tests := []int{1, 3, 5}

	for _, tt := range tests {
		es, hs := buildCluster(tt, true)
		waitCluster(t, es)
		for i := range es {
			es[len(es)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
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
	for i := range es {
		es[len(es)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
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

		for i := range hs {
			es[len(hs)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
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

		for i := range es {
			es[len(hs)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
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
		for k := 0; k < 4; k++ {
			leader, _ = es[i].s.leaderInfo()
			if leader != noneId {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if g := leader; g != lead {
			t.Errorf("#%d: lead = %d, want %d", i, g, lead)
		}

		for i := range hs {
			es[len(hs)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
}

// TODO(yichengq): cannot handle previous msgDenial correctly now
func TestModeSwitch(t *testing.T) {
	t.Skip("not passed")
	size := 5
	round := 3

	for i := 0; i < size; i++ {
		es, hs := buildCluster(size, false)
		waitCluster(t, es)

		config := config.NewClusterConfig()
		config.SyncInterval = 0
		id := int64(i)
		for j := 0; j < round; j++ {
			lead, _ := waitActiveLeader(es)
			// cluster only demotes follower
			if lead == id {
				continue
			}

			config.ActiveSize = size - 1
			if err := es[lead].p.setClusterConfig(config); err != nil {
				t.Fatalf("#%d: setClusterConfig err = %v", i, err)
			}
			if err := es[lead].p.remove(id); err != nil {
				t.Fatalf("#%d: remove err = %v", i, err)
			}

			waitMode(standbyMode, es[i])

			for k := 0; k < 4; k++ {
				if es[i].s.leader != noneId {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if g := es[i].s.leader; g != lead {
				t.Errorf("#%d: lead = %d, want %d", i, g, lead)
			}

			config.ActiveSize = size
			if err := es[lead].p.setClusterConfig(config); err != nil {
				t.Fatalf("#%d: setClusterConfig err = %v", i, err)
			}

			waitMode(participantMode, es[i])

			if err := checkParticipant(i, es); err != nil {
				t.Errorf("#%d: check alive err = %v", i, err)
			}
		}

		for i := range hs {
			es[len(hs)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
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
	e = New(c, id)
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

func waitCluster(t *testing.T, es []*Server) {
	n := len(es)
	for i, e := range es {
		var index uint64
		for k := 0; k < n; k++ {
			index++
			w, err := e.p.Watch(v2machineKVPrefix, true, false, index)
			if err != nil {
				panic(err)
			}
			v := <-w.EventChan
			// join command may appear several times due to retry
			// when timeout
			if k > 0 {
				pw := fmt.Sprintf("%s/%d", v2machineKVPrefix, k-1)
				if v.Node.Key == pw {
					continue
				}
			}
			ww := fmt.Sprintf("%s/%d", v2machineKVPrefix, k)
			if v.Node.Key != ww {
				t.Errorf("#%d path = %v, want %v", i, v.Node.Key, ww)
			}
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
