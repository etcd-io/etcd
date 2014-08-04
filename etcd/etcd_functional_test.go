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
	"math/rand"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/store"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

func TestKillLeader(t *testing.T) {
	tests := []int{3, 5, 9}

	for i, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)

		var totalTime time.Duration
		for j := 0; j < tt; j++ {
			lead, _ := waitLeader(es)
			es[lead].Stop()
			hs[lead].Close()
			time.Sleep(es[0].tickDuration * defaultElection * 2)

			start := time.Now()
			if g, _ := waitLeader(es); g == lead {
				t.Errorf("#%d.%d: lead = %d, want not %d", i, j, g, lead)
			}
			take := time.Now().Sub(start)
			totalTime += take
			avgTime := totalTime / (time.Duration)(i+1)
			fmt.Println("Total time:", totalTime, "; Avg time:", avgTime)

			c := config.New()
			c.DataDir = es[lead].config.DataDir
			c.Addr = hs[lead].Listener.Addr().String()
			id := es[lead].id
			e, h, err := buildServer(t, c, id)
			if err != nil {
				t.Fatalf("#%d.%d: %v", i, j, err)
			}
			es[lead] = e
			hs[lead] = h
		}

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestKillRandom(t *testing.T) {
	tests := []int{3, 5, 9}

	for _, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)

		for j := 0; j < tt; j++ {
			waitLeader(es)

			toKill := make(map[int64]struct{})
			for len(toKill) != tt/2-1 {
				toKill[rand.Int63n(int64(tt))] = struct{}{}
			}
			for k := range toKill {
				es[k].Stop()
				hs[k].Close()
			}

			time.Sleep(es[0].tickDuration * defaultElection * 2)

			waitLeader(es)

			for k := range toKill {
				c := config.New()
				c.DataDir = es[k].config.DataDir
				c.Addr = hs[k].Listener.Addr().String()
				id := es[k].id
				e, h, err := buildServer(t, c, id)
				if err != nil {
					t.Fatal(err)
				}
				es[k] = e
				hs[k] = h
			}
		}

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestJoinThroughFollower(t *testing.T) {
	tests := []int{3, 4, 5, 6}

	for _, tt := range tests {
		es := make([]*Server, tt)
		hs := make([]*httptest.Server, tt)
		for i := 0; i < tt; i++ {
			c := config.New()
			if i > 0 {
				c.Peers = []string{hs[i-1].URL}
			}
			es[i], hs[i] = initTestServer(c, int64(i), false)
		}

		go es[0].Run()

		for i := 1; i < tt; i++ {
			go es[i].Run()
			waitLeader(es[:i])
		}
		waitCluster(t, es)

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestClusterConfigReload(t *testing.T) {
	tests := []int{3, 4, 5, 6}

	for i, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)

		lead, _ := waitLeader(es)
		conf := config.NewClusterConfig()
		conf.ActiveSize = 15
		conf.RemoveDelay = 60
		if err := es[lead].p.setClusterConfig(conf); err != nil {
			t.Fatalf("#%d: setClusterConfig err = %v", i, err)
		}

		for k := range es {
			es[k].Stop()
			hs[k].Close()
		}

		for k := range es {
			c := config.New()
			c.DataDir = es[k].config.DataDir
			c.Addr = hs[k].Listener.Addr().String()
			id := es[k].id
			e, h, err := buildServer(t, c, id)
			if err != nil {
				t.Fatal(err)
			}
			es[k] = e
			hs[k] = h
		}

		lead, _ = waitLeader(es)
		// wait for msgAppResp to commit all entries
		time.Sleep(2 * defaultHeartbeat * es[lead].tickDuration)
		if g := es[lead].p.clusterConfig(); !reflect.DeepEqual(g, conf) {
			t.Errorf("#%d: clusterConfig = %+v, want %+v", i, g, conf)
		}

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestMultiNodeKillOne(t *testing.T) {
	tests := []int{5}

	for i, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)

		stop := make(chan bool)
		go keepSetting(hs[0].URL, stop)

		for j := 0; j < 10; j++ {
			idx := rand.Int() % tt
			es[idx].Stop()
			hs[idx].Close()

			c := config.New()
			c.DataDir = es[idx].config.DataDir
			c.Addr = hs[idx].Listener.Addr().String()
			id := es[idx].id
			e, h, err := buildServer(t, c, id)
			if err != nil {
				t.Fatalf("#%d.%d: %v", i, j, err)
			}
			es[idx] = e
			hs[idx] = h
		}

		stop <- true
		<-stop

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func TestMultiNodeKillAllAndRecovery(t *testing.T) {
	tests := []int{5}

	for i, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)
		waitLeader(es)

		c := etcd.NewClient([]string{hs[0].URL})
		for i := 0; i < 10; i++ {
			if _, err := c.Set("foo", "bar", 0); err != nil {
				panic(err)
			}
		}

		for k := range es {
			es[k].Stop()
			hs[k].Close()
		}

		for k := range es {
			c := config.New()
			c.DataDir = es[k].config.DataDir
			c.Addr = hs[k].Listener.Addr().String()
			id := es[k].id
			e, h, err := buildServer(t, c, id)
			if err != nil {
				t.Fatalf("#%d.%d: %v", i, k, err)
			}
			es[k] = e
			hs[k] = h
		}

		waitLeader(es)
		res, err := c.Set("foo", "bar", 0)
		if err != nil {
			t.Fatalf("#%d: set err after recovery: %v", err)
		}
		if g := res.Node.ModifiedIndex; g != 16 {
			t.Errorf("#%d: modifiedIndex = %d, want %d", i, g, 16)
		}

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

func BenchmarkEndToEndSet(b *testing.B) {
	es, hs := buildCluster(3, false)
	waitLeader(es)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := es[0].p.Set("foo", false, "bar", store.Permanent)
		if err != nil {
			panic("unexpect error")
		}
	}
	b.StopTimer()
	destoryCluster(nil, es, hs)
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

		destoryCluster(t, es, hs)
	}
	afterTest(t)
}

// Sending set commands
func keepSetting(urlStr string, stop chan bool) {
	stopSet := false
	i := 0
	c := etcd.NewClient([]string{urlStr})
	for {
		key := fmt.Sprintf("%s_%v", "foo", i)

		result, err := c.Set(key, "bar", 0)

		if err != nil || result.Node.Key != "/"+key || result.Node.Value != "bar" {
			select {
			case <-stop:
				stopSet = true

			default:
			}
		}

		select {
		case <-stop:
			stopSet = true

		default:
		}

		if stopSet {
			break
		}

		i++
	}
	stop <- true
}

type leadterm struct {
	lead int64
	term int64
}

func waitActiveLeader(es []*Server) (lead, term int64) {
	for {
		if l, t := waitLeader(es); l >= 0 && es[l].mode.Get() == participantMode {
			return l, t
		}
	}
}

// waitLeader waits until all alive servers are checked to have the same leader.
// WARNING: The lead returned is not guaranteed to be actual leader.
func waitLeader(es []*Server) (lead, term int64) {
	for {
		ls := make([]leadterm, 0, len(es))
		for i := range es {
			switch es[i].mode.Get() {
			case participantMode:
				ls = append(ls, getLead(es[i]))
			case standbyMode:
				//TODO(xiangli) add standby support
			case stopMode:
			}
		}
		if isSameLead(ls) {
			return ls[0].lead, ls[0].term
		}
		time.Sleep(es[0].tickDuration * defaultElection)
	}
}

func getLead(s *Server) leadterm {
	return leadterm{s.p.node.Leader(), s.p.node.Term()}
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
