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

// +build !cluster_proxy

package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"

	"golang.org/x/net/context"
)

var errExpected = errors.New("expected error")

// TestBalancerUnderNetworkPartitionPut tests when one member becomes isolated,
// first Put request fails, and following retry succeeds with client balancer
// switching to others.
func TestBalancerUnderNetworkPartitionPut(t *testing.T) {
	testBalancerUnderNetworkPartition(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Put(ctx, "a", "b")
		if err == context.DeadlineExceeded || err == rpctypes.ErrTimeout {
			return errExpected
		}
		return err
	}, time.Second)
}

func TestBalancerUnderNetworkPartitionDelete(t *testing.T) {
	testBalancerUnderNetworkPartition(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Delete(ctx, "a")
		if err == context.DeadlineExceeded || err == rpctypes.ErrTimeout {
			return errExpected
		}
		return err
	}, time.Second)
}

func TestBalancerUnderNetworkPartitionTxn(t *testing.T) {
	testBalancerUnderNetworkPartition(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Txn(ctx).
			If(clientv3.Compare(clientv3.Version("foo"), "=", 0)).
			Then(clientv3.OpPut("foo", "bar")).
			Else(clientv3.OpPut("foo", "baz")).Commit()
		if err == context.DeadlineExceeded || err == rpctypes.ErrTimeout {
			return errExpected
		}
		return err
	}, time.Second)
}

// TestBalancerUnderNetworkPartitionLinearizableGetWithLongTimeout tests
// when one member becomes isolated, first quorum Get request succeeds
// by switching endpoints within the timeout (long enough to cover endpoint switch).
func TestBalancerUnderNetworkPartitionLinearizableGetWithLongTimeout(t *testing.T) {
	testBalancerUnderNetworkPartition(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "a")
		return err
	}, 7*time.Second)
}

// TestBalancerUnderNetworkPartitionLinearizableGetWithShortTimeout tests
// when one member becomes isolated, first quorum Get request fails,
// and following retry succeeds with client balancer switching to others.
func TestBalancerUnderNetworkPartitionLinearizableGetWithShortTimeout(t *testing.T) {
	testBalancerUnderNetworkPartition(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "a")
		if err == context.DeadlineExceeded {
			return errExpected
		}
		return err
	}, time.Second)
}

func TestBalancerUnderNetworkPartitionSerializableGet(t *testing.T) {
	testBalancerUnderNetworkPartition(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "a", clientv3.WithSerializable())
		return err
	}, time.Second)
}

func testBalancerUnderNetworkPartition(t *testing.T, op func(*clientv3.Client, context.Context) error, timeout time.Duration) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:                 3,
		GRPCKeepAliveMinTime: time.Millisecond, // avoid too_many_pings
		SkipCreatingClient:   true,
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr()}

	// expect pin eps[0]
	ccfg := clientv3.Config{
		Endpoints:   []string{eps[0]},
		DialTimeout: 3 * time.Second,
	}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// wait for eps[0] to be pinned
	mustWaitPinReady(t, cli)

	// add other endpoints for later endpoint switch
	cli.SetEndpoints(eps...)
	clus.Members[0].InjectPartition(t, clus.Members[1:]...)

	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err = op(cli, ctx)
		cancel()
		if err == nil {
			break
		}
		if err != errExpected {
			t.Errorf("#%d: expected %v, got %v", i, errExpected, err)
		}
		// give enough time for endpoint switch
		// TODO: remove random sleep by syncing directly with balancer
		if i == 0 {
			time.Sleep(5 * time.Second)
		}
	}
	if err != nil {
		t.Errorf("balancer did not switch in time (%v)", err)
	}
}

// TestBalancerUnderNetworkPartitionLinearizableGetLeaderElection ensures balancer
// switches endpoint when leader fails and linearizable get requests returns
// "etcdserver: request timed out".
func TestBalancerUnderNetworkPartitionLinearizableGetLeaderElection(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:               3,
		SkipCreatingClient: true,
	})
	defer clus.Terminate(t)
	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr()}

	lead := clus.WaitLeader(t)

	timeout := 3 * clus.Members[(lead+1)%2].ServerConfig.ReqTimeout()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{eps[(lead+1)%2]},
		DialTimeout: 1 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// wait for non-leader to be pinned
	mustWaitPinReady(t, cli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	cli.SetEndpoints(eps[lead], eps[(lead+1)%2])

	// isolate leader
	clus.Members[lead].InjectPartition(t, clus.Members[(lead+1)%3], clus.Members[(lead+2)%3])

	// expects balancer endpoint switch while ongoing leader election
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	_, err = cli.Get(ctx, "a")
	cancel()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBalancerUnderNetworkPartitionWatchLeader(t *testing.T) {
	testBalancerUnderNetworkPartitionWatch(t, true)
}

func TestBalancerUnderNetworkPartitionWatchFollower(t *testing.T) {
	testBalancerUnderNetworkPartitionWatch(t, false)
}

// testBalancerUnderNetworkPartitionWatch ensures watch stream
// to a partitioned node be closed when context requires leader.
func testBalancerUnderNetworkPartitionWatch(t *testing.T, isolateLeader bool) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:               3,
		SkipCreatingClient: true,
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr()}

	target := clus.WaitLeader(t)
	if !isolateLeader {
		target = (target + 1) % 3
	}

	// pin eps[target]
	watchCli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[target]}})
	if err != nil {
		t.Fatal(err)
	}
	defer watchCli.Close()

	// wait for eps[target] to be pinned
	mustWaitPinReady(t, watchCli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	watchCli.SetEndpoints(eps...)

	wch := watchCli.Watch(clientv3.WithRequireLeader(context.Background()), "foo", clientv3.WithCreatedNotify())
	select {
	case <-wch:
	case <-time.After(3 * time.Second):
		t.Fatal("took too long to create watch")
	}

	// isolate eps[target]
	clus.Members[target].InjectPartition(t,
		clus.Members[(target+1)%3],
		clus.Members[(target+2)%3],
	)

	select {
	case ev := <-wch:
		if len(ev.Events) != 0 {
			t.Fatal("expected no event")
		}
		if err = ev.Err(); err != rpctypes.ErrNoLeader {
			t.Fatalf("expected %v, got %v", rpctypes.ErrNoLeader, err)
		}
	case <-time.After(3 * time.Second): // enough time to detect leader lost
		t.Fatal("took too long to detect leader lost")
	}
}
