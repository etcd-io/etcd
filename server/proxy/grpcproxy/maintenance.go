// Copyright 2016 The etcd Authors
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

package grpcproxy

import (
	"context"
	"errors"
	"io"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type maintenanceProxy struct {
	maintenanceClient pb.MaintenanceClient
}

func NewMaintenanceProxy(c *clientv3.Client) pb.MaintenanceServer {
	return &maintenanceProxy{
		maintenanceClient: pb.NewMaintenanceClient(c.ActiveConnection()),
	}
}

func (mp *maintenanceProxy) Defragment(ctx context.Context, dr *pb.DefragmentRequest) (*pb.DefragmentResponse, error) {
	return mp.maintenanceClient.Defragment(ctx, dr)
}

func (mp *maintenanceProxy) Snapshot(sr *pb.SnapshotRequest, stream pb.Maintenance_SnapshotServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	ctx = withClientAuthToken(ctx, stream.Context())

	sc, err := mp.maintenanceClient.Snapshot(ctx, sr)
	if err != nil {
		return err
	}

	for {
		rr, err := sc.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		err = stream.Send(rr)
		if err != nil {
			return err
		}
	}
}

func (mp *maintenanceProxy) Hash(ctx context.Context, r *pb.HashRequest) (*pb.HashResponse, error) {
	return mp.maintenanceClient.Hash(ctx, r)
}

func (mp *maintenanceProxy) HashKV(ctx context.Context, r *pb.HashKVRequest) (*pb.HashKVResponse, error) {
	return mp.maintenanceClient.HashKV(ctx, r)
}

func (mp *maintenanceProxy) Alarm(ctx context.Context, r *pb.AlarmRequest) (*pb.AlarmResponse, error) {
	return mp.maintenanceClient.Alarm(ctx, r)
}

func (mp *maintenanceProxy) Status(ctx context.Context, r *pb.StatusRequest) (*pb.StatusResponse, error) {
	return mp.maintenanceClient.Status(ctx, r)
}

func (mp *maintenanceProxy) MoveLeader(ctx context.Context, r *pb.MoveLeaderRequest) (*pb.MoveLeaderResponse, error) {
	return mp.maintenanceClient.MoveLeader(ctx, r)
}

func (mp *maintenanceProxy) Downgrade(ctx context.Context, r *pb.DowngradeRequest) (*pb.DowngradeResponse, error) {
	return mp.maintenanceClient.Downgrade(ctx, r)
}
