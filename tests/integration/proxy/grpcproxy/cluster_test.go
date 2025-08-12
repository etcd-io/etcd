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

package grpcproxy

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"go.etcd.io/etcd/server/v3/proxy/grpcproxy"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestClusterProxyMemberList(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	lg := zaptest.NewLogger(t)
	serverEps := []string{clus.Members[0].GRPCURL}
	prefix := "test-prefix"
	hostname, _ := os.Hostname()
	cts := newClusterProxyServer(lg, serverEps, prefix, t)
	defer cts.close(t)

	cfg := clientv3.Config{
		Endpoints:   []string{cts.caddr},
		DialTimeout: 5 * time.Second,
	}
	client, err := integration2.NewClient(t, cfg)
	require.NoErrorf(t, err, "err %v, want nil", err)
	defer client.Close()

	// wait some time for register-loop to write keys
	time.Sleep(200 * time.Millisecond)

	var mresp *clientv3.MemberListResponse
	mresp, err = client.Cluster.MemberList(t.Context())
	require.NoErrorf(t, err, "err %v, want nil", err)

	require.Lenf(t, mresp.Members, 1, "len(mresp.Members) expected 1, got %d (%+v)", len(mresp.Members), mresp.Members)
	require.Lenf(t, mresp.Members[0].ClientURLs, 1, "len(mresp.Members[0].ClientURLs) expected 1, got %d (%+v)", len(mresp.Members[0].ClientURLs), mresp.Members[0].ClientURLs[0])
	assert.Contains(t, mresp.Members, &pb.Member{Name: hostname, ClientURLs: []string{cts.caddr}})

	// test proxy member add
	newMemberAddr := "127.0.0.2:6789"
	grpcproxy.Register(lg, cts.c, prefix, newMemberAddr, 7)
	// wait some time for proxy update members
	time.Sleep(200 * time.Millisecond)

	// check add member succ
	mresp, err = client.Cluster.MemberList(t.Context())
	require.NoErrorf(t, err, "err %v, want nil", err)
	require.Lenf(t, mresp.Members, 2, "len(mresp.Members) expected 2, got %d (%+v)", len(mresp.Members), mresp.Members)
	assert.Contains(t, mresp.Members, &pb.Member{Name: hostname, ClientURLs: []string{newMemberAddr}})

	// test proxy member delete
	deregisterMember(cts.c, prefix, newMemberAddr, t)
	// wait some time for proxy update members
	time.Sleep(200 * time.Millisecond)

	// check delete member succ
	mresp, err = client.Cluster.MemberList(t.Context())
	require.NoErrorf(t, err, "err %v, want nil", err)
	require.Lenf(t, mresp.Members, 1, "len(mresp.Members) expected 1, got %d (%+v)", len(mresp.Members), mresp.Members)
	assert.Contains(t, mresp.Members, &pb.Member{Name: hostname, ClientURLs: []string{cts.caddr}})
}

type clusterproxyTestServer struct {
	cp     pb.ClusterServer
	c      *clientv3.Client
	server *grpc.Server
	l      net.Listener
	donec  <-chan struct{}
	caddr  string
}

func (cts *clusterproxyTestServer) close(t *testing.T) {
	cts.server.Stop()
	cts.l.Close()
	cts.c.Close()
	select {
	case <-cts.donec:
		return
	case <-time.After(5 * time.Second):
		t.Fatalf("register-loop took too long to return")
	}
}

func newClusterProxyServer(lg *zap.Logger, endpoints []string, prefix string, t *testing.T) *clusterproxyTestServer {
	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}
	client, err := integration2.NewClient(t, cfg)
	require.NoError(t, err)

	cts := &clusterproxyTestServer{
		c: client,
	}
	cts.l, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var opts []grpc.ServerOption
	cts.server = grpc.NewServer(opts...)
	servec := make(chan struct{})
	go func() {
		<-servec
		cts.server.Serve(cts.l)
	}()

	grpcproxy.Register(lg, client, prefix, cts.l.Addr().String(), 7)
	cts.cp, cts.donec = grpcproxy.NewClusterProxy(lg, client, cts.l.Addr().String(), prefix)
	cts.caddr = cts.l.Addr().String()
	pb.RegisterClusterServer(cts.server, cts.cp)
	close(servec)

	// wait some time for free port 0 to be resolved
	time.Sleep(500 * time.Millisecond)

	return cts
}

func deregisterMember(c *clientv3.Client, prefix, addr string, t *testing.T) {
	em, err := endpoints.NewManager(c, prefix)
	require.NoErrorf(t, err, "new endpoint manager failed, err")
	err = em.DeleteEndpoint(c.Ctx(), prefix+"/"+addr)
	require.NoErrorf(t, err, "delete endpoint failed, err")
}
