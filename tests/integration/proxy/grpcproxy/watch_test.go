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
	"context"
	"math"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/proxy/grpcproxy"
	"go.etcd.io/etcd/server/v3/storage/mvcc"

	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestWatchProxyWatchID(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	wpts := newWatchProxyServer([]string{clus.Members[0].GRPCURL()}, t)
	defer wpts.close()

	cfg := clientv3.Config{
		Endpoints:   []string{wpts.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := integration2.NewClient(t, cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	wc := pb.NewWatchClient(client.ActiveConnection())
	callOpts := []grpc.CallOption{
		grpc.WaitForReady(true),
		grpc.MaxCallSendMsgSize(2 * 1024 * 1024),
		grpc.MaxCallRecvMsgSize(math.MaxInt32),
	}
	wcs, err := wc.Watch(context.TODO(), callOpts...)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	//specified watch id
	req := newWatchCreateRequest(1, "aa", "ab")
	err = wcs.Send(req)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	var resp *pb.WatchResponse
	resp, err = wcs.Recv()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if resp.WatchId != 1 {
		t.Fatalf("watchID = %v, want 1", resp.WatchId)
	}

	//not specified watch id,
	req = newWatchCreateRequest(clientv3.AutoWatchID, "aa", "ab")
	err = wcs.Send(req)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	resp, err = wcs.Recv()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if resp.WatchId != 0 {
		t.Fatalf("watchID = %v, want 0", resp.WatchId)
	}

	//auto id skip the id already in use
	req = newWatchCreateRequest(clientv3.AutoWatchID, "aa", "ab")
	err = wcs.Send(req)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	resp, err = wcs.Recv()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if resp.WatchId != 2 {
		t.Fatalf("watchID = %v, want 2", resp.WatchId)
	}

	//specified duplicated watch id
	req = newWatchCreateRequest(1, "aa", "ab")
	err = wcs.Send(req)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	resp, err = wcs.Recv()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if resp.CancelReason != mvcc.ErrWatcherDuplicateID.Error() {
		t.Fatalf("resp.CancelReason = %s, want %s", resp.CancelReason, mvcc.ErrWatcherDuplicateID.Error())
	}
	wcs.CloseSend()
	client.Close()
}

type watchProxyTestServer struct {
	wp     pb.WatchServer
	c      *clientv3.Client
	server *grpc.Server
	l      net.Listener
	cancel context.CancelFunc
}

func (wpts *watchProxyTestServer) close() {
	wpts.cancel()
	wpts.server.Stop()
	wpts.l.Close()
	wpts.c.Close()

}

func newWatchProxyServer(endpoints []string, t *testing.T) *watchProxyTestServer {
	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}
	client, err := integration2.NewClient(t, cfg)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.TODO())
	wp, _ := grpcproxy.NewWatchProxy(ctx, client.GetLogger(), client)

	wpts := &watchProxyTestServer{
		wp:     wp,
		c:      client,
		cancel: cancel,
	}

	var opts []grpc.ServerOption
	wpts.server = grpc.NewServer(opts...)
	pb.RegisterWatchServer(wpts.server, wpts.wp)

	wpts.l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go wpts.server.Serve(wpts.l)

	return wpts
}

func newWatchCreateRequest(watchID int64, key, end string) *pb.WatchRequest {
	req := &pb.WatchCreateRequest{
		Key:      []byte(key),
		RangeEnd: []byte(end),
		WatchId:  watchID,
	}
	cr := &pb.WatchRequest_CreateRequest{CreateRequest: req}
	return &pb.WatchRequest{RequestUnion: cr}
}
