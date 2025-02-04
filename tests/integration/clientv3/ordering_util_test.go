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

package clientv3test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/ordering"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestEndpointSwitchResolvesViolation ensures
// - ErrNoGreaterRev error is returned from partitioned member when it has stale revision
// - no more error after partition recovers
func TestEndpointSwitchResolvesViolation(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	eps := []string{
		clus.Members[0].GRPCURL,
		clus.Members[1].GRPCURL,
		clus.Members[2].GRPCURL,
	}
	cfg := clientv3.Config{Endpoints: []string{clus.Members[0].GRPCURL}}
	cli, err := integration2.NewClient(t, cfg)
	require.NoError(t, err)
	defer cli.Close()

	ctx := context.TODO()

	_, err = clus.Client(0).Put(ctx, "foo", "bar")
	require.NoError(t, err)
	// ensure that the second member has current revision for key "foo"
	_, err = clus.Client(1).Get(ctx, "foo")
	require.NoError(t, err)

	// create partition between third members and the first two members
	// in order to guarantee that the third member's revision of "foo"
	// falls behind as updates to "foo" are issued to the first two members.
	clus.Members[2].InjectPartition(t, clus.Members[:2]...)
	time.Sleep(1 * time.Second) // give enough time for the operation

	// update to "foo" will not be replicated to the third member due to the partition
	_, err = clus.Client(1).Put(ctx, "foo", "buzz")
	require.NoError(t, err)

	cli.SetEndpoints(eps...)
	time.Sleep(1 * time.Second) // give enough time for the operation
	orderingKv := ordering.NewKV(cli.KV, ordering.NewOrderViolationSwitchEndpointClosure(cli))
	// set prevRev to the second member's revision of "foo" such that
	// the revision is higher than the third member's revision of "foo"
	_, err = orderingKv.Get(ctx, "foo")
	require.NoError(t, err)

	t.Logf("Reconfigure client to speak only to the 'partitioned' member")
	cli.SetEndpoints(clus.Members[2].GRPCURL)
	time.Sleep(1 * time.Second) // give enough time for the operation
	_, err = orderingKv.Get(ctx, "foo", clientv3.WithSerializable())
	if !errors.Is(err, ordering.ErrNoGreaterRev) {
		t.Fatal("While speaking to partitioned leader, we should get ErrNoGreaterRev error")
	}

	clus.Members[2].RecoverPartition(t, clus.Members[:2]...)
	time.Sleep(1 * time.Second) // give enough time for the operation
	_, err = orderingKv.Get(ctx, "foo")
	if err != nil {
		t.Fatal("After partition recovered, third member should recover and return no error")
	}
}

// TestUnresolvableOrderViolation ensures ErrNoGreaterRev error is returned when available members only have stale revisions
func TestUnresolvableOrderViolation(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 5, UseBridge: true})
	defer clus.Terminate(t)
	cfg := clientv3.Config{
		Endpoints: []string{
			clus.Members[0].GRPCURL,
			clus.Members[1].GRPCURL,
			clus.Members[2].GRPCURL,
			clus.Members[3].GRPCURL,
			clus.Members[4].GRPCURL,
		},
	}
	cli, err := integration2.NewClient(t, cfg)
	require.NoError(t, err)
	defer cli.Close()
	eps := cli.Endpoints()
	ctx := context.TODO()

	cli.SetEndpoints(clus.Members[0].GRPCURL)
	time.Sleep(1 * time.Second)
	_, err = cli.Put(ctx, "foo", "bar")
	require.NoError(t, err)

	// stop fourth member in order to force the member to have an outdated revision
	clus.Members[3].Stop(t)
	time.Sleep(1 * time.Second) // give enough time for operation
	// stop fifth member in order to force the member to have an outdated revision
	clus.Members[4].Stop(t)
	time.Sleep(1 * time.Second) // give enough time for operation
	_, err = cli.Put(ctx, "foo", "buzz")
	require.NoError(t, err)

	cli.SetEndpoints(eps...)
	time.Sleep(1 * time.Second) // give enough time for operation
	OrderingKv := ordering.NewKV(cli.KV, ordering.NewOrderViolationSwitchEndpointClosure(cli))
	// set prevRev to the first member's revision of "foo" such that
	// the revision is higher than the fourth and fifth members' revision of "foo"
	_, err = OrderingKv.Get(ctx, "foo")
	require.NoError(t, err)

	clus.Members[0].Stop(t)
	clus.Members[1].Stop(t)
	clus.Members[2].Stop(t)
	require.NoError(t, clus.Members[3].Restart(t))
	require.NoError(t, clus.Members[4].Restart(t))
	clus.Members[3].WaitStarted(t)
	cli.SetEndpoints(clus.Members[3].GRPCURL)
	time.Sleep(1 * time.Second) // give enough time for operation

	_, err = OrderingKv.Get(ctx, "foo", clientv3.WithSerializable())
	if !errors.Is(err, ordering.ErrNoGreaterRev) {
		t.Fatalf("expected %v, got %v", ordering.ErrNoGreaterRev, err)
	}
}
