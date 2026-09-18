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
	del := func(key, rangeEnd string) *pb.RequestOp {
		return &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{
			RequestDeleteRange: &pb.DeleteRangeRequest{Key: []byte(key), RangeEnd: []byte(rangeEnd)},
		}}
	}
	put := func(key string) *pb.RequestOp {
		return &pb.RequestOp{Request: &pb.RequestOp_RequestPut{
			RequestPut: &pb.PutRequest{Key: []byte(key), Value: []byte("v")},
		}}
	}

	tests := []struct {
		name         string
		reqs         []*pb.RequestOp
		wantConflict bool
	}{
		{"from-key delete overlaps put", []*pb.RequestOp{del("a", "\x00"), put("b")}, true},
		{"from-key delete before put", []*pb.RequestOp{del("m", "\x00"), put("a")}, false},
		{"explicit range overlaps put", []*pb.RequestOp{del("a", "c"), put("b")}, true},
		{"explicit range excludes end", []*pb.RequestOp{del("a", "c"), put("c")}, false},
		{"point delete overlaps put", []*pb.RequestOp{del("a", ""), put("a")}, true},
		{"point delete distinct put", []*pb.RequestOp{del("a", ""), put("b")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := checkIntervals(tt.reqs)
			if gotConflict := err != nil; gotConflict != tt.wantConflict {
				t.Errorf("checkIntervals conflict = %v, want %v (err=%v)", gotConflict, tt.wantConflict, err)
			}
		})
	}
}
