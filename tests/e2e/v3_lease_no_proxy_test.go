// Copyright 2024 The etcd Authors
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

//go:build !cluster_proxy

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestLeaseRevoke_IgnoreOldLeader verifies that leases shouldn't be revoked
// by old leader.
// See the case 1 in https://github.com/etcd-io/etcd/issues/15247#issuecomment-1777862093.
func TestLeaseRevoke_IgnoreOldLeader(t *testing.T) {
	testLeaseRevokeIssue(t, true)
}

// TestLeaseRevoke_ClientSwitchToOtherMember verifies that leases shouldn't
// be revoked by new leader.
// See the case 2 in https://github.com/etcd-io/etcd/issues/15247#issuecomment-1777862093.
func TestLeaseRevoke_ClientSwitchToOtherMember(t *testing.T) {
	testLeaseRevokeIssue(t, false)
}

func testLeaseRevokeIssue(t *testing.T, connectToOneFollower bool) {
	e2e.BeforeTest(t)

	ctx := context.Background()

	t.Log("Starting a new etcd cluster")
	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize:         3,
		GoFailEnabled:       true,
		GoFailClientTimeout: 40 * time.Second,
	})
	require.NoError(t, err)
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	leaderIdx := epc.WaitLeader(t)
	t.Logf("Leader index: %d", leaderIdx)

	epsForNormalOperations := epc.Procs[(leaderIdx+2)%3].EndpointsGRPC()
	t.Logf("Creating a client for normal operations: %v", epsForNormalOperations)
	client, err := clientv3.New(clientv3.Config{Endpoints: epsForNormalOperations, DialTimeout: 3 * time.Second})
	require.NoError(t, err)
	defer client.Close()

	var epsForLeaseKeepAlive []string
	if connectToOneFollower {
		epsForLeaseKeepAlive = epc.Procs[(leaderIdx+1)%3].EndpointsGRPC()
	} else {
		epsForLeaseKeepAlive = epc.EndpointsGRPC()
	}
	t.Logf("Creating a client for the leaseKeepAlive operation: %v", epsForLeaseKeepAlive)
	clientForKeepAlive, err := clientv3.New(clientv3.Config{Endpoints: epsForLeaseKeepAlive, DialTimeout: 3 * time.Second})
	require.NoError(t, err)
	defer clientForKeepAlive.Close()

	resp, err := client.Status(ctx, epsForNormalOperations[0])
	require.NoError(t, err)
	oldLeaderId := resp.Leader

	t.Log("Creating a new lease")
	leaseRsp, err := client.Grant(ctx, 20)
	require.NoError(t, err)

	t.Log("Starting a goroutine to keep alive the lease")
	doneC := make(chan struct{})
	stopC := make(chan struct{})
	startC := make(chan struct{}, 1)
	go func() {
		defer close(doneC)

		respC, kerr := clientForKeepAlive.KeepAlive(ctx, leaseRsp.ID)
		require.NoError(t, kerr)
		// ensure we have received the first response from the server
		<-respC
		startC <- struct{}{}

		for {
			select {
			case <-stopC:
				return
			case <-respC:
			}
		}
	}()

	t.Log("Wait for the keepAlive goroutine to get started")
	<-startC

	t.Log("Trigger the failpoint to simulate stalled writing")
	err = epc.Procs[leaderIdx].Failpoints().SetupHTTP(ctx, "raftBeforeSave", `sleep("30s")`)
	require.NoError(t, err)

	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Logf("Waiting for a new leader to be elected, old leader index: %d, old leader ID: %d", leaderIdx, oldLeaderId)
	testutils.ExecuteUntil(cctx, t, func() {
		for {
			resp, err = client.Status(ctx, epsForNormalOperations[0])
			if err == nil && resp.Leader != oldLeaderId {
				t.Logf("A new leader has already been elected, new leader index: %d", resp.Leader)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	cancel()

	t.Log("Writing a key/value pair")
	_, err = client.Put(ctx, "foo", "bar")
	require.NoError(t, err)

	t.Log("Sleeping 30 seconds")
	time.Sleep(30 * time.Second)

	t.Log("Remove the failpoint 'raftBeforeSave'")
	err = epc.Procs[leaderIdx].Failpoints().DeactivateHTTP(ctx, "raftBeforeSave")
	require.NoError(t, err)

	// By default, etcd tries to revoke leases every 7 seconds.
	t.Log("Sleeping 10 seconds")
	time.Sleep(10 * time.Second)

	t.Log("Confirming the lease isn't revoked")
	leases, err := client.Leases(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(leases.Leases))

	t.Log("Waiting for the keepAlive goroutine to exit")
	close(stopC)
	<-doneC
}
