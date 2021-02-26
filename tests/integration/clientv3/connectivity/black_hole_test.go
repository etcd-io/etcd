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

//go:build !cluster_proxy
// +build !cluster_proxy

package connectivity_test

import (
	"context"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/testutil"
	"go.etcd.io/etcd/tests/v3/integration"
	"go.etcd.io/etcd/tests/v3/integration/clientv3"
	"google.golang.org/grpc"
)

// TestBalancerUnderBlackholeKeepAliveWatch tests when watch discovers it cannot talk to
// blackholed endpoint, client balancer switches to healthy one.
// TODO: test server-to-client keepalive ping
func TestBalancerUnderBlackholeKeepAliveWatch(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:                 2,
		GRPCKeepAliveMinTime: time.Millisecond, // avoid too_many_pings
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr()}

	ccfg := clientv3.Config{
		Endpoints:            []string{eps[0]},
		DialTimeout:          time.Second,
		DialOptions:          []grpc.DialOption{grpc.WithBlock()},
		DialKeepAliveTime:    time.Second,
		DialKeepAliveTimeout: 500 * time.Millisecond,
	}

	// gRPC internal implementation related.
	pingInterval := ccfg.DialKeepAliveTime + ccfg.DialKeepAliveTimeout
	// 3s for slow machine to process watch and reset connections
	// TODO: only send healthy endpoint to gRPC so gRPC wont waste time to
	// dial for unhealthy endpoint.
	// then we can reduce 3s to 1s.
	timeout := pingInterval + integration.RequestWaitTimeout

	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	wch := cli.Watch(context.Background(), "foo", clientv3.WithCreatedNotify())
	if _, ok := <-wch; !ok {
		t.Fatalf("watch failed on creation")
	}

	// endpoint can switch to eps[1] when it detects the failure of eps[0]
	cli.SetEndpoints(eps...)

	// give enough time for balancer resolution
	time.Sleep(5 * time.Second)

	clus.Members[0].Blackhole()

	if _, err = clus.Client(1).Put(context.TODO(), "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-wch:
	case <-time.After(timeout):
		t.Error("took too long to receive watch events")
	}

	clus.Members[0].Unblackhole()

	// waiting for moving eps[0] out of unhealthy, so that it can be re-pined.
	time.Sleep(ccfg.DialTimeout)

	clus.Members[1].Blackhole()

	// make sure client[0] can connect to eps[0] after remove the blackhole.
	if _, err = clus.Client(0).Get(context.TODO(), "foo"); err != nil {
		t.Fatal(err)
	}
	if _, err = clus.Client(0).Put(context.TODO(), "foo", "bar1"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-wch:
	case <-time.After(timeout):
		t.Error("took too long to receive watch events")
	}
}

func TestBalancerUnderBlackholeNoKeepAlivePut(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAlive(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Put(ctx, "foo", "bar")
		if clientv3test.IsClientTimeout(err) || clientv3test.IsServerCtxTimeout(err) || err == rpctypes.ErrTimeout {
			return errExpected
		}
		return err
	})
}

func TestBalancerUnderBlackholeNoKeepAliveDelete(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAlive(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Delete(ctx, "foo")
		if clientv3test.IsClientTimeout(err) || clientv3test.IsServerCtxTimeout(err) || err == rpctypes.ErrTimeout {
			return errExpected
		}
		return err
	})
}

func TestBalancerUnderBlackholeNoKeepAliveTxn(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAlive(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Txn(ctx).
			If(clientv3.Compare(clientv3.Version("foo"), "=", 0)).
			Then(clientv3.OpPut("foo", "bar")).
			Else(clientv3.OpPut("foo", "baz")).Commit()
		if clientv3test.IsClientTimeout(err) || clientv3test.IsServerCtxTimeout(err) || err == rpctypes.ErrTimeout {
			return errExpected
		}
		return err
	})
}

func TestBalancerUnderBlackholeNoKeepAliveLinearizableGet(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAlive(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "a")
		if clientv3test.IsClientTimeout(err) || clientv3test.IsServerCtxTimeout(err) || err == rpctypes.ErrTimeout {
			return errExpected
		}
		return err
	})
}

func TestBalancerUnderBlackholeNoKeepAliveSerializableGet(t *testing.T) {
	testBalancerUnderBlackholeNoKeepAlive(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "a", clientv3.WithSerializable())
		if clientv3test.IsClientTimeout(err) || clientv3test.IsServerCtxTimeout(err) {
			return errExpected
		}
		return err
	})
}

// testBalancerUnderBlackholeNoKeepAlive ensures that first request to blackholed endpoint
// fails due to context timeout, but succeeds on next try, with endpoint switch.
func testBalancerUnderBlackholeNoKeepAlive(t *testing.T, op func(*clientv3.Client, context.Context) error) {
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
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// wait for eps[0] to be pinned
	clientv3test.MustWaitPinReady(t, cli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	cli.SetEndpoints(eps...)

	// blackhole eps[0]
	clus.Members[0].Blackhole()

	// With round robin balancer, client will make a request to a healthy endpoint
	// within a few requests.
	// TODO: first operation can succeed
	// when gRPC supports better retry on non-delivered request
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		err = op(cli, ctx)
		cancel()
		if err == nil {
			break
		} else if err == errExpected {
			t.Logf("#%d: current error %v", i, err)
		} else {
			t.Errorf("#%d: failed with error %v", i, err)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}
