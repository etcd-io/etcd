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

package ordering

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestEndpointSwitchResolvesViolation(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	eps := []string{
		clus.Members[0].GRPCAddr(),
		clus.Members[1].GRPCAddr(),
		clus.Members[2].GRPCAddr(),
	}
	cfg := clientv3.Config{Endpoints: []string{clus.Members[0].GRPCAddr()}}
	cli, err := clientv3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.TODO()

	if _, err = clus.Client(0).Put(ctx, "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	// ensure that the second member has current revision for key "foo"
	if _, err = clus.Client(1).Get(ctx, "foo"); err != nil {
		t.Fatal(err)
	}

	// create partition between third members and the first two members
	// in order to guarantee that the third member's revision of "foo"
	// falls behind as updates to "foo" are issued to the first two members.
	clus.Members[2].InjectPartition(t, clus.Members[:2]...)
	time.Sleep(1 * time.Second) // give enough time for the operation

	// update to "foo" will not be replicated to the third member due to the partition
	if _, err = clus.Client(1).Put(ctx, "foo", "buzz"); err != nil {
		t.Fatal(err)
	}

	// reset client endpoints to all members such that the copy of cli sent to
	// NewOrderViolationSwitchEndpointClosure will be able to
	// access the full list of endpoints.
	cli.SetEndpoints(eps...)
	OrderingKv := NewKV(cli.KV, NewOrderViolationSwitchEndpointClosure(*cli))
	// set prevRev to the second member's revision of "foo" such that
	// the revision is higher than the third member's revision of "foo"
	_, err = OrderingKv.Get(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}

	cli.SetEndpoints(clus.Members[2].GRPCAddr())
	time.Sleep(1 * time.Second) // give enough time for operation
	_, err = OrderingKv.Get(ctx, "foo", clientv3.WithSerializable())
	if err != nil {
		t.Fatalf("failed to resolve order violation %v", err)
	}
}

func TestUnresolvableOrderViolation(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 5, SkipCreatingClient: true})
	defer clus.Terminate(t)
	cfg := clientv3.Config{
		Endpoints: []string{
			clus.Members[0].GRPCAddr(),
			clus.Members[1].GRPCAddr(),
			clus.Members[2].GRPCAddr(),
			clus.Members[3].GRPCAddr(),
			clus.Members[4].GRPCAddr(),
		},
	}
	cli, err := clientv3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	eps := cli.Endpoints()
	ctx := context.TODO()

	cli.SetEndpoints(clus.Members[0].GRPCAddr())
	time.Sleep(1 * time.Second)
	_, err = cli.Put(ctx, "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}

	// stop fourth member in order to force the member to have an outdated revision
	clus.Members[3].Stop(t)
	time.Sleep(1 * time.Second) // give enough time for operation
	// stop fifth member in order to force the member to have an outdated revision
	clus.Members[4].Stop(t)
	time.Sleep(1 * time.Second) // give enough time for operation
	_, err = cli.Put(ctx, "foo", "buzz")
	if err != nil {
		t.Fatal(err)
	}

	// reset client endpoints to all members such that the copy of cli sent to
	// NewOrderViolationSwitchEndpointClosure will be able to
	// access the full list of endpoints.
	cli.SetEndpoints(eps...)
	OrderingKv := NewKV(cli.KV, NewOrderViolationSwitchEndpointClosure(*cli))
	// set prevRev to the first member's revision of "foo" such that
	// the revision is higher than the fourth and fifth members' revision of "foo"
	_, err = OrderingKv.Get(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}

	clus.Members[0].Stop(t)
	clus.Members[1].Stop(t)
	clus.Members[2].Stop(t)
	clus.Members[3].Restart(t)
	clus.Members[4].Restart(t)
	cli.SetEndpoints(clus.Members[3].GRPCAddr())
	time.Sleep(1 * time.Second) // give enough time for operation

	_, err = OrderingKv.Get(ctx, "foo", clientv3.WithSerializable())
	if err != ErrNoGreaterRev {
		t.Fatalf("expected %v, got %v", ErrNoGreaterRev, err)
	}
}
