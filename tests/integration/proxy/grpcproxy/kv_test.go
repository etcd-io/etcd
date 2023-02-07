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
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/proxy/grpcproxy"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestKVProxyRangeWithCache(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvts := newKVProxyServer(t, []string{clus.Members[0].GRPCURL()}, grpcproxy.WithCache(true))
	defer kvts.close()

	// create a client and try to get key from proxy.
	cfg := clientv3.Config{
		Endpoints:   []string{kvts.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := integration2.NewClient(t, cfg)
	require.NoError(t, err)
	defer client.Close()

	// Put and check value
	_, err = clus.Client(0).Put(context.Background(), "foo", "bar")
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "foo")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, "bar", string(resp.Kvs[0].Value))

	// Update value and check outdated value is in cache
	_, err = clus.Client(0).Put(context.Background(), "foo", "xyz")
	require.NoError(t, err)

	resp, err = client.Get(context.Background(), "foo", clientv3.WithSerializable())
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, "bar", string(resp.Kvs[0].Value))
}

func TestKVProxyRangeWithoutCache(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvts := newKVProxyServer(t, []string{clus.Members[0].GRPCURL()})
	defer kvts.close()

	// create a client and try to get key from proxy.
	cfg := clientv3.Config{
		Endpoints:   []string{kvts.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := integration2.NewClient(t, cfg)
	require.NoError(t, err)
	defer client.Close()

	// Put and check value
	_, err = clus.Client(0).Put(context.Background(), "foo", "bar")
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "foo")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, "bar", string(resp.Kvs[0].Value))

	// Update value and get new value via proxy
	_, err = clus.Client(0).Put(context.Background(), "foo", "xyz")
	require.NoError(t, err)

	resp, err = client.Get(context.Background(), "foo", clientv3.WithSerializable())
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.Equal(t, "xyz", string(resp.Kvs[0].Value))
}

type kvproxyTestServer struct {
	kp     pb.KVServer
	c      *clientv3.Client
	server *grpc.Server
	l      net.Listener
}

func (kts *kvproxyTestServer) close() {
	kts.server.Stop()
	kts.l.Close()
	kts.c.Close()
}

func newKVProxyServer(t *testing.T, endpoints []string, kvOpts ...grpcproxy.KvProxyOpt) *kvproxyTestServer {
	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}
	client, err := integration2.NewClient(t, cfg)
	if err != nil {
		t.Fatal(err)
	}

	kvp, _ := grpcproxy.NewKvProxy(client, kvOpts...)

	kvts := &kvproxyTestServer{
		kp: kvp,
		c:  client,
	}

	var opts []grpc.ServerOption
	kvts.server = grpc.NewServer(opts...)
	pb.RegisterKVServer(kvts.server, kvts.kp)

	kvts.l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		require.NoError(t, kvts.server.Serve(kvts.l))
	}()

	return kvts
}
