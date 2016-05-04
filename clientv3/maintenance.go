// Copyright 2016 CoreOS, Inc.
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

package clientv3

import (
	"io"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type (
	DefragmentResponse pb.DefragmentResponse
	AlarmResponse      pb.AlarmResponse
	AlarmMember        pb.AlarmMember
	StatusResponse     pb.StatusResponse
)

type Maintenance interface {
	// AlarmList gets all active alarms.
	AlarmList(ctx context.Context) (*AlarmResponse, error)

	// AlarmDisarm disarms a given alarm.
	AlarmDisarm(ctx context.Context, m *AlarmMember) (*AlarmResponse, error)

	// Defragment defragments storage backend of the etcd member with given endpoint.
	// Defragment is only needed when deleting a large number of keys and want to reclaim
	// the resources.
	// Defragment is an expensive operation. User should avoid defragmenting multiple members
	// at the same time.
	// To defragment multiple members in the cluster, user need to call defragment multiple
	// times with different endpoints.
	Defragment(ctx context.Context, endpoint string) (*DefragmentResponse, error)

	// Status gets the status of the endpoint.
	Status(ctx context.Context, endpoint string) (*StatusResponse, error)

	// Snapshot provides a reader for a snapshot of a backend.
	Snapshot(ctx context.Context) (io.ReadCloser, error)
}

type maintenance struct {
	c *Client

	rc     *remoteClient
	remote pb.MaintenanceClient
}

func NewMaintenance(c *Client) Maintenance {
	ret := &maintenance{c: c}
	f := func(conn *grpc.ClientConn) { ret.remote = pb.NewMaintenanceClient(conn) }
	ret.rc = newRemoteClient(c, f)
	return ret
}

func (m *maintenance) AlarmList(ctx context.Context) (*AlarmResponse, error) {
	req := &pb.AlarmRequest{
		Action:   pb.AlarmRequest_GET,
		MemberID: 0,                 // all
		Alarm:    pb.AlarmType_NONE, // all
	}
	for {
		resp, err := m.getRemote().Alarm(ctx, req)
		if err == nil {
			return (*AlarmResponse)(resp), nil
		}
		if isHaltErr(ctx, err) {
			return nil, rpctypes.Error(err)
		}
		if err = m.rc.reconnectWait(ctx, err); err != nil {
			return nil, err
		}
	}
}

func (m *maintenance) AlarmDisarm(ctx context.Context, am *AlarmMember) (*AlarmResponse, error) {
	req := &pb.AlarmRequest{
		Action:   pb.AlarmRequest_DEACTIVATE,
		MemberID: am.MemberID,
		Alarm:    am.Alarm,
	}

	if req.MemberID == 0 && req.Alarm == pb.AlarmType_NONE {
		ar, err := m.AlarmList(ctx)
		if err != nil {
			return nil, rpctypes.Error(err)
		}
		ret := AlarmResponse{}
		for _, am := range ar.Alarms {
			dresp, derr := m.AlarmDisarm(ctx, (*AlarmMember)(am))
			if derr != nil {
				return nil, rpctypes.Error(derr)
			}
			ret.Alarms = append(ret.Alarms, dresp.Alarms...)
		}
		return &ret, nil
	}

	resp, err := m.getRemote().Alarm(ctx, req)
	if err == nil {
		return (*AlarmResponse)(resp), nil
	}
	if !isHaltErr(ctx, err) {
		m.rc.reconnect(err)
	}
	return nil, rpctypes.Error(err)
}

func (m *maintenance) Defragment(ctx context.Context, endpoint string) (*DefragmentResponse, error) {
	conn, err := m.c.Dial(endpoint)
	if err != nil {
		return nil, rpctypes.Error(err)
	}
	remote := pb.NewMaintenanceClient(conn)
	resp, err := remote.Defragment(ctx, &pb.DefragmentRequest{})
	if err != nil {
		return nil, rpctypes.Error(err)
	}
	return (*DefragmentResponse)(resp), nil
}

func (m *maintenance) Status(ctx context.Context, endpoint string) (*StatusResponse, error) {
	conn, err := m.c.Dial(endpoint)
	if err != nil {
		return nil, rpctypes.Error(err)
	}
	remote := pb.NewMaintenanceClient(conn)
	resp, err := remote.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, rpctypes.Error(err)
	}
	return (*StatusResponse)(resp), nil
}

func (m *maintenance) Snapshot(ctx context.Context) (io.ReadCloser, error) {
	ss, err := m.getRemote().Snapshot(ctx, &pb.SnapshotRequest{})
	if err != nil {
		return nil, rpctypes.Error(err)
	}

	pr, pw := io.Pipe()
	go func() {
		for {
			resp, err := ss.Recv()
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			if resp == nil && err == nil {
				break
			}
			if _, werr := pw.Write(resp.Blob); werr != nil {
				pw.CloseWithError(werr)
				return
			}
		}
		pw.Close()
	}()
	return pr, nil
}

func (m *maintenance) getRemote() pb.MaintenanceClient {
	m.rc.mu.Lock()
	defer m.rc.mu.Unlock()
	return m.remote
}
