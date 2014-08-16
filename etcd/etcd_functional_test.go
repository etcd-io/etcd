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
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/conf"
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
			cl.Node(lead).Stop()
			// wait for leader election timeout
			time.Sleep(cl.Node(0).e.tickDuration * defaultElection * 2)
			if g, _ := cl.Leader(); g == lead {
				t.Errorf("#%d.%d: lead = %d, want not %d", i, j, g, lead)
			}
			cl.Node(lead).Start()
			cl.Node(lead).WaitMode(participantMode)
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
				cl.Node(int(k)).WaitMode(participantMode)
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
			c.Name = fmt.Sprint(i)
			c.Peers = []string{seed}
			ts := &testServer{Config: c}
			ts.Start()
			ts.WaitMode(participantMode)
			cl.nodes = append(cl.nodes, ts)
			cl.Leader()
			seed = ts.URL
		}
		cl.Destroy()
	}
}

func TestJoinWithoutHTTPScheme(t *testing.T) {
	bt := &testServer{}
	bt.Start()

	cl := testCluster{nodes: []*testServer{bt}}
	seed := bt.URL
	u, err := url.Parse(seed)
	if err != nil {
		t.Fatal(err)
	}
	// remove HTTP scheme
	seed = u.Host + u.Path

	for i := 1; i < 3; i++ {
		c := newTestConfig()
		c.Name = "server-" + fmt.Sprint(i)
		c.Peers = []string{seed}
		ts := &testServer{Config: c}
		ts.Start()
		ts.WaitMode(participantMode)
		cl.nodes = append(cl.nodes, ts)
		cl.Leader()
	}
	cl.Destroy()
}

func TestClusterConfigReload(t *testing.T) {
	defer afterTest(t)

	cl := &testCluster{Size: 5}
	cl.Start()
	defer cl.Destroy()

	lead, _ := cl.Leader()
	cc := conf.NewClusterConfig()
	cc.ActiveSize = 15
	cc.RemoveDelay = 60
	if err := cl.Participant(lead).setClusterConfig(cc); err != nil {
		t.Fatalf("setClusterConfig err = %v", err)
	}

	cl.Stop()
	cl.Restart()

	lead, _ = cl.Leader()
	// wait for msgAppResp to commit all entries
	time.Sleep(2 * defaultHeartbeat * cl.Participant(0).tickDuration)
	if g := cl.Participant(lead).clusterConfig(); !reflect.DeepEqual(g, cc) {
		t.Errorf("clusterConfig = %+v, want %+v", g, cc)
	}
}

func TestFiveNodeKillOneAndRecover(t *testing.T) {
	defer afterTest(t)
	cl := testCluster{Size: 5}
	cl.Start()
	for n := 0; n < 5; n++ {
		i := rand.Int() % 5
		cl.Node(i).Stop()
		cl.Leader()
		cl.Node(i).Start()
		cl.Node(i).WaitMode(participantMode)
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
