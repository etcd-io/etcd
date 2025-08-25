// Copyright 2018 The etcd Authors
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

package mockserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

// MockServer provides a mocked out grpc server of the etcdserver interface.
type MockServer struct {
	ln         net.Listener
	Network    string
	Address    string
	GRPCServer *grpc.Server
	// Maintenance provides access to mock maintenance server state for tests.
	Maintenance *mockMaintenanceServer
}

func (ms *MockServer) ResolverAddress() resolver.Address {
	switch ms.Network {
	case "unix":
		return resolver.Address{Addr: fmt.Sprintf("unix://%s", ms.Address)}
	case "tcp":
		return resolver.Address{Addr: ms.Address}
	default:
		panic("illegal network type: " + ms.Network)
	}
}

// MockServers provides a cluster of mocket out gprc servers of the etcdserver interface.
type MockServers struct {
	mu      sync.RWMutex
	Servers []*MockServer
	wg      sync.WaitGroup
}

// StartMockServers creates the desired count of mock servers
// and starts them.
func StartMockServers(count int) (ms *MockServers, err error) {
	return StartMockServersOnNetwork(count, "tcp")
}

// StartMockServersOnNetwork creates mock servers on either 'tcp' or 'unix' sockets.
func StartMockServersOnNetwork(count int, network string) (ms *MockServers, err error) {
	switch network {
	case "tcp":
		return startMockServersTCP(count)
	case "unix":
		return startMockServersUnix(count)
	default:
		return nil, fmt.Errorf("unsupported network type: %s", network)
	}
}

func startMockServersTCP(count int) (ms *MockServers, err error) {
	addrs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// Prefer explicit IPv4 loopback to avoid IPv6-only addresses like "[::]:port"
		addrs = append(addrs, "127.0.0.1:0")
	}
	return startMockServers("tcp", addrs)
}

func startMockServersUnix(count int) (ms *MockServers, err error) {
	dir := os.TempDir()
	addrs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		f, err := os.CreateTemp(dir, "etcd-unix-so-")
		if err != nil {
			return nil, fmt.Errorf("failed to allocate temp file for unix socket: %w", err)
		}
		fn := f.Name()
		err = os.Remove(fn)
		if err != nil {
			return nil, fmt.Errorf("failed to remove temp file before creating unix socket: %w", err)
		}
		addrs = append(addrs, fn)
	}
	return startMockServers("unix", addrs)
}

func startMockServers(network string, addrs []string) (ms *MockServers, err error) {
	ms = &MockServers{
		Servers: make([]*MockServer, len(addrs)),
		wg:      sync.WaitGroup{},
	}
	defer func() {
		if err != nil {
			ms.Stop()
		}
	}()
	for idx, addr := range addrs {
		ln, err := net.Listen(network, addr)
		if err != nil {
			return nil, fmt.Errorf("failed to listen %w", err)
		}
		ms.Servers[idx] = &MockServer{ln: ln, Network: network, Address: ln.Addr().String()}
		ms.StartAt(idx)
	}
	return ms, nil
}

// StartAt restarts mock server at given index.
func (ms *MockServers) StartAt(idx int) (err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.Servers[idx].ln == nil {
		ms.Servers[idx].ln, err = net.Listen(ms.Servers[idx].Network, ms.Servers[idx].Address)
		if err != nil {
			return fmt.Errorf("failed to listen %w", err)
		}
	}

	svr := grpc.NewServer()
	pb.RegisterKVServer(svr, &mockKVServer{})
	pb.RegisterLeaseServer(svr, &mockLeaseServer{})
	// Initialize and register mock maintenance server
	mtn := newMockMaintenanceServer()
	pb.RegisterMaintenanceServer(svr, mtn)
	ms.Servers[idx].Maintenance = mtn
	ms.Servers[idx].GRPCServer = svr

	ms.wg.Add(1)
	go func(svr *grpc.Server, l net.Listener) {
		svr.Serve(l)
	}(ms.Servers[idx].GRPCServer, ms.Servers[idx].ln)
	return nil
}

// StopAt stops mock server at given index.
func (ms *MockServers) StopAt(idx int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.Servers[idx].ln == nil {
		return
	}

	ms.Servers[idx].GRPCServer.Stop()
	ms.Servers[idx].GRPCServer = nil
	ms.Servers[idx].ln = nil
	ms.wg.Done()
}

// Stop stops the mock server, immediately closing all open connections and listeners.
func (ms *MockServers) Stop() {
	for idx := range ms.Servers {
		ms.StopAt(idx)
	}
	ms.wg.Wait()
}

type mockKVServer struct{}

func (m *mockKVServer) Range(context.Context, *pb.RangeRequest) (*pb.RangeResponse, error) {
	return &pb.RangeResponse{}, nil
}

func (m *mockKVServer) Put(context.Context, *pb.PutRequest) (*pb.PutResponse, error) {
	return &pb.PutResponse{}, nil
}

func (m *mockKVServer) DeleteRange(context.Context, *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	return &pb.DeleteRangeResponse{}, nil
}

func (m *mockKVServer) Txn(context.Context, *pb.TxnRequest) (*pb.TxnResponse, error) {
	return &pb.TxnResponse{}, nil
}

func (m *mockKVServer) Compact(context.Context, *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	return &pb.CompactionResponse{}, nil
}

func (m *mockKVServer) Lease(context.Context, *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error) {
	return &pb.LeaseGrantResponse{}, nil
}

type mockLeaseServer struct{}

func (s mockLeaseServer) LeaseGrant(context.Context, *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error) {
	return &pb.LeaseGrantResponse{}, nil
}

func (s *mockLeaseServer) LeaseRevoke(context.Context, *pb.LeaseRevokeRequest) (*pb.LeaseRevokeResponse, error) {
	return &pb.LeaseRevokeResponse{}, nil
}

func (s *mockLeaseServer) LeaseKeepAlive(pb.Lease_LeaseKeepAliveServer) error {
	return nil
}

func (s *mockLeaseServer) LeaseTimeToLive(context.Context, *pb.LeaseTimeToLiveRequest) (*pb.LeaseTimeToLiveResponse, error) {
	return &pb.LeaseTimeToLiveResponse{}, nil
}

func (s *mockLeaseServer) LeaseLeases(context.Context, *pb.LeaseLeasesRequest) (*pb.LeaseLeasesResponse, error) {
	return &pb.LeaseLeasesResponse{}, nil
}

// mockMaintenanceServer implements pb.MaintenanceServer with minimal Alarm support.
type mockMaintenanceServer struct {
	pb.UnimplementedMaintenanceServer
	mu sync.Mutex
	// alarms indexed by memberID -> alarmType
	alarms map[uint64]map[pb.AlarmType]struct{}
}

func newMockMaintenanceServer() *mockMaintenanceServer {
	return &mockMaintenanceServer{
		alarms: make(map[uint64]map[pb.AlarmType]struct{}),
	}
}

// SetAlarms replaces the current alarms with the provided list.
func (m *mockMaintenanceServer) SetAlarms(list []*pb.AlarmMember) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alarms = make(map[uint64]map[pb.AlarmType]struct{})
	for _, am := range list {
		if am == nil {
			continue
		}
		if _, ok := m.alarms[am.MemberID]; !ok {
			m.alarms[am.MemberID] = make(map[pb.AlarmType]struct{})
		}
		m.alarms[am.MemberID][am.Alarm] = struct{}{}
	}
}

// listAlarms returns a snapshot slice of current alarms.
func (m *mockMaintenanceServer) listAlarms() []*pb.AlarmMember {
	res := make([]*pb.AlarmMember, 0)
	for mid, set := range m.alarms {
		for at := range set {
			am := &pb.AlarmMember{MemberID: mid, Alarm: at}
			res = append(res, am)
		}
	}
	return res
}

// Alarm supports GET and DEACTIVATE actions.
func (m *mockMaintenanceServer) Alarm(ctx context.Context, req *pb.AlarmRequest) (*pb.AlarmResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch req.GetAction() {
	case pb.AlarmRequest_GET:
		return &pb.AlarmResponse{Alarms: m.listAlarms()}, nil
	case pb.AlarmRequest_DEACTIVATE:
		// If MemberID==0 and Alarm==NONE, do nothing here. The client-side
		// implementation expands it into multiple specific DEACTIVATE calls.
		mid := req.GetMemberID()
		at := req.GetAlarm()
		var deactivated []*pb.AlarmMember
		if mid == 0 || at == pb.AlarmType_NONE {
			// Nothing to deactivate explicitly
			return &pb.AlarmResponse{Alarms: deactivated}, nil
		}
		if set, ok := m.alarms[mid]; ok {
			if _, ok2 := set[at]; ok2 {
				delete(set, at)
				deactivated = append(deactivated, &pb.AlarmMember{MemberID: mid, Alarm: at})
				if len(set) == 0 {
					delete(m.alarms, mid)
				}
			}
		}
		return &pb.AlarmResponse{Alarms: deactivated}, nil
	default:
		return &pb.AlarmResponse{}, nil
	}
}

// SetMaintenanceAlarmsAt seeds alarms for a specific server index.
func (ms *MockServers) SetMaintenanceAlarmsAt(idx int, list []*pb.AlarmMember) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if idx < 0 || idx >= len(ms.Servers) || ms.Servers[idx] == nil || ms.Servers[idx].Maintenance == nil {
		return
	}
	ms.Servers[idx].Maintenance.SetAlarms(list)
}

// SetMaintenanceAlarmsAll seeds alarms for all servers.
func (ms *MockServers) SetMaintenanceAlarmsAll(list []*pb.AlarmMember) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, s := range ms.Servers {
		if s != nil && s.Maintenance != nil {
			s.Maintenance.SetAlarms(list)
		}
	}
}
