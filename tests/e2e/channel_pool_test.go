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

package e2e

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/kubernetes"
)

func TestChannelPoolCreatesConnectionPerEndpoint(t *testing.T) {
	const endpointCount = 3
	servers := make([]*trackedKVServer, 0, endpointCount)
	endpoints := make([]string, 0, endpointCount)
	for range endpointCount {
		s := newTrackedKVServer(t)
		servers = append(servers, s)
		endpoints = append(endpoints, "http://"+s.addr())
	}

	// DialTimeout makes client initialization wait for both the default channel
	// and the configured "bulk" channel to become ready.
	c, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 30 * time.Second,
		ChannelKeys: []string{"bulk"},
	})
	require.NoError(t, err)
	defer c.Close()

	requireChannelConnections(t, servers, 6)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	for i := range endpointCount {
		_, err = c.Get(ctx, fmt.Sprintf("default-%d", i))
		require.NoError(t, err)
		_, err = c.Get(clientv3.WithChannelKey(ctx, "bulk"), fmt.Sprintf("bulk-%d", i))
		require.NoError(t, err)
	}

	for _, s := range servers {
		defaultPeer, ok := s.peerAddr("default")
		require.True(t, ok)
		bulkPeer, ok := s.peerAddr("bulk")
		require.True(t, ok)
		require.NotEqual(t, defaultPeer, bulkPeer)
	}
}

func TestKubernetesClientListSelectsChannel(t *testing.T) {
	const endpointCount = 3
	servers := make([]*trackedKVServer, 0, endpointCount)
	endpoints := make([]string, 0, endpointCount)
	for range endpointCount {
		s := newTrackedKVServer(t)
		servers = append(servers, s)
		endpoints = append(endpoints, "http://"+s.addr())
	}

	c, err := kubernetes.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 30 * time.Second,
		ChannelKeys: []string{"bulk"},
	})
	require.NoError(t, err)
	defer c.Close()

	requireChannelConnections(t, servers, 6)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	for i := range endpointCount {
		_, err = c.Kubernetes.List(ctx, fmt.Sprintf("default-%d", i), kubernetes.ListOptions{})
		require.NoError(t, err)
		_, err = c.Kubernetes.List(clientv3.WithChannelKey(ctx, "bulk"), fmt.Sprintf("bulk-%d", i), kubernetes.ListOptions{})
		require.NoError(t, err)
	}

	for _, s := range servers {
		defaultPeer, ok := s.peerAddr("default")
		require.True(t, ok)
		bulkPeer, ok := s.peerAddr("bulk")
		require.True(t, ok)
		require.NotEqual(t, defaultPeer, bulkPeer)
	}
}

func requireChannelConnections(t *testing.T, servers []*trackedKVServer, totalConnections uint64) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var total uint64
		for _, s := range servers {
			total += s.accepted()
			require.Equal(c, uint64(2), s.accepted())
		}
		require.Equal(c, totalConnections, total)
	}, 30*time.Second, 10*time.Millisecond)
}

type trackedKVServer struct {
	etcdserverpb.UnimplementedKVServer

	listener *trackedListener
	server   *grpc.Server
	donec    chan error
	mu       sync.Mutex
	peers    map[string]string
}

func newTrackedKVServer(t *testing.T) *trackedKVServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	trackedLn := &trackedListener{Listener: ln}

	s := &trackedKVServer{
		listener: trackedLn,
		server:   grpc.NewServer(),
		donec:    make(chan error, 1),
		peers:    make(map[string]string),
	}
	healthpb.RegisterHealthServer(s.server, health.NewServer())
	etcdserverpb.RegisterKVServer(s.server, s)
	go func() {
		s.donec <- s.server.Serve(trackedLn)
	}()
	t.Cleanup(func() {
		s.server.Stop()
		<-s.donec
	})
	return s
}

func (s *trackedKVServer) Range(ctx context.Context, req *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		keyPrefix, _, _ := strings.Cut(string(req.Key), "-")
		if keyPrefix == "default" || keyPrefix == "bulk" {
			s.mu.Lock()
			s.peers[keyPrefix] = p.Addr.String()
			s.mu.Unlock()
		}
	}
	return &etcdserverpb.RangeResponse{Header: &etcdserverpb.ResponseHeader{}}, nil
}

func (s *trackedKVServer) addr() string {
	return s.listener.Addr().String()
}

func (s *trackedKVServer) accepted() uint64 {
	return s.listener.accepted.Load()
}

func (s *trackedKVServer) peerAddr(keyPrefix string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	peer, ok := s.peers[keyPrefix]
	return peer, ok
}

type trackedListener struct {
	net.Listener
	accepted atomic.Uint64
}

func (l *trackedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.accepted.Add(1)
	}
	return conn, err
}
