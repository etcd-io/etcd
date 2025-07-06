// Copyright 2017 The etcd Authors
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
	"strings"
	"testing"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"golang.org/x/sync/errgroup"
)

func TestMoveLeader(t *testing.T)        { testMoveLeader(t, true) }
func TestMoveLeaderService(t *testing.T) { testMoveLeader(t, false) }

func testMoveLeader(t *testing.T, auto bool) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	oldLeadIdx := clus.WaitLeader(t)
	oldLeadID := uint64(clus.Members[oldLeadIdx].s.ID())

	// ensure followers go through leader transition while leadership transfer
	idc := make(chan uint64)
	stopc := make(chan struct{})
	defer close(stopc)

	for i := range clus.Members {
		if oldLeadIdx != i {
			go func(m *member) {
				select {
				case idc <- checkLeaderTransition(m, oldLeadID):
				case <-stopc:
				}
			}(clus.Members[i])
		}
	}

	target := uint64(clus.Members[(oldLeadIdx+1)%3].s.ID())
	if auto {
		err := clus.Members[oldLeadIdx].s.TransferLeadership()
		if err != nil {
			t.Fatal(err)
		}
	} else {
		mvc := toGRPC(clus.Client(oldLeadIdx)).Maintenance
		_, err := mvc.MoveLeader(context.TODO(), &pb.MoveLeaderRequest{TargetID: target})
		if err != nil {
			t.Fatal(err)
		}
	}

	// wait until leader transitions have happened
	var newLeadIDs [2]uint64
	for i := range newLeadIDs {
		select {
		case newLeadIDs[i] = <-idc:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for leader transition")
		}
	}

	// remaining members must agree on the same leader
	if newLeadIDs[0] != newLeadIDs[1] {
		t.Fatalf("expected same new leader %d == %d", newLeadIDs[0], newLeadIDs[1])
	}

	// new leader must be different than the old leader
	if oldLeadID == newLeadIDs[0] {
		t.Fatalf("expected old leader %d != new leader %d", oldLeadID, newLeadIDs[0])
	}

	// if move-leader were used, new leader must match transferee
	if !auto {
		if newLeadIDs[0] != target {
			t.Fatalf("expected new leader %d != target %d", newLeadIDs[0], target)
		}
	}
}

// TestMoveLeaderError ensures that request to non-leader fail.
func TestMoveLeaderError(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	oldLeadIdx := clus.WaitLeader(t)
	followerIdx := (oldLeadIdx + 1) % 3

	target := uint64(clus.Members[(oldLeadIdx+2)%3].s.ID())

	mvc := toGRPC(clus.Client(followerIdx)).Maintenance
	_, err := mvc.MoveLeader(context.TODO(), &pb.MoveLeaderRequest{TargetID: target})
	if !eqErrGRPC(err, rpctypes.ErrGRPCNotLeader) {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrGRPCNotLeader)
	}
}

// TestMoveLeaderToLearnerError ensures that leader transfer to learner member will fail.
func TestMoveLeaderToLearnerError(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	// we have to add and launch learner member after initial cluster was created, because
	// bootstrapping a cluster with learner member is not supported.
	clus.AddAndLaunchLearnerMember(t)

	learners, err := clus.GetLearnerMembers()
	if err != nil {
		t.Fatalf("failed to get the learner members in cluster: %v", err)
	}
	if len(learners) != 1 {
		t.Fatalf("added 1 learner to cluster, got %d", len(learners))
	}

	learnerID := learners[0].ID
	leaderIdx := clus.WaitLeader(t)
	cli := clus.Client(leaderIdx)
	_, err = cli.MoveLeader(context.Background(), learnerID)
	if err == nil {
		t.Fatalf("expecting leader transfer to learner to fail, got no error")
	}
	expectedErrKeywords := "bad leader transferee"
	if !strings.Contains(err.Error(), expectedErrKeywords) {
		t.Errorf("expecting error to contain %s, got %s", expectedErrKeywords, err.Error())
	}
}

// TestTransferLeadershipWithLearner ensures TransferLeadership does not timeout due to learner is
// automatically picked by leader as transferee.
func TestTransferLeadershipWithLearner(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	clus.AddAndLaunchLearnerMember(t)

	learners, err := clus.GetLearnerMembers()
	if err != nil {
		t.Fatalf("failed to get the learner members in cluster: %v", err)
	}
	if len(learners) != 1 {
		t.Fatalf("added 1 learner to cluster, got %d", len(learners))
	}

	leaderIdx := clus.WaitLeader(t)
	errCh := make(chan error, 1)
	go func() {
		// note that this cluster has 1 leader and 1 learner. TransferLeadership should return nil.
		// Leadership transfer is skipped in cluster with 1 voting member.
		errCh <- clus.Members[leaderIdx].s.TransferLeadership()
	}()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("got error during leadership transfer: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for leader transition")
	}
}

func TestFirstCommitNotification(t *testing.T) {
	BeforeTest(t)
	ctx := context.Background()
	clusterSize := 3
	cluster := NewClusterV3(t, &ClusterConfig{Size: clusterSize})
	defer cluster.Terminate(t)

	oldLeaderIdx := cluster.WaitLeader(t)
	oldLeaderClient := cluster.Client(oldLeaderIdx)

	newLeaderIdx := (oldLeaderIdx + 1) % clusterSize
	newLeaderId := uint64(cluster.Members[newLeaderIdx].ID())

	notifiers := make(map[int]<-chan struct{}, clusterSize)
	for i, clusterMember := range cluster.Members {
		notifiers[i] = clusterMember.s.FirstCommitInTermNotify()
	}

	_, err := oldLeaderClient.MoveLeader(context.Background(), newLeaderId)

	if err != nil {
		t.Errorf("got error during leadership transfer: %v", err)
	}

	t.Logf("Leadership transferred.")
	t.Logf("Submitting write to make sure empty and 'foo' index entry was already flushed")
	cli := cluster.RandClient()

	if _, err := cli.Put(ctx, "foo", "bar"); err != nil {
		t.Fatalf("Failed to put kv pair.")
	}

	// It's guaranteed now that leader contains the 'foo'->'bar' index entry.
	leaderAppliedIndex := cluster.Members[newLeaderIdx].s.AppliedIndex()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	group, groupContext := errgroup.WithContext(ctx)

	for i, notifier := range notifiers {
		member, notifier := cluster.Members[i], notifier
		group.Go(func() error {
			return checkFirstCommitNotification(groupContext, t, member, leaderAppliedIndex, notifier)
		})
	}

	err = group.Wait()
	if err != nil {
		t.Error(err)
	}
}

func checkFirstCommitNotification(
	ctx context.Context,
	t testing.TB,
	member *member,
	leaderAppliedIndex uint64,
	notifier <-chan struct{},
) error {
	// wait until server applies all the changes of leader
	for member.s.AppliedIndex() < leaderAppliedIndex {
		t.Logf("member.s.AppliedIndex():%v <= leaderAppliedIndex:%v", member.s.AppliedIndex(), leaderAppliedIndex)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	select {
	case msg, ok := <-notifier:
		if ok {
			return fmt.Errorf(
				"member with ID %d got message via notifier, msg: %v",
				member.ID(),
				msg,
			)
		}
	default:
		t.Logf("member.s.AppliedIndex():%v >= leaderAppliedIndex:%v", member.s.AppliedIndex(), leaderAppliedIndex)
		return fmt.Errorf(
			"notification was not triggered, member ID: %d",
			member.ID(),
		)
	}

	return nil
}
