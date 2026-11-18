// Copyright 2023 The etcd Authors
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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestLeaseRevoke_IgnoreOldLeader verifies that leases shouldn't be revoked
// by old leader.
// See the case 1 in https://github.com/etcd-io/etcd/issues/15247#issuecomment-1777862093.
func TestLeaseRevoke_IgnoreOldLeader(t *testing.T) {
	t.Run("3 members", func(t *testing.T) {
		testLeaseRevokeIssue(t, 3, true)
	})
	t.Run("5 members", func(t *testing.T) {
		testLeaseRevokeIssue(t, 5, true)
	})
}

// TestLeaseRevoke_ClientSwitchToOtherMember verifies that leases shouldn't
// be revoked by new leader.
// See the case 2 in https://github.com/etcd-io/etcd/issues/15247#issuecomment-1777862093.
func TestLeaseRevoke_ClientSwitchToOtherMember(t *testing.T) {
	t.Run("3 members", func(t *testing.T) {
		testLeaseRevokeIssue(t, 3, false)
	})
	t.Run("5 members", func(t *testing.T) {
		testLeaseRevokeIssue(t, 5, false)
	})
}

// TestLeaseRevoke_PausedLeader verifies that a leader whose process is paused
// for longer than the leases' TTL, and replaced meanwhile, doesn't revoke on
// resume the leases that clients kept alive through the other members.
// See https://github.com/etcd-io/etcd/issues/22714.
func TestLeaseRevoke_PausedLeader(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := t.Context()

	t.Log("Starting a new etcd cluster")
	epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithClusterSize(3))
	require.NoError(t, err)
	defer func() {
		require.NoErrorf(t, epc.Close(), "error closing etcd processes")
	}()

	leaderIdx := epc.WaitLeader(t)
	var followerEps []string
	for i, proc := range epc.Procs {
		if i != leaderIdx {
			followerEps = append(followerEps, proc.EndpointsGRPC()...)
		}
	}
	t.Logf("Leader index: %d, creating a client connected to the followers only: %v", leaderIdx, followerEps)
	client, err := clientv3.New(clientv3.Config{Endpoints: followerEps, DialTimeout: 3 * time.Second})
	require.NoError(t, err)
	defer client.Close()

	resp, err := client.Status(ctx, followerEps[0])
	require.NoError(t, err)
	oldLeaderID := resp.Leader

	// The TTL must outlast the request timeout of the renewals that the
	// followers forward to the paused leader, so that the client keeps the
	// leases alive until a new leader is elected.
	const (
		leaseCount = 10
		ttl        = 20 // seconds
	)
	t.Logf("Creating %d leases with a key each, kept alive through the followers", leaseCount)
	keepAliveCtx, stopKeepAlive := context.WithCancel(ctx)
	defer stopKeepAlive()
	for i := range leaseCount {
		leaseRsp, gerr := client.Grant(ctx, ttl)
		require.NoError(t, gerr)
		_, perr := client.Put(ctx, fmt.Sprintf("key-%d", i), "bar", clientv3.WithLease(leaseRsp.ID))
		require.NoError(t, perr)
		respC, kerr := client.KeepAlive(keepAliveCtx, leaseRsp.ID)
		require.NoError(t, kerr)
		// The client gives up a lease whose first keepalive gets no answer,
		// so wait for it before pausing the leader.
		_, ok := <-respC
		require.Truef(t, ok, "no first keepalive response for lease %x", leaseRsp.ID)
		go func() {
			for range respC {
			}
		}()
	}

	t.Log("Pausing the leader for longer than the TTL: it is replaced, and its own lessor sees every lease expired")
	require.NoError(t, epc.Procs[leaderIdx].Pause())
	time.Sleep((ttl + 5) * time.Second)
	require.NoError(t, epc.Procs[leaderIdx].Resume())

	resp, err = client.Status(ctx, followerEps[0])
	require.NoError(t, err)
	require.NotEqualf(t, oldLeaderID, resp.Leader, "the paused leader should have been replaced")

	t.Log("Sleeping 5 seconds: the resumed member would revoke the expired leases right away")
	time.Sleep(5 * time.Second)

	t.Log("Confirming no lease kept alive was revoked")
	leases, err := client.Leases(ctx)
	require.NoError(t, err)
	require.Len(t, leases.Leases, leaseCount)
	keys, err := client.Get(ctx, "key-", clientv3.WithPrefix(), clientv3.WithCountOnly())
	require.NoError(t, err)
	require.Equal(t, int64(leaseCount), keys.Count)
}

func testLeaseRevokeIssue(t *testing.T, clusterSize int, connectToOneFollower bool) {
	e2e.BeforeTest(t)

	ctx := t.Context()

	t.Log("Starting a new etcd cluster")
	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(clusterSize),
		e2e.WithGoFailEnabled(true),
		e2e.WithGoFailClientTimeout(40*time.Second),
	)
	require.NoError(t, err)
	defer func() {
		require.NoErrorf(t, epc.Close(), "error closing etcd processes")
	}()

	leaderIdx := epc.WaitLeader(t)
	t.Logf("Leader index: %d", leaderIdx)

	epsForNormalOperations := epc.Procs[(leaderIdx+2)%clusterSize].EndpointsGRPC()
	t.Logf("Creating a client for normal operations: %v", epsForNormalOperations)
	client, err := clientv3.New(clientv3.Config{Endpoints: epsForNormalOperations, DialTimeout: 3 * time.Second})
	require.NoError(t, err)
	defer client.Close()

	var epsForLeaseKeepAlive []string
	if connectToOneFollower {
		epsForLeaseKeepAlive = epc.Procs[(leaderIdx+1)%clusterSize].EndpointsGRPC()
	} else {
		epsForLeaseKeepAlive = epc.EndpointsGRPC()
	}
	t.Logf("Creating a client for the leaseKeepAlive operation: %v", epsForLeaseKeepAlive)
	clientForKeepAlive, err := clientv3.New(clientv3.Config{Endpoints: epsForLeaseKeepAlive, DialTimeout: 3 * time.Second})
	require.NoError(t, err)
	defer clientForKeepAlive.Close()

	resp, err := client.Status(ctx, epsForNormalOperations[0])
	require.NoError(t, err)
	oldLeaderID := resp.Leader

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
		assert.NoError(t, kerr)
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

	cctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Logf("Waiting for a new leader to be elected, old leader index: %d, old leader ID: %d", leaderIdx, oldLeaderID)
	testutils.ExecuteUntil(cctx, t, func() {
		for {
			resp, err = client.Status(ctx, epsForNormalOperations[0])
			if err == nil && resp.Leader != oldLeaderID {
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
	require.Len(t, leases.Leases, 1)

	t.Log("Waiting for the keepAlive goroutine to exit")
	close(stopC)
	<-doneC
}

func TestLeaseRevokeDuringRenew(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := t.Context()

	t.Log("Starting a new etcd cluster")
	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithGoFailEnabled(true),
		e2e.WithGoFailClientTimeout(40*time.Second),
	)
	require.NoError(t, err)
	defer func() {
		require.NoErrorf(t, epc.Close(), "error closing etcd processes")
	}()

	eps := epc.Procs[0].EndpointsGRPC()
	t.Logf("Creating two clients for lease operations: %v", eps)
	clientForRenew, err := clientv3.New(clientv3.Config{Endpoints: eps, DialTimeout: 3 * time.Second})
	require.NoError(t, err)
	defer clientForRenew.Close()

	clientForRevoke, err := clientv3.New(clientv3.Config{Endpoints: eps, DialTimeout: 3 * time.Second})
	require.NoError(t, err)
	defer clientForRevoke.Close()

	t.Log("Creating a new lease")
	leaseRsp, err := clientForRenew.Grant(ctx, 60)
	require.NoError(t, err)

	t.Log("Activate the 'beforeCheckpointInLeaseRenew' failpoint")
	require.NoError(t, epc.Procs[0].Failpoints().SetupHTTP(ctx, "beforeCheckpointInLeaseRenew", `sleep("3s")`))

	t.Logf("Starting a goroutine to keep alive the lease: %d", leaseRsp.ID)
	doneC := make(chan struct{})
	startC := make(chan struct{}, 1)
	var renewError error
	go func() {
		defer close(doneC)
		startC <- struct{}{}
		_, renewError = clientForRenew.KeepAliveOnce(ctx, leaseRsp.ID)
	}()

	t.Log("Wait for the KeepAliveOnce goroutine to get started")
	<-startC
	time.Sleep(200 * time.Millisecond)

	t.Logf("Revoke the lease: %d", leaseRsp.ID)
	_, lerr := clientForRevoke.Revoke(ctx, leaseRsp.ID)
	require.NoError(t, lerr)

	t.Log("Waiting for the keepAlive goroutine to exit")
	<-doneC

	require.ErrorIs(t, rpctypes.ErrLeaseNotFound, renewError)
}
