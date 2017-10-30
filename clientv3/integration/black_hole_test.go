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
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestBalancerUnderBlackholeNoKeepAlivePut(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAliveMutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Put(ctx, "foo", "bar")
		return err
	})
}

func TestBalancerUnderBlackholeNoKeepAliveDelete(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAliveMutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Delete(ctx, "foo")
		return err
	})
}

func TestBalancerUnderBlackholeNoKeepAliveTxn(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAliveMutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Txn(ctx).
			If(clientv3.Compare(clientv3.Version("foo"), "=", 0)).
			Then(clientv3.OpPut("foo", "bar")).
			Else(clientv3.OpPut("foo", "baz")).Commit()
		return err
	})
}

func testBalancerUnderBlackholeNoKeepAliveMutable(t *testing.T, op func(*clientv3.Client, context.Context) error) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:               2,
		SkipCreatingClient: true,
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr()}

	ccfg := clientv3.Config{
		Endpoints:   []string{eps[0]},
		DialTimeout: 1 * time.Second,
	}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// wait for eps[0] to be pinned
	mustWaitPinReady(t, cli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	cli.SetEndpoints(eps...)

	// blackhole eps[0]
	clus.Members[0].Blackhole()

	// fail first due to blackhole, retry should succeed
	// TODO: first mutable operation can succeed
	// when gRPC supports better retry on non-delivered request
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = op(cli, ctx)
		cancel()
		if i == 0 {
			if err != context.DeadlineExceeded {
				t.Fatalf("#%d: err = %v, want %v", i, err, context.DeadlineExceeded)
			}
		} else if err != nil {
			t.Errorf("#%d: mutable operation failed with error %v", i, err)
		}
	}
}
