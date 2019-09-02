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
	gContext "context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/integration"
	"go.etcd.io/etcd/pkg/testutil"
)

func TestDetectKvOrderViolation(t *testing.T) {
	var errOrderViolation = errors.New("Detected Order Violation")

	defer testutil.AfterTest(t)
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
	orderingKv := NewKV(cli.KV,
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

	defer testutil.AfterTest(t)
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
	orderingKv := NewKV(cli.KV,
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

type mockKV struct {
	clientv3.KV
	response clientv3.OpResponse
}

func (kv *mockKV) Do(ctx gContext.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return kv.response, nil
}

var rangeTests = []struct {
	prevRev  int64
	response *clientv3.GetResponse
}{
	{
		5,
		&clientv3.GetResponse{
			Header: &pb.ResponseHeader{
				Revision: 5,
			},
		},
	},
	{
		5,
		&clientv3.GetResponse{
			Header: &pb.ResponseHeader{
				Revision: 4,
			},
		},
	},
	{
		5,
		&clientv3.GetResponse{
			Header: &pb.ResponseHeader{
				Revision: 6,
			},
		},
	},
}

func TestKvOrdering(t *testing.T) {
	for i, tt := range rangeTests {
		mKV := &mockKV{clientv3.NewKVFromKVClient(nil, nil), tt.response.OpResponse()}
		kv := &kvOrdering{
			mKV,
			func(r *clientv3.GetResponse) OrderViolationFunc {
				return func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error {
					r.Header.Revision++
					return nil
				}
			}(tt.response),
			tt.prevRev,
			sync.RWMutex{},
		}
		res, err := kv.Get(nil, "mockKey")
		if err != nil {
			t.Errorf("#%d: expected response %+v, got error %+v", i, tt.response, err)
		}
		if rev := res.Header.Revision; rev < tt.prevRev {
			t.Errorf("#%d: expected revision %d, got %d", i, tt.prevRev, rev)
		}
	}
}

var txnTests = []struct {
	prevRev  int64
	response *clientv3.TxnResponse
}{
	{
		5,
		&clientv3.TxnResponse{
			Header: &pb.ResponseHeader{
				Revision: 5,
			},
		},
	},
	{
		5,
		&clientv3.TxnResponse{
			Header: &pb.ResponseHeader{
				Revision: 8,
			},
		},
	},
	{
		5,
		&clientv3.TxnResponse{
			Header: &pb.ResponseHeader{
				Revision: 4,
			},
		},
	},
}

func TestTxnOrdering(t *testing.T) {
	for i, tt := range txnTests {
		mKV := &mockKV{clientv3.NewKVFromKVClient(nil, nil), tt.response.OpResponse()}
		kv := &kvOrdering{
			mKV,
			func(r *clientv3.TxnResponse) OrderViolationFunc {
				return func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error {
					r.Header.Revision++
					return nil
				}
			}(tt.response),
			tt.prevRev,
			sync.RWMutex{},
		}
		txn := &txnOrdering{
			kv.Txn(context.Background()),
			kv,
			context.Background(),
			sync.Mutex{},
			[]clientv3.Cmp{},
			[]clientv3.Op{},
			[]clientv3.Op{},
		}
		res, err := txn.Commit()
		if err != nil {
			t.Errorf("#%d: expected response %+v, got error %+v", i, tt.response, err)
		}
		if rev := res.Header.Revision; rev < tt.prevRev {
			t.Errorf("#%d: expected revision %d, got %d", i, tt.prevRev, rev)
		}
	}
}
