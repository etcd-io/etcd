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

package v3rpc

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func TestIsRPCSupportedForLearner(t *testing.T) {
	tt := []struct {
		req  any
		want bool
	}{
		{req: &pb.StatusRequest{}, want: true},
		{req: &pb.DefragmentRequest{}, want: true},
		{req: &pb.RangeRequest{Serializable: true}, want: true},
		{req: &pb.RangeRequest{Serializable: false}, want: false},
		{req: &pb.PutRequest{}, want: false},
		{req: &pb.DeleteRangeRequest{}, want: false},
		{req: &pb.AlarmRequest{}, want: false},
	}
	for i, tc := range tt {
		got := isRPCSupportedForLearner(tc.req)
		if got != tc.want {
			t.Errorf("#%d: isRPCSupportedForLearner(%T) = %v, want %v", i, tc.req, got, tc.want)
		}
	}
}

func TestGRPCError(t *testing.T) {
	tt := []struct {
		err error
		exp error
	}{
		{err: mvcc.ErrCompacted, exp: rpctypes.ErrGRPCCompacted},
		{err: mvcc.ErrFutureRev, exp: rpctypes.ErrGRPCFutureRev},
		{err: context.Canceled, exp: context.Canceled},
		{err: context.DeadlineExceeded, exp: context.DeadlineExceeded},
		{err: errors.New("foo"), exp: status.Error(codes.Unknown, "foo")},
	}
	for i := range tt {
		if err := togRPCError(tt[i].err); !errors.Is(err, tt[i].exp) {
			if _, ok := status.FromError(err); ok {
				if err.Error() == tt[i].exp.Error() {
					continue
				}
			}
			t.Errorf("#%d: got %v, expected %v", i, err, tt[i].exp)
		}
	}
}
