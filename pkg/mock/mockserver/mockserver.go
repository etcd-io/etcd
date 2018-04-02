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
	"sync"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"

	"google.golang.org/grpc"
)

// MockServer provides a mocked out grpc server of the etcdserver interface.
type MockServer struct {
	ln         net.Listener
	Address    string
	GrpcServer *grpc.Server
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
	ms = &MockServers{
		Servers: make([]*MockServer, count),
		wg:      sync.WaitGroup{},
	}
	defer func() {
		if err != nil {
			ms.Stop()
		}
	}()
	for idx := 0; idx < count; idx++ {
		ln, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return nil, fmt.Errorf("failed to listen %v", err)
		}
		ms.Servers[idx] = &MockServer{ln: ln, Address: ln.Addr().String()}
		ms.StartAt(idx)
	}
	return ms, nil
}

// StartAt restarts mock server at given index.
func (ms *MockServers) StartAt(idx int) (err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.Servers[idx].ln == nil {
		ms.Servers[idx].ln, err = net.Listen("tcp", ms.Servers[idx].Address)
		if err != nil {
			return fmt.Errorf("failed to listen %v", err)
		}
	}

	svr := grpc.NewServer()
	pb.RegisterKVServer(svr, &mockKVServer{})
	ms.Servers[idx].GrpcServer = svr

	go func(svr *grpc.Server, l net.Listener) {
		ms.wg.Add(1)
		svr.Serve(l)
	}(ms.Servers[idx].GrpcServer, ms.Servers[idx].ln)

	return nil
}

// StopAt stops mock server at given index.
func (ms *MockServers) StopAt(idx int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.Servers[idx].ln == nil {
		return
	}

	ms.Servers[idx].GrpcServer.Stop()
	ms.Servers[idx].GrpcServer = nil
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
