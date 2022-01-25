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

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func init() {
	// open microsecond-level time log for integration test debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
	if t := os.Getenv("ETCD_ELECTION_TIMEOUT_TICKS"); t != "" {
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			integration.ElectionTicks = int(i)
		}
	}
}

func TestClusterOf1(t *testing.T) { testCluster(t, 1) }
func TestClusterOf3(t *testing.T) { testCluster(t, 3) }

func testCluster(t *testing.T, size int) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: size})
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestTLSClusterOf3(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, PeerTLS: &integration.TestTLSInfo})
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

// Test that a cluster can progress when using separate client and server certs when peering. This supports certificate
// authorities that don't issue dual-usage certificates.
func TestTLSClusterOf3WithSpecificUsage(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, PeerTLS: &integration.TestTLSInfoWithSpecificUsage})
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}

func TestDoubleClusterSizeOf1(t *testing.T) { testDoubleClusterSize(t, 1) }
func TestDoubleClusterSizeOf3(t *testing.T) { testDoubleClusterSize(t, 3) }

func testDoubleClusterSize(t *testing.T, size int) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: size})
	defer c.Terminate(t)

	for i := 0; i < size; i++ {
		c.AddMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDoubleTLSClusterSizeOf3(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 1, PeerTLS: &integration.TestTLSInfo})
	defer c.Terminate(t)

	for i := 0; i < 3; i++ {
		c.AddMember(t)
	}
	clusterMustProgress(t, c.Members)
}

func TestDecreaseClusterSizeOf3(t *testing.T) { testDecreaseClusterSize(t, 3) }
func TestDecreaseClusterSizeOf5(t *testing.T) { testDecreaseClusterSize(t, 5) }

func testDecreaseClusterSize(t *testing.T, size int) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: size})
	defer c.Terminate(t)

	// TODO: remove the last but one member
	for i := 0; i < size-1; i++ {
		id := c.Members[len(c.Members)-1].Server.ID()
		// may hit second leader election on slow machines
		if err := c.RemoveMember(t, c.Members[0].Client, uint64(id)); err != nil {
			if strings.Contains(err.Error(), "no leader") {
				t.Logf("got leader error (%v)", err)
				i--
				continue
			}
			t.Fatal(err)
		}
		c.WaitMembersForLeader(t, c.Members)
	}
	clusterMustProgress(t, c.Members)
}

func TestForceNewCluster(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true})
	defer c.Terminate(t)

	ctx, cancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
	resp, err := c.Members[0].Client.Put(ctx, "/foo", "bar")
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	cancel()
	// ensure create has been applied in this machine
	ctx, cancel = context.WithTimeout(context.Background(), integration.RequestTimeout)
	watch := c.Members[0].Client.Watcher.Watch(ctx, "/foo", clientv3.WithRev(resp.Header.Revision-1))
	for resp := range watch {
		if len(resp.Events) != 0 {
			break
		}
		if resp.Err() != nil {
			t.Fatalf("unexpected watch error: %q", resp.Err())
		}
		if resp.Canceled {
			t.Fatalf("watch  cancelled")
		}
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
	c.WaitMembersForLeader(t, c.Members[:1])

	// use new http client to init new connection
	// ensure force restart keep the old data, and new Cluster can make progress
	ctx, cancel = context.WithTimeout(context.Background(), integration.RequestTimeout)
	watch = c.Members[0].Client.Watcher.Watch(ctx, "/foo", clientv3.WithRev(resp.Header.Revision-1))
	for resp := range watch {
		if len(resp.Events) != 0 {
			break
		}
		if resp.Err() != nil {
			t.Fatalf("unexpected watch error: %q", resp.Err())
		}
		if resp.Canceled {
			t.Fatalf("watch  cancelled")
		}
	}
	cancel()
	clusterMustProgress(t, c.Members[:1])
}

func TestAddMemberAfterClusterFullRotation(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer c.Terminate(t)

	// remove all the previous three members and add in three new members.
	for i := 0; i < 3; i++ {
		if err := c.RemoveMember(t, c.Members[0].Client, uint64(c.Members[1].Server.ID())); err != nil {
			t.Fatal(err)
		}
		c.WaitMembersForLeader(t, c.Members)

		c.AddMember(t)
		c.WaitMembersForLeader(t, c.Members)
	}

	c.AddMember(t)
	c.WaitMembersForLeader(t, c.Members)

	clusterMustProgress(t, c.Members)
}

// Ensure we can remove a member then add a new one back immediately.
func TestIssue2681(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 5})
	defer c.Terminate(t)

	if err := c.RemoveMember(t, c.Members[0].Client, uint64(c.Members[4].Server.ID())); err != nil {
		t.Fatal(err)
	}
	c.WaitMembersForLeader(t, c.Members)

	c.AddMember(t)
	c.WaitMembersForLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

// Ensure we can remove a member after a snapshot then add a new one back.
func TestIssue2746(t *testing.T) { testIssue2746(t, 5) }

// With 3 nodes TestIssue2476 sometimes had a shutdown with an inflight snapshot.
func TestIssue2746WithThree(t *testing.T) { testIssue2746(t, 3) }

func testIssue2746(t *testing.T, members int) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: members, SnapshotCount: 10})
	defer c.Terminate(t)

	// force a snapshot
	for i := 0; i < 20; i++ {
		clusterMustProgress(t, c.Members)
	}

	if err := c.RemoveMember(t, c.Members[0].Client, uint64(c.Members[members-1].Server.ID())); err != nil {
		t.Fatal(err)
	}
	c.WaitMembersForLeader(t, c.Members)

	c.AddMember(t)
	c.WaitMembersForLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

// Ensure etcd will not panic when removing a just started member.
func TestIssue2904(t *testing.T) {
	integration.BeforeTest(t)
	// start 1-member Cluster to ensure member 0 is the leader of the Cluster.
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 2, UseBridge: true})
	defer c.Terminate(t)
	c.WaitLeader(t)

	c.AddMember(t)
	c.Members[2].Stop(t)

	// send remove member-1 request to the Cluster.
	ctx, cancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
	// the proposal is not committed because member 1 is stopped, but the
	// proposal is appended to leader'Server raft log.
	c.Members[0].Client.MemberRemove(ctx, uint64(c.Members[2].Server.ID()))
	cancel()

	// restart member, and expect it to send UpdateAttributes request.
	// the log in the leader is like this:
	// [..., remove 1, ..., update attr 1, ...]
	c.Members[2].Restart(t)
	// when the member comes back, it ack the proposal to remove itself,
	// and apply it.
	<-c.Members[2].Server.StopNotify()

	// terminate removed member
	c.Members[2].Client.Close()
	c.Members[2].Terminate(t)
	c.Members = c.Members[:2]
	// wait member to be removed.
	c.WaitMembersMatch(t, c.ProtoMembers())
}

// TestIssue3699 tests minority failure during cluster configuration; it was
// deadlocking.
func TestIssue3699(t *testing.T) {
	// start a Cluster of 3 nodes a, b, c
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true})
	defer c.Terminate(t)

	// make node a unavailable
	c.Members[0].Stop(t)

	// add node d
	c.AddMember(t)

	// electing node d as leader makes node a unable to participate
	leaderID := c.WaitMembersForLeader(t, c.Members)
	for leaderID != 3 {
		c.Members[leaderID].Stop(t)
		<-c.Members[leaderID].Server.StopNotify()
		// do not restart the killed member immediately.
		// the member will advance its election timeout after restart,
		// so it will have a better chance to become the leader again.
		time.Sleep(time.Duration(integration.ElectionTicks * int(integration.TickDuration)))
		c.Members[leaderID].Restart(t)
		leaderID = c.WaitMembersForLeader(t, c.Members)
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
	case <-c.Members[0].Server.StopNotify():
		t.Fatalf("should not be stopped")
	case <-c.Members[0].Server.ReadyNotify():
	}
	// must WaitMembersForLeader so goroutines don't leak on terminate
	c.WaitMembersForLeader(t, c.Members)

	// try to participate in Cluster
	ctx, cancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
	if _, err := c.Members[0].Client.Put(ctx, "/foo", "bar"); err != nil {
		t.Fatalf("unexpected error on Set (%v)", err)
	}
	cancel()
}

// TestRejectUnhealthyAdd ensures an unhealthy cluster rejects adding members.
func TestRejectUnhealthyAdd(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true, StrictReconfigCheck: true})
	defer c.Terminate(t)

	// make Cluster unhealthy and wait for downed peer
	c.Members[0].Stop(t)
	c.WaitLeader(t)

	// all attempts to add member should fail
	for i := 1; i < len(c.Members); i++ {
		err := c.AddMemberByURL(t, c.Members[i].Client, "unix://foo:12345")
		if err == nil {
			t.Fatalf("should have failed adding peer")
		}
		// TODO: client should return descriptive error codes for internal errors
		if !strings.Contains(err.Error(), "unhealthy cluster") {
			t.Errorf("unexpected error (%v)", err)
		}
	}

	// make cluster healthy
	c.Members[0].Restart(t)
	c.WaitLeader(t)
	time.Sleep(2 * etcdserver.HealthInterval)

	// add member should succeed now that it'Server healthy
	var err error
	for i := 1; i < len(c.Members); i++ {
		if err = c.AddMemberByURL(t, c.Members[i].Client, "unix://foo:12345"); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("should have added peer to healthy Cluster (%v)", err)
	}
}

// TestRejectUnhealthyRemove ensures an unhealthy cluster rejects removing members
// if quorum will be lost.
func TestRejectUnhealthyRemove(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 5, UseBridge: true, StrictReconfigCheck: true})
	defer c.Terminate(t)

	// make cluster unhealthy and wait for downed peer; (3 up, 2 down)
	c.Members[0].Stop(t)
	c.Members[1].Stop(t)
	leader := c.WaitLeader(t)

	// reject remove active member since (3,2)-(1,0) => (2,2) lacks quorum
	err := c.RemoveMember(t, c.Members[leader].Client, uint64(c.Members[2].Server.ID()))
	if err == nil {
		t.Fatalf("should reject quorum breaking remove: %s", err)
	}
	// TODO: client should return more descriptive error codes for internal errors
	if !strings.Contains(err.Error(), "unhealthy cluster") {
		t.Errorf("unexpected error (%v)", err)
	}

	// member stopped after launch; wait for missing heartbeats
	time.Sleep(time.Duration(integration.ElectionTicks * int(integration.TickDuration)))

	// permit remove dead member since (3,2) - (0,1) => (3,1) has quorum
	if err = c.RemoveMember(t, c.Members[2].Client, uint64(c.Members[0].Server.ID())); err != nil {
		t.Fatalf("should accept removing down member: %s", err)
	}

	// bring cluster to (4,1)
	c.Members[0].Restart(t)

	// restarted member must be connected for a HealthInterval before remove is accepted
	time.Sleep((3 * etcdserver.HealthInterval) / 2)

	// accept remove member since (4,1)-(1,0) => (3,1) has quorum
	if err = c.RemoveMember(t, c.Members[1].Client, uint64(c.Members[0].Server.ID())); err != nil {
		t.Fatalf("expected to remove member, got error %v", err)
	}
}

// TestRestartRemoved ensures that restarting removed member must exit
// if 'initial-cluster-state' is set 'new' and old data directory still exists
// (see https://github.com/etcd-io/etcd/issues/7512 for more).
func TestRestartRemoved(t *testing.T) {
	integration.BeforeTest(t)

	// 1. start single-member Cluster
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 1, StrictReconfigCheck: true})
	defer c.Terminate(t)

	// 2. add a new member
	c.Cfg.StrictReconfigCheck = false
	c.AddMember(t)
	c.WaitLeader(t)

	firstMember := c.Members[0]
	firstMember.KeepDataDirTerminate = true

	// 3. remove first member, shut down without deleting data
	if err := c.RemoveMember(t, c.Members[1].Client, uint64(firstMember.Server.ID())); err != nil {
		t.Fatalf("expected to remove member, got error %v", err)
	}
	c.WaitLeader(t)

	// 4. restart first member with 'initial-cluster-state=new'
	// wrong config, expects exit within ReqTimeout
	firstMember.ServerConfig.NewCluster = false
	if err := firstMember.Restart(t); err != nil {
		t.Fatalf("unexpected ForceRestart error: %v", err)
	}
	defer func() {
		firstMember.Close()
		os.RemoveAll(firstMember.ServerConfig.DataDir)
	}()
	select {
	case <-firstMember.Server.StopNotify():
	case <-time.After(time.Minute):
		t.Fatalf("removed member didn't exit within %v", time.Minute)
	}
}

// clusterMustProgress ensures that cluster can make progress. It creates
// a random key first, and check the new key could be got from all client urls
// of the cluster.
func clusterMustProgress(t *testing.T, members []*integration.Member) {
	key := fmt.Sprintf("foo%d", rand.Int())
	var (
		err  error
		resp *clientv3.PutResponse
	)
	// retry in case of leader loss induced by slow CI
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
		resp, err = members[0].Client.Put(ctx, key, "bar")
		cancel()
		if err == nil {
			break
		}
		t.Logf("failed to create key on #0 (%v)", err)
	}
	if err != nil {
		t.Fatalf("create on #0 error: %v", err)
	}

	for i, m := range members {
		mctx, mcancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
		watch := m.Client.Watcher.Watch(mctx, key, clientv3.WithRev(resp.Header.Revision-1))
		for resp := range watch {
			if len(resp.Events) != 0 {
				break
			}
			if resp.Err() != nil {
				t.Fatalf("#%d: watch error: %q", i, resp.Err())
			}
			if resp.Canceled {
				t.Fatalf("#%d: watch: cancelled", i)
			}
		}
		mcancel()
	}
}

func TestSpeedyTerminate(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true})
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
		t.Fatalf("Cluster took too long to terminate")
	case <-donec:
	}
}
