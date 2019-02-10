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
