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
	"context"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

// TestNetworkPartitionBalancerPut tests when one member becomes isolated,
// first Put request fails, and following retry succeeds with client balancer
// switching to others.
func TestNetworkPartitionBalancerPut(t *testing.T) {
	testNetworkPartitionBalancer(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Put(ctx, "a", "b")
		return err
	})
}

// TestNetworkPartitionBalancerGet tests when one member becomes isolated,
// first Get request fails, and following retry succeeds with client balancer
// switching to others.
func TestNetworkPartitionBalancerGet(t *testing.T) {
	testNetworkPartitionBalancer(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "a")
		return err
	})
}

func testNetworkPartitionBalancer(t *testing.T, op func(*clientv3.Client, context.Context) error) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:                 3,
		GRPCKeepAliveMinTime: time.Millisecond, // avoid too_many_pings
		SkipCreatingClient:   true,
	})
	defer clus.Terminate(t)

	// expect pin ep[0]
	ccfg := clientv3.Config{
		Endpoints:            []string{clus.Members[0].GRPCAddr()},
		DialTimeout:          3 * time.Second,
		DialKeepAliveTime:    2 * time.Second,
		DialKeepAliveTimeout: 2 * time.Second,
	}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// add other endpoints for later endpoint switch
	cli.SetEndpoints(clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr())
	clus.Members[0].InjectPartition(t, clus.Members[1:])

	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = op(cli, ctx)
		cancel()
		if err == nil {
			break
		}
		// TODO: separate put and get test for error checking.
		// we do not really expect ErrTimeout on get.
		if err != context.DeadlineExceeded && err != rpctypes.ErrTimeout {
			t.Errorf("#%d: expected %v or %v, got %v", i, context.DeadlineExceeded, rpctypes.ErrTimeout, err)
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
