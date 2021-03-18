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

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/ordering"
	"go.etcd.io/etcd/tests/v3/integration"
)

func TestDetectKvOrderViolation(t *testing.T) {
	var errOrderViolation = errors.New("Detected Order Violation")

	integration.BeforeTest(t)
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cfg := clientv3.Config{
		Endpoints: []string{
			clus.Members[0].GRPCAddr(),
			clus.Members[1].GRPCAddr(),
			clus.Members[2].GRPCAddr(),
		},
	}
	cli, err := clientv3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	ctx := context.TODO()

	if _, err = clus.Client(0).Put(ctx, "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	// ensure that the second member has the current revision for the key foo
	if _, err = clus.Client(1).Get(ctx, "foo"); err != nil {
		t.Fatal(err)
	}

	// stop third member in order to force the member to have an outdated revision
	clus.Members[2].Stop(t)
	time.Sleep(1 * time.Second) // give enough time for operation
	_, err = cli.Put(ctx, "foo", "buzz")
	if err != nil {
		t.Fatal(err)
	}

	// perform get request against the first member, in order to
	// set up kvOrdering to expect "foo" revisions greater than that of
	// the third member.
	orderingKv := ordering.NewKV(cli.KV,
		func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error {
			return errOrderViolation
		})
	_, err = orderingKv.Get(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}

	// ensure that only the third member is queried during requests
	clus.Members[0].Stop(t)
	clus.Members[1].Stop(t)
	clus.Members[2].Restart(t)
	// force OrderingKv to query the third member
	cli.SetEndpoints(clus.Members[2].GRPCAddr())
	time.Sleep(2 * time.Second) // FIXME: Figure out how pause SetEndpoints sufficiently that this is not needed

	_, err = orderingKv.Get(ctx, "foo", clientv3.WithSerializable())
	if err != errOrderViolation {
		t.Fatalf("expected %v, got %v", errOrderViolation, err)
	}
}

func TestDetectTxnOrderViolation(t *testing.T) {
	var errOrderViolation = errors.New("Detected Order Violation")

	integration.BeforeTest(t)
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cfg := clientv3.Config{
		Endpoints: []string{
			clus.Members[0].GRPCAddr(),
			clus.Members[1].GRPCAddr(),
			clus.Members[2].GRPCAddr(),
		},
	}
	cli, err := clientv3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	ctx := context.TODO()

	if _, err = clus.Client(0).Put(ctx, "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	// ensure that the second member has the current revision for the key foo
	if _, err = clus.Client(1).Get(ctx, "foo"); err != nil {
		t.Fatal(err)
	}

	// stop third member in order to force the member to have an outdated revision
	clus.Members[2].Stop(t)
	time.Sleep(1 * time.Second) // give enough time for operation
	if _, err = clus.Client(1).Put(ctx, "foo", "buzz"); err != nil {
		t.Fatal(err)
	}

	// perform get request against the first member, in order to
	// set up kvOrdering to expect "foo" revisions greater than that of
	// the third member.
	orderingKv := ordering.NewKV(cli.KV,
		func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error {
			return errOrderViolation
		})
	orderingTxn := orderingKv.Txn(ctx)
	_, err = orderingTxn.If(
		clientv3.Compare(clientv3.Value("b"), ">", "a"),
	).Then(
		clientv3.OpGet("foo"),
	).Commit()
	if err != nil {
		t.Fatal(err)
	}

	// ensure that only the third member is queried during requests
	clus.Members[0].Stop(t)
	clus.Members[1].Stop(t)
	clus.Members[2].Restart(t)
	// force OrderingKv to query the third member
	cli.SetEndpoints(clus.Members[2].GRPCAddr())
	time.Sleep(2 * time.Second) // FIXME: Figure out how pause SetEndpoints sufficiently that this is not needed
	_, err = orderingKv.Get(ctx, "foo", clientv3.WithSerializable())
	if err != errOrderViolation {
		t.Fatalf("expected %v, got %v", errOrderViolation, err)
	}
	orderingTxn = orderingKv.Txn(ctx)
	_, err = orderingTxn.If(
		clientv3.Compare(clientv3.Value("b"), ">", "a"),
	).Then(
		clientv3.OpGet("foo", clientv3.WithSerializable()),
	).Commit()
	if err != errOrderViolation {
		t.Fatalf("expected %v, got %v", errOrderViolation, err)
	}
}
