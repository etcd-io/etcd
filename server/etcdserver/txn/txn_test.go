// Copyright 2022 The etcd Authors
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

package txn

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"

	"github.com/stretchr/testify/assert"
)

func TestReadonlyTxnError(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup cancelled context
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	// put some data to prevent early termination in rangeKeys
	// we are expecting failure on cancelled context check
	s.Put([]byte("foo"), []byte("bar"), lease.NoLease)

	txn := &pb.TxnRequest{
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: &pb.RangeRequest{
						Key: []byte("foo"),
					},
				},
			},
		},
	}

	_, _, err := Txn(ctx, zaptest.NewLogger(t), txn, false, s, &lease.FakeLessor{})
	if err == nil || !strings.Contains(err.Error(), "applyTxn: failed Range: rangeKeys: context cancelled: context canceled") {
		t.Fatalf("Expected context canceled error, got %v", err)
	}
}

func TestWriteTxnPanic(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup cancelled context
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	// write txn that puts some data and then fails in range due to cancelled context
	txn := &pb.TxnRequest{
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   []byte("foo"),
						Value: []byte("bar"),
					},
				},
			},
			{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: &pb.RangeRequest{
						Key: []byte("foo"),
					},
				},
			},
		},
	}

	assert.Panics(t, func() { Txn(ctx, zaptest.NewLogger(t), txn, false, s, &lease.FakeLessor{}) }, "Expected panic in Txn with writes")
}

func TestCheckTxnAuth(t *testing.T) {
	lg := zaptest.NewLogger(t)

	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)

	simpleTokenTTLDefault := 300 * time.Second
	tokenTypeSimple := "simple"
	dummyIndexWaiter := func(index uint64) <-chan struct{} {
		ch := make(chan struct{}, 1)
		go func() {
			ch <- struct{}{}
		}()
		return ch
	}

	tp, _ := auth.NewTokenProvider(zaptest.NewLogger(t), tokenTypeSimple, dummyIndexWaiter, simpleTokenTTLDefault)

	as := auth.NewAuthStore(lg, schema.NewAuthBackend(lg, be), tp, 4)

	// create "root" user and "foo" user with limited range
	if _, err := as.RoleAdd(&pb.AuthRoleAddRequest{Name: "root"}); err != nil {
		t.Fatal(err)
	}
	if _, err := as.RoleAdd(&pb.AuthRoleAddRequest{Name: "rw"}); err != nil {
		t.Fatal(err)
	}
	if _, err := as.RoleGrantPermission(&pb.AuthRoleGrantPermissionRequest{
		Name: "rw",
		Perm: &authpb.Permission{
			PermType: authpb.READWRITE,
			Key:      []byte("foo"),
			RangeEnd: []byte("zoo"),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := as.UserAdd(&pb.AuthUserAddRequest{Name: "root", Password: "foo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := as.UserAdd(&pb.AuthUserAddRequest{Name: "foo", Password: "foo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "root", Role: "root"}); err != nil {
		t.Fatal(err)
	}
	if _, err := as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "foo", Role: "rw"}); err != nil {
		t.Fatal(err)
	}

	if err := as.AuthEnable(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		txnRequest *pb.TxnRequest
		err        error
	}{
		{
			name: "Out of range compare is unauthorized",
			txnRequest: &pb.TxnRequest{
				Compare: []*pb.Compare{
					{
						Key:      []byte("boo"),
						RangeEnd: []byte("zoo"),
					},
				},
				Success: []*pb.RequestOp{},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "In range compare is authorized",
			txnRequest: &pb.TxnRequest{
				Compare: []*pb.Compare{
					{
						Key:      []byte("foo"),
						RangeEnd: []byte("zoo"),
					},
				},
				Success: []*pb.RequestOp{},
			},
			err: nil,
		},
		{
			name: "Nil request range is always authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: nil,
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "Range request in range is authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "Range request out of range success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key:      []byte("boo"),
								RangeEnd: []byte("zoo"),
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Range request out of range failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key:      []byte("boo"),
								RangeEnd: []byte("zoo"),
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Nil Put request is always authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: nil,
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "Put request in range in authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte("foo"),
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte("foo"),
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "Put request out of range success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte("boo"),
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte("foo"),
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Put request out of range failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte("foo"),
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte("boo"),
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Nil delete request is authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: nil,
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "Delete range request in range is authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "Delete range request out of range success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("boo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Delete range request out of range failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("boo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Delete range request out of range and PrevKv false success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("boo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   false,
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Delete range request out of range and PrevKv false failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("foo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   true,
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key:      []byte("boo"),
								RangeEnd: []byte("zoo"),
								PrevKv:   false,
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckTxnAuth(as, &auth.AuthInfo{Username: "foo", Revision: 8}, tt.txnRequest)
			if err != tt.err {
				t.Errorf("expected error to be: %v; got: %v", tt.err, err)
			}
		})
	}
}
