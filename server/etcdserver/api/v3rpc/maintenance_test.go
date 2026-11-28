// Copyright 2026 The etcd Authors
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
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/errors"
)

type fakeAlarmer struct {
	called bool
	resp   *pb.AlarmResponse
	err    error
}

func (f *fakeAlarmer) Alarms() []*pb.AlarmMember {
	return nil
}

func (f *fakeAlarmer) Alarm(_ context.Context, _ *pb.AlarmRequest) (*pb.AlarmResponse, error) {
	f.called = true
	return f.resp, f.err
}

type fakeRaftStatus struct{}

func (fakeRaftStatus) MemberID() types.ID     { return 1 }
func (fakeRaftStatus) Leader() types.ID       { return 1 }
func (fakeRaftStatus) CommittedIndex() uint64 { return 1 }
func (fakeRaftStatus) AppliedIndex() uint64   { return 1 }
func (fakeRaftStatus) Term() uint64           { return 2 }

func newTestMaintenanceServer(alarmer Alarmer) *maintenanceServer {
	return &maintenanceServer{
		a: alarmer,
		hdr: header{
			clusterID: 10,
			memberID:  1,
			sg:        fakeRaftStatus{},
			rev:       func() int64 { return 3 },
		},
	}
}

func TestMaintenanceServerAlarmInvalidAction(t *testing.T) {
	tests := []struct {
		name string
		req  *pb.AlarmRequest
	}{
		{
			name: "nil request",
			req:  nil,
		},
		{
			name: "unknown positive action",
			req:  &pb.AlarmRequest{Action: pb.AlarmRequest_AlarmAction(60)},
		},
		{
			name: "unknown negative action",
			req:  &pb.AlarmRequest{Action: pb.AlarmRequest_AlarmAction(-1)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alarmer := &fakeAlarmer{resp: &pb.AlarmResponse{}}
			ms := newTestMaintenanceServer(alarmer)

			resp, err := ms.Alarm(t.Context(), tc.req)
			require.Nil(t, resp)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.False(t, alarmer.called, "unknown alarm action should be rejected before proposing to server")
		})
	}
}

func TestMaintenanceServerAlarmValidAndDefensePaths(t *testing.T) {
	t.Run("valid action fills header", func(t *testing.T) {
		alarmer := &fakeAlarmer{resp: &pb.AlarmResponse{}}
		ms := newTestMaintenanceServer(alarmer)

		resp, err := ms.Alarm(t.Context(), &pb.AlarmRequest{Action: pb.AlarmRequest_GET})
		require.NoError(t, err)
		require.True(t, alarmer.called)
		require.NotNil(t, resp.Header)
		require.Equal(t, uint64(10), resp.Header.ClusterId)
		require.Equal(t, uint64(1), resp.Header.MemberId)
		require.Equal(t, int64(3), resp.Header.Revision)
	})

	t.Run("ErrUnknownAlarmAction maps to InvalidArgument", func(t *testing.T) {
		alarmer := &fakeAlarmer{err: errors.ErrUnknownAlarmAction}
		ms := newTestMaintenanceServer(alarmer)

		resp, err := ms.Alarm(t.Context(), &pb.AlarmRequest{Action: pb.AlarmRequest_ACTIVATE})
		require.Nil(t, resp)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("nil response without error returns Internal instead of panicking", func(t *testing.T) {
		alarmer := &fakeAlarmer{resp: nil, err: nil}
		ms := newTestMaintenanceServer(alarmer)

		resp, err := ms.Alarm(t.Context(), &pb.AlarmRequest{Action: pb.AlarmRequest_ACTIVATE})
		require.Nil(t, resp)
		require.Equal(t, codes.Internal, status.Code(err))
	})
}
