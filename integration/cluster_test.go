// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/client"
	"go.etcd.io/etcd/v3/etcdserver"
	"go.etcd.io/etcd/v3/pkg/testutil"
)

func init() {
	// open microsecond-level time log for integration test debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
	if t := os.Getenv("ETCD_ELECTION_TIMEOUT_TICKS"); t != "" {
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			electionTicks = int(i)
		}
	}
}

func TestClusterOf1(t *testing.T) { testCluster(t, 1) }
func TestClusterOf3(t *testing.T) { testCluster(t, 3) }

func testCluster(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestTLSClusterOf3(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewClusterByConfig(t, &ClusterConfig{Size: 3, PeerTLS: &testTLSInfo})
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestClusterOf1UsingDiscovery(t *testing.T) { testClusterUsingDiscovery(t, 1) }
func TestClusterOf3UsingDiscovery(t *testing.T) { testClusterUsingDiscovery(t, 3) }

func testClusterUsingDiscovery(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	dc := NewCluster(t, 1)
	dc.Launch(t)
	defer dc.Terminate(t)
	// init discovery token space
	dcc := MustNewHTTPClient(t, dc.URLs(), nil)
	dkapi := client.NewKeysAPI(dcc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	if _, err := dkapi.Create(ctx, "/_config/size", fmt.Sprintf("%d", size)); err != nil {
		t.Fatal(err)
	}
	cancel()

	c := NewClusterByConfig(
		t,
		&ClusterConfig{Size: size, DiscoveryURL: dc.URL(0) + "/v2/keys"},
	)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestTLSClusterOf3UsingDiscovery(t *testing.T) {
	defer testutil.AfterTest(t)
	dc := NewCluster(t, 1)
	dc.Launch(t)
	defer dc.Terminate(t)
	// init discovery token space
	dcc := MustNewHTTPClient(t, dc.URLs(), nil)
	dkapi := client.NewKeysAPI(dcc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	if _, err := dkapi.Create(ctx, "/_config/size", fmt.Sprintf("%d", 3)); err != nil {
		t.Fatal(err)
	}
	cancel()

	c := NewClusterByConfig(t,
		&ClusterConfig{
			Size:         3,
			PeerTLS:      &testTLSInfo,
			DiscoveryURL: dc.URL(0) + "/v2/keys"},
	)
	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestDoubleClusterSizeOf1(t *testing.T) { testDoubleClusterSize(t, 1) }
func TestDoubleClusterSizeOf3(t *testing.T) { testDoubleClusterSize(t, 3) }

func testDoubleClusterSize(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)

	for i := 0; i < size; i++ {
		c.AddMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDoubleTLSClusterSizeOf3(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewClusterByConfig(t, &ClusterConfig{Size: 3, PeerTLS: &testTLSInfo})
	c.Launch(t)
	defer c.Terminate(t)

	for i := 0; i < 3; i++ {
		c.AddMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDecreaseClusterSizeOf3(t *testing.T) { testDecreaseClusterSize(t, 3) }
func TestDecreaseClusterSizeOf5(t *testing.T) { testDecreaseClusterSize(t, 5) }

func testDecreaseClusterSize(t *testing.T, size int) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, size)
	c.Launch(t)
	defer c.Terminate(t)

	// TODO: remove the last but one member
	for i := 0; i < size-1; i++ {
		id := c.Members[len(c.Members)-1].s.ID()
		// may hit second leader election on slow machines
		if err := c.removeMember(t, uint64(id)); err != nil {
			if strings.Contains(err.Error(), "no leader") {
				t.Logf("got leader error (%v)", err)
				i--
				continue
			}
			t.Fatal(err)
		}
		c.waitLeader(t, c.Members)
	}
	clusterMustProgress(t, c.Members)
}

func TestForceNewCluster(t *testing.T) {
	c := NewCluster(t, 3)
	c.Launch(t)
	cc := MustNewHTTPClient(t, []string{c.Members[0].URL()}, nil)
	kapi := client.NewKeysAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	resp, err := kapi.Create(ctx, "/foo", "bar")
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	cancel()
	// ensure create has been applied in this machine
	ctx, cancel = context.WithTimeout(context.Background(), requestTimeout)
	if _, err = kapi.Watcher("/foo", &client.WatcherOptions{AfterIndex: resp.Node.ModifiedIndex - 1}).Next(ctx); err != nil {
		t.Fatalf("unexpected watch error: %v", err)
	}
	cancel()

	c.Members[0].Stop(t)
	c.Members[1].Terminate(t)
	c.Members[2].Terminate(t)
	c.Members[0].ForceNewCluster = true
	err = c.Members[0].Restart(t)
	if err != nil {
		t.Fatalf("unexpected ForceRestart error: %v", err)
	}
	defer c.Members[0].Terminate(t)
	c.waitLeader(t, c.Members[:1])

	// use new http client to init new connection
	cc = MustNewHTTPClient(t, []string{c.Members[0].URL()}, nil)
	kapi = client.NewKeysAPI(cc)
	// ensure force restart keep the old data, and new cluster can make progress
	ctx, cancel = context.WithTimeout(context.Background(), requestTimeout)
	if _, err := kapi.Watcher("/foo", &client.WatcherOptions{AfterIndex: resp.Node.ModifiedIndex - 1}).Next(ctx); err != nil {
		t.Fatalf("unexpected watch error: %v", err)
	}
	cancel()
	clusterMustProgress(t, c.Members[:1])
}

func TestAddMemberAfterClusterFullRotation(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, 3)
	c.Launch(t)
	defer c.Terminate(t)

	// remove all the previous three members and add in three new members.
	for i := 0; i < 3; i++ {
		c.RemoveMember(t, uint64(c.Members[0].s.ID()))
		c.waitLeader(t, c.Members)

		c.AddMember(t)
		c.waitLeader(t, c.Members)
	}

	c.AddMember(t)
	c.waitLeader(t, c.Members)

	clusterMustProgress(t, c.Members)
}

// Ensure we can remove a member then add a new one back immediately.
func TestIssue2681(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, 5)
	c.Launch(t)
	defer c.Terminate(t)

	c.RemoveMember(t, uint64(c.Members[4].s.ID()))
	c.waitLeader(t, c.Members)

	c.AddMember(t)
	c.waitLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

// Ensure we can remove a member after a snapshot then add a new one back.
func TestIssue2746(t *testing.T) { testIssue2746(t, 5) }

// With 3 nodes TestIssue2476 sometimes had a shutdown with an inflight snapshot.
func TestIssue2746WithThree(t *testing.T) { testIssue2746(t, 3) }

func testIssue2746(t *testing.T, members int) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, members)

	for _, m := range c.Members {
		m.SnapshotCount = 10
	}

	c.Launch(t)
	defer c.Terminate(t)

	// force a snapshot
	for i := 0; i < 20; i++ {
		clusterMustProgress(t, c.Members)
	}

	c.RemoveMember(t, uint64(c.Members[members-1].s.ID()))
	c.waitLeader(t, c.Members)

	c.AddMember(t)
	c.waitLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

// Ensure etcd will not panic when removing a just started member.
func TestIssue2904(t *testing.T) {
	defer testutil.AfterTest(t)
	// start 1-member cluster to ensure member 0 is the leader of the cluster.
	c := NewCluster(t, 1)
	c.Launch(t)
	defer c.Terminate(t)

	c.AddMember(t)
	c.Members[1].Stop(t)

	// send remove member-1 request to the cluster.
	cc := MustNewHTTPClient(t, c.URLs(), nil)
	ma := client.NewMembersAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	// the proposal is not committed because member 1 is stopped, but the
	// proposal is appended to leader's raft log.
	ma.Remove(ctx, c.Members[1].s.ID().String())
	cancel()

	// restart member, and expect it to send UpdateAttributes request.
	// the log in the leader is like this:
	// [..., remove 1, ..., update attr 1, ...]
	c.Members[1].Restart(t)
	// when the member comes back, it ack the proposal to remove itself,
	// and apply it.
	<-c.Members[1].s.StopNotify()

	// terminate removed member
	c.Members[1].Terminate(t)
	c.Members = c.Members[:1]
	// wait member to be removed.
	c.waitMembersMatch(t, c.HTTPMembers())
}

// TestIssue3699 tests minority failure during cluster configuration; it was
// deadlocking.
func TestIssue3699(t *testing.T) {
	// start a cluster of 3 nodes a, b, c
	defer testutil.AfterTest(t)
	c := NewCluster(t, 3)
	c.Launch(t)
	defer c.Terminate(t)

	// make node a unavailable
	c.Members[0].Stop(t)

	// add node d
	c.AddMember(t)

	// electing node d as leader makes node a unable to participate
	leaderID := c.waitLeader(t, c.Members)
	for leaderID != 3 {
		c.Members[leaderID].Stop(t)
		<-c.Members[leaderID].s.StopNotify()
		// do not restart the killed member immediately.
		// the member will advance its election timeout after restart,
		// so it will have a better chance to become the leader again.
		time.Sleep(time.Duration(electionTicks * int(tickDuration)))
		c.Members[leaderID].Restart(t)
		leaderID = c.waitLeader(t, c.Members)
	}

	// bring back node a
	// node a will remain useless as long as d is the leader.
	if err := c.Members[0].Restart(t); err != nil {
		t.Fatal(err)
	}
	select {
	// waiting for ReadyNotify can take several seconds
	case <-time.After(10 * time.Second):
		t.Fatalf("waited too long for ready notification")
	case <-c.Members[0].s.StopNotify():
		t.Fatalf("should not be stopped")
	case <-c.Members[0].s.ReadyNotify():
	}
	// must waitLeader so goroutines don't leak on terminate
	c.waitLeader(t, c.Members)

	// try to participate in cluster
	cc := MustNewHTTPClient(t, []string{c.URL(0)}, c.cfg.ClientTLS)
	kapi := client.NewKeysAPI(cc)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	if _, err := kapi.Set(ctx, "/foo", "bar", nil); err != nil {
		t.Fatalf("unexpected error on Set (%v)", err)
	}
	cancel()
}

// TestRejectUnhealthyAdd ensures an unhealthy cluster rejects adding members.
func TestRejectUnhealthyAdd(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, 3)
	for _, m := range c.Members {
		m.ServerConfig.StrictReconfigCheck = true
	}
	c.Launch(t)
	defer c.Terminate(t)

	// make cluster unhealthy and wait for downed peer
	c.Members[0].Stop(t)
	c.WaitLeader(t)

	// all attempts to add member should fail
	for i := 1; i < len(c.Members); i++ {
		err := c.addMemberByURL(t, c.URL(i), "unix://foo:12345")
		if err == nil {
			t.Fatalf("should have failed adding peer")
		}
		// TODO: client should return descriptive error codes for internal errors
		if !strings.Contains(err.Error(), "has no leader") {
			t.Errorf("unexpected error (%v)", err)
		}
	}

	// make cluster healthy
	c.Members[0].Restart(t)
	c.WaitLeader(t)
	time.Sleep(2 * etcdserver.HealthInterval)

	// add member should succeed now that it's healthy
	var err error
	for i := 1; i < len(c.Members); i++ {
		if err = c.addMemberByURL(t, c.URL(i), "unix://foo:12345"); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("should have added peer to healthy cluster (%v)", err)
	}
}

// TestRejectUnhealthyRemove ensures an unhealthy cluster rejects removing members
// if quorum will be lost.
func TestRejectUnhealthyRemove(t *testing.T) {
	defer testutil.AfterTest(t)
	c := NewCluster(t, 5)
	for _, m := range c.Members {
		m.ServerConfig.StrictReconfigCheck = true
	}
	c.Launch(t)
	defer c.Terminate(t)

	// make cluster unhealthy and wait for downed peer; (3 up, 2 down)
	c.Members[0].Stop(t)
	c.Members[1].Stop(t)
	c.WaitLeader(t)

	// reject remove active member since (3,2)-(1,0) => (2,2) lacks quorum
	err := c.removeMember(t, uint64(c.Members[2].s.ID()))
	if err == nil {
		t.Fatalf("should reject quorum breaking remove")
	}
	// TODO: client should return more descriptive error codes for internal errors
	if !strings.Contains(err.Error(), "has no leader") {
		t.Errorf("unexpected error (%v)", err)
	}

	// member stopped after launch; wait for missing heartbeats
	time.Sleep(time.Duration(electionTicks * int(tickDuration)))

	// permit remove dead member since (3,2) - (0,1) => (3,1) has quorum
	if err = c.removeMember(t, uint64(c.Members[0].s.ID())); err != nil {
		t.Fatalf("should accept removing down member")
	}

	// bring cluster to (4,1)
	c.Members[0].Restart(t)

	// restarted member must be connected for a HealthInterval before remove is accepted
	time.Sleep((3 * etcdserver.HealthInterval) / 2)

	// accept remove member since (4,1)-(1,0) => (3,1) has quorum
	if err = c.removeMember(t, uint64(c.Members[0].s.ID())); err != nil {
		t.Fatalf("expected to remove member, got error %v", err)
	}
}

// TestRestartRemoved ensures that restarting removed member must exit
// if 'initial-cluster-state' is set 'new' and old data directory still exists
// (see https://github.com/etcd-io/etcd/issues/7512 for more).
func TestRestartRemoved(t *testing.T) {
	defer testutil.AfterTest(t)

	// 1. start single-member cluster
	c := NewCluster(t, 1)
	for _, m := range c.Members {
		m.ServerConfig.StrictReconfigCheck = true
	}
	c.Launch(t)
	defer c.Terminate(t)

	// 2. add a new member
	c.AddMember(t)
	c.WaitLeader(t)

	oldm := c.Members[0]
	oldm.keepDataDirTerminate = true

	// 3. remove first member, shut down without deleting data
	if err := c.removeMember(t, uint64(c.Members[0].s.ID())); err != nil {
		t.Fatalf("expected to remove member, got error %v", err)
	}
	c.WaitLeader(t)

	// 4. restart first member with 'initial-cluster-state=new'
	// wrong config, expects exit within ReqTimeout
	oldm.ServerConfig.NewCluster = false
	if err := oldm.Restart(t); err != nil {
		t.Fatalf("unexpected ForceRestart error: %v", err)
	}
	defer func() {
		oldm.Close()
		os.RemoveAll(oldm.ServerConfig.DataDir)
	}()
	select {
	case <-oldm.s.StopNotify():
	case <-time.After(time.Minute):
		t.Fatalf("removed member didn't exit within %v", time.Minute)
	}
}

// clusterMustProgress ensures that cluster can make progress. It creates
// a random key first, and check the new key could be got from all client urls
// of the cluster.
func clusterMustProgress(t *testing.T, membs []*member) {
	cc := MustNewHTTPClient(t, []string{membs[0].URL()}, nil)
	kapi := client.NewKeysAPI(cc)
	key := fmt.Sprintf("foo%d", rand.Int())
	var (
		err  error
		resp *client.Response
	)
	// retry in case of leader loss induced by slow CI
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		resp, err = kapi.Create(ctx, "/"+key, "bar")
		cancel()
		if err == nil {
			break
		}
		t.Logf("failed to create key on %q (%v)", membs[0].URL(), err)
	}
	if err != nil {
		t.Fatalf("create on %s error: %v", membs[0].URL(), err)
	}

	for i, m := range membs {
		u := m.URL()
		mcc := MustNewHTTPClient(t, []string{u}, nil)
		mkapi := client.NewKeysAPI(mcc)
		mctx, mcancel := context.WithTimeout(context.Background(), requestTimeout)
		if _, err := mkapi.Watcher(key, &client.WatcherOptions{AfterIndex: resp.Node.ModifiedIndex - 1}).Next(mctx); err != nil {
			t.Fatalf("#%d: watch on %s error: %v", i, u, err)
		}
		mcancel()
	}
}

func TestSpeedyTerminate(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	// Stop/Restart so requests will time out on lost leaders
	for i := 0; i < 3; i++ {
		clus.Members[i].Stop(t)
		clus.Members[i].Restart(t)
	}
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		clus.Terminate(t)
	}()
	select {
	case <-time.After(10 * time.Second):
		t.Fatalf("cluster took too long to terminate")
	case <-donec:
	}
}
