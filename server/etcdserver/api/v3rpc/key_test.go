// Copyright 2021 The etcd Authors
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

package v3rpc

import (
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"

	"github.com/stretchr/testify/assert"
)

func TestCheckRangeRequest(t *testing.T) {
	rangeReqs := []struct {
		sortOrder     pb.RangeRequest_SortOrder
		sortTarget    pb.RangeRequest_SortTarget
		expectedError error
	}{
		{
			sortOrder:     pb.RangeRequest_ASCEND,
			sortTarget:    pb.RangeRequest_CREATE,
			expectedError: nil,
		},
		{
			sortOrder:     pb.RangeRequest_ASCEND,
			sortTarget:    100,
			expectedError: rpctypes.ErrGRPCInvalidSortOption,
		},
		{
			sortOrder:     200,
			sortTarget:    pb.RangeRequest_MOD,
			expectedError: rpctypes.ErrGRPCInvalidSortOption,
		},
	}

	for _, req := range rangeReqs {
		rangeReq := pb.RangeRequest{
			Key:        []byte{1, 2, 3},
			SortOrder:  req.sortOrder,
			SortTarget: req.sortTarget,
		}

		actualRet := checkRangeRequest(&rangeReq)
		if getError(actualRet) != getError(req.expectedError) {
			t.Errorf("expected sortOrder (%d) and sortTarget (%d) to be %q, but got %q",
				req.sortOrder, req.sortTarget, getError(req.expectedError), getError(actualRet))
		}
	}
}

func getError(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

func TestCheckIntervals(t *testing.T) {
	tests := []struct {
		name      string
		requestOp []*pb.RequestOp
		err       error
	}{
		// check ok
		{
			name:      "No duplicate key is ok",
			requestOp: []*pb.RequestOp{put_op, put_op1},
			err:       nil,
		},
		{
			name:      "Nested no duplicate key is ok",
			requestOp: []*pb.RequestOp{txn_op_put_success, txn_op_put_success1},
			err:       nil,
		},
		{
			name: "Overlap in different branch is ok",
			requestOp: []*pb.RequestOp{
				{
					Request: &pb.RequestOp_RequestTxn{
						RequestTxn: &pb.TxnRequest{
							Success: []*pb.RequestOp{put_op},
							Failure: []*pb.RequestOp{del_op},
						},
					},
				},
			},
			err: nil,
		},
		// check err
		{
			name:      "Duplicate key should fail",
			requestOp: []*pb.RequestOp{put_op, put_op},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Nested duplicate key should fail",
			requestOp: []*pb.RequestOp{txn_op_put_success, txn_op_put_success},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Overlap put and delete should fail",
			requestOp: []*pb.RequestOp{put_op, del_op},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Overlap put-in-txn and delete should fail",
			requestOp: []*pb.RequestOp{txn_op_put_success, del_op},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Overlap put and delete-in-txn should fail",
			requestOp: []*pb.RequestOp{put_op, txn_op_del_success},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Nested overlap put and delete should fail, case 0",
			requestOp: []*pb.RequestOp{txn_op_put_success, txn_op_del_success},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Nested overlap put and delete should fail, case 1",
			requestOp: []*pb.RequestOp{txn_op_put_success, txn_op_del_failure},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Nested overlap put and delete should fail, case 2",
			requestOp: []*pb.RequestOp{txn_op_put_failure, txn_op_del_success},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
		{
			name:      "Nested overlap put and delete should fail, case 3 (with the order swapped)",
			requestOp: []*pb.RequestOp{txn_op_del_success, txn_op_put_success},
			err:       rpctypes.ErrGRPCDuplicateKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := checkIntervals(tt.requestOp)
			assert.Equal(t, tt.err, err)
		})
	}

}

// TestCheckIntervals variables setup.
var (
	put_op = &pb.RequestOp{
		Request: &pb.RequestOp_RequestPut{
			RequestPut: &pb.PutRequest{
				Key: []byte("foo1"),
			},
		},
	}
	put_op1 = &pb.RequestOp{
		Request: &pb.RequestOp_RequestPut{
			RequestPut: &pb.PutRequest{
				Key: []byte("foo2"),
			},
		},
	}
	// overlap with `put_op`
	del_op = &pb.RequestOp{
		Request: &pb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: &pb.DeleteRangeRequest{
				Key:      []byte("foo0"),
				RangeEnd: []byte("foo3"),
			},
		},
	}
	// does not overlap with `put_op`
	del_op1 = &pb.RequestOp{
		Request: &pb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: &pb.DeleteRangeRequest{
				Key:      []byte("foo2"),
				RangeEnd: []byte("foo3"),
			},
		},
	}
	txn_op_put_success = &pb.RequestOp{
		Request: &pb.RequestOp_RequestTxn{
			RequestTxn: &pb.TxnRequest{
				Success: []*pb.RequestOp{put_op},
			},
		},
	}
	txn_op_put_success1 = &pb.RequestOp{
		Request: &pb.RequestOp_RequestTxn{
			RequestTxn: &pb.TxnRequest{
				Success: []*pb.RequestOp{put_op1},
			},
		},
	}
	txn_op_put_failure = &pb.RequestOp{
		Request: &pb.RequestOp_RequestTxn{
			RequestTxn: &pb.TxnRequest{
				Success: []*pb.RequestOp{put_op},
			},
		},
	}
	txn_op_del_success = &pb.RequestOp{
		Request: &pb.RequestOp_RequestTxn{
			RequestTxn: &pb.TxnRequest{
				Success: []*pb.RequestOp{del_op},
			},
		},
	}
	txn_op_del_failure = &pb.RequestOp{
		Request: &pb.RequestOp_RequestTxn{
			RequestTxn: &pb.TxnRequest{
				Failure: []*pb.RequestOp{del_op},
			},
		},
	}
)
