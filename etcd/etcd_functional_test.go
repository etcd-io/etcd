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
	"math/rand"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

func TestKillLeader(t *testing.T) {
	defer afterTest(t)
	tests := []int{3, 5, 9}

	for i, tt := range tests {
		cl := testCluster{Size: tt}
		cl.Start()
		for j := 0; j < tt; j++ {
			lead, _ := cl.Leader()
			cl.Node(int(lead)).Stop()
			// wait for leader election timeout
			time.Sleep(cl.Node(0).e.tickDuration * defaultElection * 2)
			if g, _ := cl.Leader(); g == lead {
				t.Errorf("#%d.%d: lead = %d, want not %d", i, j, g, lead)
			}
			cl.Node(int(lead)).Start()
			cl.Node(int(lead)).WaitMode(participantMode, 3)
		}
		cl.Destroy()
	}
}

func TestKillRandom(t *testing.T) {
	defer afterTest(t)
	tests := []int{3, 5, 9}

	for _, tt := range tests {
		cl := testCluster{Size: tt}
		cl.Start()
		for j := 0; j < tt; j++ {
			// we cannot kill the majority
			// wait for the majority
			cl.Leader()
			toKill := make(map[int64]struct{})
			for len(toKill) != tt/2-1 {
				toKill[rand.Int63n(int64(tt))] = struct{}{}
			}
			for k := range toKill {
				cl.Node(int(k)).Stop()
			}

			// wait for leader election timeout
			time.Sleep(cl.Node(0).e.tickDuration * defaultElection * 2)
			cl.Leader()
			for k := range toKill {
				cl.Node(int(k)).Start()
				cl.Node(int(k)).WaitMode(participantMode, 3)
			}
		}
		cl.Destroy()
	}
}

func TestJoinThroughFollower(t *testing.T) {
	defer afterTest(t)
	tests := []int{3, 5, 7}

	for _, tt := range tests {
		bt := &testServer{}
		bt.Start()
		cl := testCluster{nodes: []*testServer{bt}}
		seed := bt.URL

		for i := 1; i < tt; i++ {
			c := newTestConfig()
			c.Peers = []string{seed}
			ts := &testServer{Config: c, Id: int64(i)}
			ts.Start()
			ts.WaitMode(participantMode, 3)
			cl.nodes = append(cl.nodes, ts)
			cl.Leader()
			seed = ts.URL
		}
		cl.Destroy()
	}
}

// func TestClusterConfigReload(t *testing.T) {
// 	defer afterTest(t)
// 	tests := []int{3, 5, 7}

// 	for i, tt := range tests {
// 		es, hs := buildCluster(tt, false)
// 		waitCluster(t, es)

// 		lead, _ := waitLeader(es)
// 		conf := config.NewClusterConfig()
// 		conf.ActiveSize = 15
// 		conf.RemoveDelay = 60
// 		if err := es[lead].p.setClusterConfig(conf); err != nil {
// 			t.Fatalf("#%d: setClusterConfig err = %v", i, err)
// 		}

// 		for k := range es {
// 			es[k].Stop()
// 			hs[k].Close()
// 		}

// 		for k := range es {
// 			c := newTestConfig()
// 			c.DataDir = es[k].config.DataDir
// 			c.Addr = hs[k].Listener.Addr().String()
// 			id := es[k].id
// 			e, h := newUnstartedTestServer(c, id, false)
// 			err := startServer(t, e)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			es[k] = e
// 			hs[k] = h
// 		}

// 		lead, _ = waitLeader(es)
// 		// wait for msgAppResp to commit all entries
// 		time.Sleep(2 * defaultHeartbeat * es[lead].tickDuration)
// 		if g := es[lead].p.clusterConfig(); !reflect.DeepEqual(g, conf) {
// 			t.Errorf("#%d: clusterConfig = %+v, want %+v", i, g, conf)
// 		}

// 		destoryCluster(t, es, hs)
// 	}
// }

func TestFiveNodeKillOneAndRecover(t *testing.T) {
	defer afterTest(t)
	cl := testCluster{Size: 5}
	cl.Start()
	for n := 0; n < 5; n++ {
		i := rand.Int() % 5
		cl.Node(i).Stop()
		cl.Leader()
		cl.Node(i).Start()
		cl.Node(i).WaitMode(participantMode, 3)
		cl.Leader()
	}
	cl.Destroy()
}

func TestFiveNodeKillAllAndRecover(t *testing.T) {
	defer afterTest(t)

	cl := testCluster{Size: 5}
	cl.Start()
	defer cl.Destroy()

	cl.Leader()
	c := etcd.NewClient([]string{cl.URL(0)})
	for i := 0; i < 10; i++ {
		if _, err := c.Set("foo", "bar", 0); err != nil {
			panic(err)
		}
	}

	cl.Stop()

	cl.Restart()
	cl.Leader()
	res, err := c.Set("foo", "bar", 0)
	if err != nil {
		t.Fatalf("set err after recovery: %v", err)
	}
	if g := res.Node.ModifiedIndex; g != 16 {
		t.Errorf("modifiedIndex = %d, want %d", g, 16)
	}
}

// TestModeSwitch tests switch mode between standby and peer.
func TestModeSwitch(t *testing.T) {
	t.Skip("not implemented")
}
