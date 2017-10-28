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

package readonly

import (
	"context"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type readOnlyMaintenanceProxy struct {
	pb.MaintenanceServer
}

func NewReadOnlyMaintenanceProxy(mp pb.MaintenanceServer) pb.MaintenanceServer {
	return &readOnlyMaintenanceProxy{MaintenanceServer: mp}
}

func (mp *readOnlyMaintenanceProxy) Defragment(ctx context.Context, dr *pb.DefragmentRequest) (*pb.DefragmentResponse, error) {
	return nil, ErrReadOnly
}

func (mp *readOnlyMaintenanceProxy) Alarm(ctx context.Context, r *pb.AlarmRequest) (*pb.AlarmResponse, error) {
	if r.Action != pb.AlarmRequest_GET {
		return nil, ErrReadOnly
	}

	return mp.MaintenanceServer.Alarm(ctx, r)
}

func (mp *readOnlyMaintenanceProxy) MoveLeader(ctx context.Context, r *pb.MoveLeaderRequest) (*pb.MoveLeaderResponse, error) {
	return nil, ErrReadOnly
}
