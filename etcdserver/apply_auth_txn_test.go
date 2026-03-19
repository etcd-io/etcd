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

package etcdserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/auth"
	"go.etcd.io/etcd/auth/authpb"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	betesting "go.etcd.io/etcd/mvcc/backend"
)

// checkTxnAuth variables setup.
var (
	inRangeCompare = &pb.Compare{
		Key:      []byte("foo"),
		RangeEnd: []byte("zoo"),
	}
	outOfRangeCompare = &pb.Compare{
		Key:      []byte("boo"),
		RangeEnd: []byte("zoo"),
	}
	nilRequestPut = &pb.RequestOp{
		Request: &pb.RequestOp_RequestPut{
			RequestPut: nil,
		},
	}
	inRangeRequestPut = &pb.RequestOp{
		Request: &pb.RequestOp_RequestPut{
			RequestPut: &pb.PutRequest{
				Key: []byte("foo"),
			},
		},
	}
	outOfRangeRequestPut = &pb.RequestOp{
		Request: &pb.RequestOp_RequestPut{
			RequestPut: &pb.PutRequest{
				Key: []byte("boo"),
			},
		},
	}
	nilRequestRange = &pb.RequestOp{
		Request: &pb.RequestOp_RequestRange{
			RequestRange: nil,
		},
	}
	inRangeRequestRange = &pb.RequestOp{
		Request: &pb.RequestOp_RequestRange{
			RequestRange: &pb.RangeRequest{
				Key:      []byte("foo"),
				RangeEnd: []byte("zoo"),
			},
		},
	}
	outOfRangeRequestRange = &pb.RequestOp{
		Request: &pb.RequestOp_RequestRange{
			RequestRange: &pb.RangeRequest{
				Key:      []byte("boo"),
				RangeEnd: []byte("zoo"),
			},
		},
	}
	nilRequestDeleteRange = &pb.RequestOp{
		Request: &pb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: nil,
		},
	}
	inRangeRequestDeleteRange = &pb.RequestOp{
		Request: &pb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: &pb.DeleteRangeRequest{
				Key:      []byte("foo"),
				RangeEnd: []byte("zoo"),
				PrevKv:   true,
			},
		},
	}
	outOfRangeRequestDeleteRange = &pb.RequestOp{
		Request: &pb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: &pb.DeleteRangeRequest{
				Key:      []byte("boo"),
				RangeEnd: []byte("zoo"),
				PrevKv:   true,
			},
		},
	}
	outOfRangeRequestDeleteRangeKvFalse = &pb.RequestOp{
		Request: &pb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: &pb.DeleteRangeRequest{
				Key:      []byte("boo"),
				RangeEnd: []byte("zoo"),
				PrevKv:   false,
			},
		},
	}
)

func setupAuth(t *testing.T, be betesting.Backend) auth.AuthStore {
	lg := zaptest.NewLogger(t)

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

	as := auth.NewAuthStore(lg, be, tp, 4)

	// create "root" user and "foo" user with limited range
	_, err := as.RoleAdd(&pb.AuthRoleAddRequest{Name: "root"})
	require.NoError(t, err)

	_, err = as.RoleAdd(&pb.AuthRoleAddRequest{Name: "rw"})
	require.NoError(t, err)

	_, err = as.RoleGrantPermission(&pb.AuthRoleGrantPermissionRequest{
		Name: "rw",
		Perm: &authpb.Permission{
			PermType: authpb.READWRITE,
			Key:      []byte("foo"),
			RangeEnd: []byte("zoo"),
		},
	})
	require.NoError(t, err)

	_, err = as.UserAdd(&pb.AuthUserAddRequest{Name: "root", Password: "foo"})
	require.NoError(t, err)

	_, err = as.UserAdd(&pb.AuthUserAddRequest{Name: "foo", Password: "foo"})
	require.NoError(t, err)

	_, err = as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "root", Role: "root"})
	require.NoError(t, err)

	_, err = as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "foo", Role: "rw"})
	require.NoError(t, err)

	err = as.AuthEnable()
	require.NoError(t, err)

	return as
}

func TestCheckTxnAuth(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend()
	defer be.Close()
	as := setupAuth(t, be)

	tests := []struct {
		name       string
		txnRequest *pb.TxnRequest
		err        error
	}{
		{
			name: "Out of range compare is unauthorized",
			txnRequest: &pb.TxnRequest{
				Compare: []*pb.Compare{outOfRangeCompare},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "In range compare is authorized",
			txnRequest: &pb.TxnRequest{
				Compare: []*pb.Compare{inRangeCompare},
			},
			err: nil,
		},
		{
			name: "Nil request range is always authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{nilRequestRange},
			},
			err: nil,
		},
		{
			name: "Range request in range is authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{inRangeRequestRange},
				Failure: []*pb.RequestOp{inRangeRequestRange},
			},
			err: nil,
		},
		{
			name: "Range request out of range success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{outOfRangeRequestRange},
				Failure: []*pb.RequestOp{inRangeRequestRange},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Range request out of range failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{inRangeRequestRange},
				Failure: []*pb.RequestOp{outOfRangeRequestRange},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Nil Put request is always authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{nilRequestPut},
			},
			err: nil,
		},
		{
			name: "Put request in range in authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{inRangeRequestPut},
				Failure: []*pb.RequestOp{inRangeRequestPut},
			},
			err: nil,
		},
		{
			name: "Put request out of range success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{outOfRangeRequestPut},
				Failure: []*pb.RequestOp{inRangeRequestPut},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Put request out of range failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{inRangeRequestPut},
				Failure: []*pb.RequestOp{outOfRangeRequestPut},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Nil delete request is authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{nilRequestDeleteRange},
			},
			err: nil,
		},
		{
			name: "Delete range request in range is authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{inRangeRequestDeleteRange},
				Failure: []*pb.RequestOp{inRangeRequestDeleteRange},
			},
			err: nil,
		},
		{
			name: "Delete range request out of range success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{outOfRangeRequestDeleteRange},
				Failure: []*pb.RequestOp{inRangeRequestDeleteRange},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Delete range request out of range failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{inRangeRequestDeleteRange},
				Failure: []*pb.RequestOp{outOfRangeRequestDeleteRange},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Delete range request out of range and PrevKv false success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{outOfRangeRequestDeleteRangeKvFalse},
				Failure: []*pb.RequestOp{inRangeRequestDeleteRange},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Delete range request out of range and PrevKv false failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{inRangeRequestDeleteRange},
				Failure: []*pb.RequestOp{outOfRangeRequestDeleteRangeKvFalse},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Nested txn request in range is authorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestTxn{
							RequestTxn: &pb.TxnRequest{
								Success: []*pb.RequestOp{inRangeRequestRange, inRangeRequestPut},
								Failure: []*pb.RequestOp{inRangeRequestDeleteRange},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name: "Nested txn request out of range success case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestTxn{
							RequestTxn: &pb.TxnRequest{
								Success: []*pb.RequestOp{outOfRangeRequestRange},
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Nested txn request out of range failure case is unauthorized",
			txnRequest: &pb.TxnRequest{
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestTxn{
							RequestTxn: &pb.TxnRequest{
								Failure: []*pb.RequestOp{outOfRangeRequestPut},
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Nested txn request out of range delete is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestTxn{
							RequestTxn: &pb.TxnRequest{
								Success: []*pb.RequestOp{outOfRangeRequestDeleteRange},
							},
						},
					},
				},
			},
			err: auth.ErrPermissionDenied,
		},
		{
			name: "Two level nested txn request out of range delete is unauthorized",
			txnRequest: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestTxn{
							RequestTxn: &pb.TxnRequest{
								Failure: []*pb.RequestOp{
									{
										Request: &pb.RequestOp_RequestTxn{
											RequestTxn: &pb.TxnRequest{
												Success: []*pb.RequestOp{outOfRangeRequestDeleteRange},
											},
										},
									},
								},
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
			err := checkTxnAuth(as, &auth.AuthInfo{Username: "foo", Revision: 8}, tt.txnRequest)
			assert.Equal(t, tt.err, err)
		})
	}
}
