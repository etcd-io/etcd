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
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
	"go.etcd.io/etcd/integration"
	"go.etcd.io/etcd/pkg/testutil"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestBalancerUnderServerShutdownWatch expects that watch client
// switch its endpoints when the member of the pinned endpoint fails.
func TestBalancerUnderServerShutdownWatch(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:               3,
		SkipCreatingClient: true,
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr()}

	lead := clus.WaitLeader(t)

	// pin eps[lead]
	watchCli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[lead]}})
	if err != nil {
		t.Fatal(err)
	}
	defer watchCli.Close()

	// wait for eps[lead] to be pinned
	mustWaitPinReady(t, watchCli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	watchCli.SetEndpoints(eps...)

	key, val := "foo", "bar"
	wch := watchCli.Watch(context.Background(), key, clientv3.WithCreatedNotify())
	select {
	case <-wch:
	case <-time.After(integration.RequestWaitTimeout):
		t.Fatal("took too long to create watch")
	}

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		// switch to others when eps[lead] is shut down
		select {
		case ev := <-wch:
			if werr := ev.Err(); werr != nil {
				t.Error(werr)
			}
			if len(ev.Events) != 1 {
				t.Errorf("expected one event, got %+v", ev)
			}
			if !bytes.Equal(ev.Events[0].Kv.Value, []byte(val)) {
				t.Errorf("expected %q, got %+v", val, ev.Events[0].Kv)
			}
		case <-time.After(7 * time.Second):
			t.Error("took too long to receive events")
		}
	}()

	// shut down eps[lead]
	clus.Members[lead].Terminate(t)

	// writes to eps[lead+1]
	putCli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[(lead+1)%3]}})
	if err != nil {
		t.Fatal(err)
	}
	defer putCli.Close()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = putCli.Put(ctx, key, val)
		cancel()
		if err == nil {
			break
		}
		if isClientTimeout(err) || isServerCtxTimeout(err) || err == rpctypes.ErrTimeout || err == rpctypes.ErrTimeoutDueToLeaderFail {
			continue
		}
		t.Fatal(err)
	}

	select {
	case <-donec:
	case <-time.After(5 * time.Second): // enough time for balancer switch
		t.Fatal("took too long to receive events")
	}
}

func TestBalancerUnderServerShutdownPut(t *testing.T) {
	testBalancerUnderServerShutdownMutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Put(ctx, "foo", "bar")
		return err
	})
}

func TestBalancerUnderServerShutdownDelete(t *testing.T) {
	testBalancerUnderServerShutdownMutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Delete(ctx, "foo")
		return err
	})
}

func TestBalancerUnderServerShutdownTxn(t *testing.T) {
	testBalancerUnderServerShutdownMutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Txn(ctx).
			If(clientv3.Compare(clientv3.Version("foo"), "=", 0)).
			Then(clientv3.OpPut("foo", "bar")).
			Else(clientv3.OpPut("foo", "baz")).Commit()
		return err
	})
}

// testBalancerUnderServerShutdownMutable expects that when the member of
// the pinned endpoint is shut down, the balancer switches its endpoints
// and all subsequent put/delete/txn requests succeed with new endpoints.
func testBalancerUnderServerShutdownMutable(t *testing.T, op func(*clientv3.Client, context.Context) error) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:               3,
		SkipCreatingClient: true,
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr()}

	// pin eps[0]
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[0]}})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// wait for eps[0] to be pinned
	mustWaitPinReady(t, cli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	cli.SetEndpoints(eps...)

	// shut down eps[0]
	clus.Members[0].Terminate(t)

	// switched to others when eps[0] was explicitly shut down
	// and following request should succeed
	// TODO: remove this (expose client connection state?)
	time.Sleep(time.Second)

	cctx, ccancel := context.WithTimeout(context.Background(), time.Second)
	err = op(cli, cctx)
	ccancel()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBalancerUnderServerShutdownGetLinearizable(t *testing.T) {
	testBalancerUnderServerShutdownImmutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "foo")
		return err
	}, 7*time.Second) // give enough time for leader election, balancer switch
}

func TestBalancerUnderServerShutdownGetSerializable(t *testing.T) {
	testBalancerUnderServerShutdownImmutable(t, func(cli *clientv3.Client, ctx context.Context) error {
		_, err := cli.Get(ctx, "foo", clientv3.WithSerializable())
		return err
	}, 2*time.Second)
}

// testBalancerUnderServerShutdownImmutable expects that when the member of
// the pinned endpoint is shut down, the balancer switches its endpoints
// and all subsequent range requests succeed with new endpoints.
func testBalancerUnderServerShutdownImmutable(t *testing.T, op func(*clientv3.Client, context.Context) error, timeout time.Duration) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:               3,
		SkipCreatingClient: true,
	})
	defer clus.Terminate(t)

	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr(), clus.Members[2].GRPCAddr()}

	// pin eps[0]
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[0]}})
	if err != nil {
		t.Errorf("failed to create client: %v", err)
	}
	defer cli.Close()

	// wait for eps[0] to be pinned
	mustWaitPinReady(t, cli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	cli.SetEndpoints(eps...)

	// shut down eps[0]
	clus.Members[0].Terminate(t)

	// switched to others when eps[0] was explicitly shut down
	// and following request should succeed
	cctx, ccancel := context.WithTimeout(context.Background(), timeout)
	err = op(cli, cctx)
	ccancel()
	if err != nil {
		t.Errorf("failed to finish range request in time %v (timeout %v)", err, timeout)
	}
}

func TestBalancerUnderServerStopInflightLinearizableGetOnRestart(t *testing.T) {
	tt := []pinTestOpt{
		{pinLeader: true, stopPinFirst: true},
		{pinLeader: true, stopPinFirst: false},
		{pinLeader: false, stopPinFirst: true},
		{pinLeader: false, stopPinFirst: false},
	}
	for i := range tt {
		testBalancerUnderServerStopInflightRangeOnRestart(t, true, tt[i])
	}
}

func TestBalancerUnderServerStopInflightSerializableGetOnRestart(t *testing.T) {
	tt := []pinTestOpt{
		{pinLeader: true, stopPinFirst: true},
		{pinLeader: true, stopPinFirst: false},
		{pinLeader: false, stopPinFirst: true},
		{pinLeader: false, stopPinFirst: false},
	}
	for i := range tt {
		testBalancerUnderServerStopInflightRangeOnRestart(t, false, tt[i])
	}
}

type pinTestOpt struct {
	pinLeader    bool
	stopPinFirst bool
}

// testBalancerUnderServerStopInflightRangeOnRestart expects
// inflight range request reconnects on server restart.
func testBalancerUnderServerStopInflightRangeOnRestart(t *testing.T, linearizable bool, opt pinTestOpt) {
	defer testutil.AfterTest(t)

	cfg := &integration.ClusterConfig{
		Size:               2,
		SkipCreatingClient: true,
	}
	if linearizable {
		cfg.Size = 3
	}

	clus := integration.NewClusterV3(t, cfg)
	defer clus.Terminate(t)
	eps := []string{clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr()}
	if linearizable {
		eps = append(eps, clus.Members[2].GRPCAddr())
	}

	lead := clus.WaitLeader(t)

	target := lead
	if !opt.pinLeader {
		target = (target + 1) % 2
	}

	// pin eps[target]
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[target]}})
	if err != nil {
		t.Errorf("failed to create client: %v", err)
	}
	defer cli.Close()

	// wait for eps[target] to be pinned
	mustWaitPinReady(t, cli)

	// add all eps to list, so that when the original pined one fails
	// the client can switch to other available eps
	cli.SetEndpoints(eps...)

	if opt.stopPinFirst {
		clus.Members[target].Stop(t)
		// give some time for balancer switch before stopping the other
		time.Sleep(time.Second)
		clus.Members[(target+1)%2].Stop(t)
	} else {
		clus.Members[(target+1)%2].Stop(t)
		// balancer cannot pin other member since it's already stopped
		clus.Members[target].Stop(t)
	}

	// 3-second is the minimum interval between endpoint being marked
	// as unhealthy and being removed from unhealthy, so possibly
	// takes >5-second to unpin and repin an endpoint
	// TODO: decrease timeout when balancer switch rewrite
	clientTimeout := 7 * time.Second

	var gops []clientv3.OpOption
	if !linearizable {
		gops = append(gops, clientv3.WithSerializable())
	}

	donec, readyc := make(chan struct{}), make(chan struct{}, 1)
	go func() {
		defer close(donec)
		ctx, cancel := context.WithTimeout(context.TODO(), clientTimeout)
		readyc <- struct{}{}

		// TODO: The new grpc load balancer will not pin to an endpoint
		// as intended by this test. But it will round robin member within
		// two attempts.
		// Remove retry loop once the new grpc load balancer provides retry.
		for i := 0; i < 2; i++ {
			_, err = cli.Get(ctx, "abc", gops...)
			if err == nil {
				break
			}
		}
		cancel()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()

	<-readyc
	clus.Members[target].Restart(t)

	select {
	case <-time.After(clientTimeout + integration.RequestWaitTimeout):
		t.Fatalf("timed out waiting for Get [linearizable: %v, opt: %+v]", linearizable, opt)
	case <-donec:
	}
}

// e.g. due to clock drifts in server-side,
// client context times out first in server-side
// while original client-side context is not timed out yet
func isServerCtxTimeout(err error) bool {
	if err == nil {
		return false
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.DeadlineExceeded && strings.Contains(err.Error(), "context deadline exceeded")
}

// In grpc v1.11.3+ dial timeouts can error out with transport.ErrConnClosing. Previously dial timeouts
// would always error out with context.DeadlineExceeded.
func isClientTimeout(err error) bool {
	if err == nil {
		return false
	}
	if err == context.DeadlineExceeded {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.DeadlineExceeded
}

func isCanceled(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.Canceled
}

func isUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.Unavailable
}
