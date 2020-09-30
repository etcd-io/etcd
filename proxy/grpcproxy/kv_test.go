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

	"go.etcd.io/etcd/v3/clientv3"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/v3/integration"
	"go.etcd.io/etcd/v3/pkg/testutil"

	"google.golang.org/grpc"
)

func TestKVProxyRange(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvts := newKVProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer kvts.close()

	// create a client and try to get key from proxy.
	cfg := clientv3.Config{
		Endpoints:   []string{kvts.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	_, err = client.Get(context.Background(), "foo")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	client.Close()
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

func newKVProxyServer(endpoints []string, t *testing.T) *kvproxyTestServer {
	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	kvp, _ := NewKvProxy(client)

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

	go kvts.server.Serve(kvts.l)

	return kvts
}
