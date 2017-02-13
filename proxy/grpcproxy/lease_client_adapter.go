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
	"golang.org/x/net/context"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ls2lc struct {
	leaseServer pb.LeaseServer
}

func LeaseServerToLeaseClient(ls pb.LeaseServer) pb.LeaseClient {
	return &ls2lc{ls}
}

func (c *ls2lc) LeaseGrant(ctx context.Context, in *pb.LeaseGrantRequest, opts ...grpc.CallOption) (*pb.LeaseGrantResponse, error) {
	return c.leaseServer.LeaseGrant(ctx, in)
}

func (c *ls2lc) LeaseRevoke(ctx context.Context, in *pb.LeaseRevokeRequest, opts ...grpc.CallOption) (*pb.LeaseRevokeResponse, error) {
	return c.leaseServer.LeaseRevoke(ctx, in)
}

func (c *ls2lc) LeaseKeepAlive(ctx context.Context, opts ...grpc.CallOption) (pb.Lease_LeaseKeepAliveClient, error) {
	// ch1 is buffered so server can send error on close
	ch1, ch2 := make(chan interface{}, 1), make(chan interface{})
	headerc, trailerc := make(chan metadata.MD, 1), make(chan metadata.MD, 1)

	cctx, ccancel := context.WithCancel(ctx)
	cli := &chanStream{recvc: ch1, sendc: ch2, ctx: cctx, cancel: ccancel}
	lclient := &ls2lcClientStream{chanClientStream{headerc, trailerc, cli}}

	sctx, scancel := context.WithCancel(ctx)
	srv := &chanStream{recvc: ch2, sendc: ch1, ctx: sctx, cancel: scancel}
	lserver := &ls2lcServerStream{chanServerStream{headerc, trailerc, srv, nil}}
	go func() {
		if err := c.leaseServer.LeaseKeepAlive(lserver); err != nil {
			select {
			case srv.sendc <- err:
			case <-sctx.Done():
			case <-cctx.Done():
			}
		}
		scancel()
		ccancel()
	}()
	return lclient, nil
}

func (c *ls2lc) LeaseTimeToLive(ctx context.Context, in *pb.LeaseTimeToLiveRequest, opts ...grpc.CallOption) (*pb.LeaseTimeToLiveResponse, error) {
	return c.leaseServer.LeaseTimeToLive(ctx, in)
}

// ls2lcClientStream implements Lease_LeaseKeepAliveClient
type ls2lcClientStream struct{ chanClientStream }

// ls2lcServerStream implements Lease_LeaseKeepAliveServer
type ls2lcServerStream struct{ chanServerStream }

func (s *ls2lcClientStream) Send(rr *pb.LeaseKeepAliveRequest) error {
	return s.SendMsg(rr)
}
func (s *ls2lcClientStream) Recv() (*pb.LeaseKeepAliveResponse, error) {
	var v interface{}
	if err := s.RecvMsg(&v); err != nil {
		return nil, err
	}
	return v.(*pb.LeaseKeepAliveResponse), nil
}

func (s *ls2lcServerStream) Send(rr *pb.LeaseKeepAliveResponse) error {
	return s.SendMsg(rr)
}
func (s *ls2lcServerStream) Recv() (*pb.LeaseKeepAliveRequest, error) {
	var v interface{}
	if err := s.RecvMsg(&v); err != nil {
		return nil, err
	}
	return v.(*pb.LeaseKeepAliveRequest), nil
}
