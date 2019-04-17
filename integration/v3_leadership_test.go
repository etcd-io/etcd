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
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/testutil"
)

func TestMoveLeader(t *testing.T)        { testMoveLeader(t, true) }
func TestMoveLeaderService(t *testing.T) { testMoveLeader(t, false) }

func testMoveLeader(t *testing.T, auto bool) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	oldLeadIdx := clus.WaitLeader(t)
	oldLeadID := uint64(clus.Members[oldLeadIdx].s.ID())

	// ensure followers go through leader transition while learship transfer
	idc := make(chan uint64)
	for i := range clus.Members {
		if oldLeadIdx != i {
			go func(m *member) {
				idc <- checkLeaderTransition(m, oldLeadID)
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
	defer testutil.AfterTest(t)

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
	defer testutil.AfterTest(t)

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
	defer testutil.AfterTest(t)

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
